package plugins

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// FileWatcher monitors plugin configuration files for changes
type FileWatcher interface {
	Start(ctx context.Context) error
	Stop() error
	SetEventHandler(handler FileEventHandler)
	AddWatchPath(path string) error
	RemoveWatchPath(path string) error
}

// FileEventHandler processes file system events
type FileEventHandler func(event PluginFileEvent) error

// PluginFileEvent represents a plugin configuration file change
type PluginFileEvent struct {
	Path        string       `json:"path"`
	Operation   fsnotify.Op  `json:"operation"`
	Timestamp   time.Time    `json:"timestamp"`
	ConfigDelta *ConfigDelta `json:"config_delta,omitempty"`
}

// ConfigDelta represents changes in plugin configuration
type ConfigDelta struct {
	PluginName string                 `json:"plugin_name"`
	OldConfig  *PluginConfig          `json:"old_config,omitempty"`
	NewConfig  *PluginConfig          `json:"new_config,omitempty"`
	Changes    map[string]interface{} `json:"changes"`
	ChangeType DeltaType              `json:"change_type"`
}

// DeltaType represents the type of configuration change
type DeltaType string

const (
	DeltaTypeCreate DeltaType = "create"
	DeltaTypeUpdate DeltaType = "update"
	DeltaTypeDelete DeltaType = "delete"
)

// fileWatcherImpl provides fsnotify-based file watching
type fileWatcherImpl struct {
	watcher     *fsnotify.Watcher
	eventChan   chan PluginFileEvent
	handler     FileEventHandler
	watchPaths  map[string]bool
	configCache map[string]*PluginConfig
	
	// Debouncing
	debouncers  map[string]*time.Timer
	debounceDelay time.Duration
	
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewFileWatcher creates a new FileWatcher instance
func NewFileWatcher() FileWatcher {
	return &fileWatcherImpl{
		eventChan:     make(chan PluginFileEvent, 100),
		watchPaths:    make(map[string]bool),
		configCache:   make(map[string]*PluginConfig),
		debouncers:    make(map[string]*time.Timer),
		debounceDelay: 100 * time.Millisecond, // Default debounce delay
	}
}

// Start begins the file watching process
func (fw *fileWatcherImpl) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	
	fw.watcher = watcher
	fw.ctx, fw.cancel = context.WithCancel(ctx)
	
	// Add all watch paths to fsnotify watcher
	fw.mu.RLock()
	for path := range fw.watchPaths {
		if err := fw.watcher.Add(path); err != nil {
			fw.mu.RUnlock()
			return fmt.Errorf("failed to add watch path %s: %w", path, err)
		}
	}
	fw.mu.RUnlock()
	
	go fw.watchLoop()
	return nil
}

// Stop halts the file watching process
func (fw *fileWatcherImpl) Stop() error {
	if fw.cancel != nil {
		fw.cancel()
	}
	
	if fw.watcher != nil {
		return fw.watcher.Close()
	}
	
	return nil
}

// SetEventHandler sets the handler for file events
func (fw *fileWatcherImpl) SetEventHandler(handler FileEventHandler) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.handler = handler
}

// AddWatchPath adds a path to be monitored
func (fw *fileWatcherImpl) AddWatchPath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	fw.watchPaths[path] = true
	
	// If watcher is already running, add the path
	if fw.watcher != nil {
		if err := fw.watcher.Add(path); err != nil {
			return fmt.Errorf("failed to add watch path %s: %w", path, err)
		}
	}
	
	return nil
}

// RemoveWatchPath removes a path from monitoring
func (fw *fileWatcherImpl) RemoveWatchPath(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	delete(fw.watchPaths, path)
	
	// If watcher is running, remove the path
	if fw.watcher != nil {
		if err := fw.watcher.Remove(path); err != nil {
			return fmt.Errorf("failed to remove watch path %s: %w", path, err)
		}
	}
	
	return nil
}

// watchLoop is the main event processing loop
func (fw *fileWatcherImpl) watchLoop() {
	for {
		select {
		case <-fw.ctx.Done():
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.processEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue
			log.Printf("FileWatcher error: %v", err)
		}
	}
}

