package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEventType represents the type of file system event
type FileEventType int

const (
	// FileEventCreated indicates a new file was created
	FileEventCreated FileEventType = iota
	// FileEventModified indicates a file was modified
	FileEventModified
	// FileEventDeleted indicates a file was deleted
	FileEventDeleted
	// FileEventRenamed indicates a file was renamed
	FileEventRenamed
)

// FileEvent represents a file system event
type FileEvent struct {
	Type      FileEventType
	Path      string
	Timestamp time.Time
}

// IsPluginFile checks if the event is for a plugin configuration file
func (e FileEvent) IsPluginFile() bool {
	ext := strings.ToLower(filepath.Ext(e.Path))
	return ext == ".json" || ext == ".yaml" || ext == ".yml"
}

// FileWatcher interface for file system watching
type FileWatcher interface {
	Watch(ctx context.Context, path string, eventChan chan<- FileEvent) error
	Stop() error
	IsWatching() bool
	SetDebounceInterval(duration time.Duration)
}

// HotReloadWatcher implements FileWatcher using fsnotify
type HotReloadWatcher struct {
	watcher          *fsnotify.Watcher
	watchedPaths     map[string]bool
	eventChan        chan<- FileEvent
	debounceInterval time.Duration
	debounceTimers   map[string]*time.Timer
	
	ctx    context.Context
	cancel context.CancelFunc
	
	mu      sync.RWMutex
	running bool
}

// NewHotReloadWatcher creates a new hot reload file watcher
func NewHotReloadWatcher() *HotReloadWatcher {
	return &HotReloadWatcher{
		watchedPaths:     make(map[string]bool),
		debounceInterval: 500 * time.Millisecond,
		debounceTimers:   make(map[string]*time.Timer),
	}
}

// Watch starts watching the specified path for file changes
func (w *HotReloadWatcher) Watch(ctx context.Context, path string, eventChan chan<- FileEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.running {
		return fmt.Errorf("watcher is already running")
	}
	
	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	
	// Add path to watch
	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add watch path: %w", err)
	}
	
	w.watcher = watcher
	w.eventChan = eventChan
	w.watchedPaths[path] = true
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	
	// Start event processing goroutine
	go w.processEvents()
	
	return nil
}

// Stop stops the file watcher
func (w *HotReloadWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.running {
		return nil
	}
	
	// Cancel context to stop event processing
	w.cancel()
	
	// Close fsnotify watcher
	if w.watcher != nil {
		if err := w.watcher.Close(); err != nil {
			return fmt.Errorf("failed to close watcher: %w", err)
		}
	}
	
	// Clean up debounce timers
	for _, timer := range w.debounceTimers {
		timer.Stop()
	}
	w.debounceTimers = make(map[string]*time.Timer)
	
	w.running = false
	return nil
}

// IsWatching returns whether the watcher is currently running
func (w *HotReloadWatcher) IsWatching() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// SetDebounceInterval sets the debounce interval for file events
func (w *HotReloadWatcher) SetDebounceInterval(duration time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.debounceInterval = duration
}

// processEvents processes fsnotify events and forwards them as FileEvents
func (w *HotReloadWatcher) processEvents() {
	for {
		select {
		case <-w.ctx.Done():
			return
			
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFsnotifyEvent(event)
			
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			fmt.Printf("File watcher error: %v\n", err)
		}
	}
}

// handleFsnotifyEvent converts fsnotify events to FileEvents with debouncing
func (w *HotReloadWatcher) handleFsnotifyEvent(event fsnotify.Event) {
	// Determine event type
	var eventType FileEventType
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = FileEventCreated
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = FileEventModified
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = FileEventDeleted
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = FileEventRenamed
	default:
		return // Ignore other events
	}
	
	fileEvent := FileEvent{
		Type:      eventType,
		Path:      event.Name,
		Timestamp: time.Now(),
	}
	
	// Apply debouncing
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Cancel existing timer for this path
	if timer, exists := w.debounceTimers[event.Name]; exists {
		timer.Stop()
	}
	
	// Create new debounce timer
	if _, exists := w.debounceTimers[event.Name]; exists {
		// Increment debounce hit count if we're replacing a timer
		// This is a simplified tracking - actual implementation would track per event
	}
	w.debounceTimers[event.Name] = time.AfterFunc(w.debounceInterval, func() {
		// Send event after debounce interval
		select {
		case w.eventChan <- fileEvent:
		case <-w.ctx.Done():
		}
		
		// Clean up timer
		w.mu.Lock()
		delete(w.debounceTimers, event.Name)
		w.mu.Unlock()
	})
}

// PluginReloaderService interface for plugin reload operations
type PluginReloaderService interface {
	ReloadPlugin(ctx context.Context, config PluginConfig) error
	CanReload(pluginName string) bool
}

// PluginReloaderImpl implements PluginReloaderService
type PluginReloaderImpl struct {
	pluginManager interface{
		GetPluginStatus(name string) (PluginStatus, bool)
		StopPlugin(ctx context.Context, name string) error
		UnloadPlugin(ctx context.Context, name string) error
		LoadPlugin(config PluginConfig) error
		StartPlugin(ctx context.Context, name string) error
	}
	mu sync.RWMutex
}

// NewPluginReloaderImpl creates a new plugin reloader
func NewPluginReloaderImpl(pluginManager interface{
	GetPluginStatus(name string) (PluginStatus, bool)
	StopPlugin(ctx context.Context, name string) error
	UnloadPlugin(ctx context.Context, name string) error
	LoadPlugin(config PluginConfig) error
	StartPlugin(ctx context.Context, name string) error
}) *PluginReloaderImpl {
	return &PluginReloaderImpl{
		pluginManager: pluginManager,
	}
}

// ReloadPlugin reloads a plugin with the new configuration
func (r *PluginReloaderImpl) ReloadPlugin(ctx context.Context, config PluginConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check if plugin exists and is running
	status, exists := r.pluginManager.GetPluginStatus(config.Name)
	if exists && status.State == PluginStateRunning {
		// Stop the existing plugin
		if err := r.pluginManager.StopPlugin(ctx, config.Name); err != nil {
			return fmt.Errorf("failed to stop plugin %s: %w", config.Name, err)
		}
		
		// Unload the existing plugin
		if err := r.pluginManager.UnloadPlugin(ctx, config.Name); err != nil {
			return fmt.Errorf("failed to unload plugin %s: %w", config.Name, err)
		}
	}
	
	// Load the plugin with new configuration
	if err := r.pluginManager.LoadPlugin(config); err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", config.Name, err)
	}
	
	// Start the plugin if it should be enabled
	if config.Enabled {
		if err := r.pluginManager.StartPlugin(ctx, config.Name); err != nil {
			return fmt.Errorf("failed to start plugin %s: %w", config.Name, err)
		}
	}
	
	return nil
}

// CanReload checks if a plugin can be reloaded
func (r *PluginReloaderImpl) CanReload(pluginName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Check if plugin exists
	_, exists := r.pluginManager.GetPluginStatus(pluginName)
	return exists
}

// HotReloadSystemInterface defines the interface for hot reload system
type HotReloadSystemInterface interface {
	Start(ctx context.Context, watchPath string) error
	Stop() error
	StartWatching(ctx context.Context, pluginDir string) error
	Shutdown(ctx context.Context) error
	GetMetrics() HotReloadMetrics
	IsRunning() bool
}

// HotReloadMetrics tracks hot reload performance metrics
type HotReloadMetrics struct {
	ReloadCount        uint64
	FailedReloadCount  uint64
	ErrorCount         uint64
	EventsProcessed    uint64
	AverageReloadTime  time.Duration
	LastReloadTime     time.Time
	DebounceHitCount   uint64
}