// processEvent handles individual file system events
func (fw *fileWatcherImpl) processEvent(fsEvent fsnotify.Event) {
	if !fw.isPluginConfigFile(fsEvent.Name) {
		return
	}
	
	// Implement debouncing
	fw.mu.Lock()
	if timer, exists := fw.debouncers[fsEvent.Name]; exists {
		timer.Stop()
	}
	
	fw.debouncers[fsEvent.Name] = time.AfterFunc(fw.debounceDelay, func() {
		fw.handleDebouncedEvent(fsEvent)
		
		fw.mu.Lock()
		delete(fw.debouncers, fsEvent.Name)
		fw.mu.Unlock()
	})
	fw.mu.Unlock()
}

// handleDebouncedEvent processes the event after debounce delay
func (fw *fileWatcherImpl) handleDebouncedEvent(fsEvent fsnotify.Event) {
	delta, err := fw.calculateConfigDelta(fsEvent.Name, fsEvent.Op)
	if err != nil {
		log.Printf("Failed to calculate config delta for %s: %v", fsEvent.Name, err)
		return
	}
	
	pluginEvent := PluginFileEvent{
		Path:        fsEvent.Name,
		Operation:   fsEvent.Op,
		Timestamp:   time.Now(),
		ConfigDelta: delta,
	}
	
	fw.mu.RLock()
	handler := fw.handler
	fw.mu.RUnlock()
	
	if handler != nil {
		if err := handler(pluginEvent); err != nil {
			log.Printf("Event handler failed for %s: %v", fsEvent.Name, err)
		}
	}
}

// isPluginConfigFile checks if a file is a plugin configuration
func (fw *fileWatcherImpl) isPluginConfigFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}

// calculateConfigDelta computes the configuration changes
func (fw *fileWatcherImpl) calculateConfigDelta(configPath string, op fsnotify.Op) (*ConfigDelta, error) {
	pluginName := fw.extractPluginName(configPath)
	
	fw.mu.RLock()
	oldConfig := fw.configCache[configPath]
	fw.mu.RUnlock()
	
	var newConfig *PluginConfig
	var err error
	
	if op != fsnotify.Remove && op != fsnotify.Rename {
		newConfig, err = fw.loadConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	
	delta := &ConfigDelta{
		PluginName: pluginName,
		OldConfig:  oldConfig,
		NewConfig:  newConfig,
		Changes:    make(map[string]interface{}),
	}
	
	// Determine change type and calculate differences
	switch {
	case oldConfig == nil && newConfig != nil:
		delta.ChangeType = DeltaTypeCreate
		if newConfig.Settings != nil {
			delta.Changes = newConfig.Settings
		}
	case oldConfig != nil && newConfig == nil:
		delta.ChangeType = DeltaTypeDelete
		if oldConfig.Settings != nil {
			delta.Changes = oldConfig.Settings
		}
	case oldConfig != nil && newConfig != nil:
		delta.ChangeType = DeltaTypeUpdate
		delta.Changes = fw.calculateSettingsDiff(oldConfig.Settings, newConfig.Settings)
	}
	
	// Update cache
	fw.mu.Lock()
	if newConfig != nil {
		fw.configCache[configPath] = newConfig
	} else {
		delete(fw.configCache, configPath)
	}
	fw.mu.Unlock()
	
	return delta, nil
}

// extractPluginName derives the plugin name from the file path
func (fw *fileWatcherImpl) extractPluginName(configPath string) string {
	base := filepath.Base(configPath)
	// Remove extension
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return name
}

// loadConfigFromFile reads and parses a plugin configuration file
func (fw *fileWatcherImpl) loadConfigFromFile(configPath string) (*PluginConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config PluginConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// If plugin name is not set, use filename
	if config.Name == "" {
		config.Name = fw.extractPluginName(configPath)
	}
	
	return &config, nil
}

// calculateSettingsDiff computes the differences between two settings maps
func (fw *fileWatcherImpl) calculateSettingsDiff(oldSettings, newSettings map[string]interface{}) map[string]interface{} {
	changes := make(map[string]interface{})
	
	// Find added or modified settings
	for key, newValue := range newSettings {
		if oldValue, exists := oldSettings[key]; !exists || !fw.isEqual(oldValue, newValue) {
			changes[key] = newValue
		}
	}
	
	// Find deleted settings (represented as nil)
	for key := range oldSettings {
		if _, exists := newSettings[key]; !exists {
			changes[key] = nil
		}
	}
	
	return changes
}

// isEqual compares two values for equality
func (fw *fileWatcherImpl) isEqual(a, b interface{}) bool {
	// Simple equality check - could be enhanced for deep comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}