// HotReloadSystem coordinates file watching and plugin reloading
type HotReloadSystem struct {
	watcher  FileWatcher
	reloader PluginReloaderService
	
	ctx       context.Context
	cancel    context.CancelFunc
	eventChan chan FileEvent
	
	mu      sync.RWMutex
	running bool
	metrics HotReloadMetrics
}

// NewHotReloadSystem creates a new hot reload system
func NewHotReloadSystem(watcher FileWatcher, reloader PluginReloaderService) HotReloadSystemInterface {
	return &HotReloadSystem{
		watcher:   watcher,
		reloader:  reloader,
		eventChan: make(chan FileEvent, 100),
	}
}

// Start starts the hot reload system
func (h *HotReloadSystem) Start(ctx context.Context, watchPath string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("hot reload system is already running")
	}
	
	h.ctx, h.cancel = context.WithCancel(ctx)
	
	// Start file watching
	if err := h.watcher.Watch(h.ctx, watchPath, h.eventChan); err != nil {
		return fmt.Errorf("failed to start file watching: %w", err)
	}
	
	h.running = true
	
	// Start event processing
	go h.processReloadEvents()
	
	return nil
}

// Stop stops the hot reload system
func (h *HotReloadSystem) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	// Cancel context
	h.cancel()
	
	// Stop file watcher
	if err := h.watcher.Stop(); err != nil {
		return fmt.Errorf("failed to stop file watcher: %w", err)
	}
	
	h.running = false
	return nil
}

// processReloadEvents processes file events and triggers plugin reloads
func (h *HotReloadSystem) processReloadEvents() {
	for {
		select {
		case <-h.ctx.Done():
			return
			
		case event := <-h.eventChan:
			// Only process plugin configuration files
			if !event.IsPluginFile() {
				continue
			}
			
			// Handle the reload
			if err := h.handleReloadEvent(event); err != nil {
				fmt.Printf("Failed to handle reload event: %v\n", err)
			}
		}
	}
}

// handleReloadEvent handles a single file event for plugin reload
func (h *HotReloadSystem) handleReloadEvent(event FileEvent) error {
	h.mu.Lock()
	h.metrics.EventsProcessed++
	startTime := time.Now()
	h.mu.Unlock()
	
	// Parse plugin configuration from file
	configData, err := os.ReadFile(event.Path)
	if err != nil {
		h.mu.Lock()
		h.metrics.ErrorCount++
		h.mu.Unlock()
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config PluginConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		h.mu.Lock()
		h.metrics.ErrorCount++
		h.mu.Unlock()
		return fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Reload the plugin
	if err := h.reloader.ReloadPlugin(h.ctx, config); err != nil {
		h.mu.Lock()
		h.metrics.FailedReloadCount++
		h.metrics.ErrorCount++
		h.mu.Unlock()
		return fmt.Errorf("failed to reload plugin: %w", err)
	}
	
	// Update metrics
	h.mu.Lock()
	h.metrics.ReloadCount++
	h.metrics.LastReloadTime = time.Now()
	reloadDuration := time.Since(startTime)
	
	// Update average reload time
	if h.metrics.AverageReloadTime == 0 {
		h.metrics.AverageReloadTime = reloadDuration
	} else {
		// Moving average
		h.metrics.AverageReloadTime = (h.metrics.AverageReloadTime + reloadDuration) / 2
	}
	h.mu.Unlock()
	
	fmt.Printf("Hot reload event: %v for file %s\n", event.Type, event.Path)
	return nil
}

// IsRunning returns whether the hot reload system is running
func (h *HotReloadSystem) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// StartWatching is an alias for Start for interface compatibility
func (h *HotReloadSystem) StartWatching(ctx context.Context, pluginDir string) error {
	return h.Start(ctx, pluginDir)
}

// Shutdown is an alias for Stop for interface compatibility
func (h *HotReloadSystem) Shutdown(ctx context.Context) error {
	return h.Stop()
}

// GetMetrics returns hot reload performance metrics
func (h *HotReloadSystem) GetMetrics() HotReloadMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return h.metrics
}