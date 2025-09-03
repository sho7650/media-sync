package plugins

import (
	"context"
	"fmt"
	"sync"
)

// PluginManager coordinates plugin lifecycle operations
type PluginManager struct {
	registry  *PluginRegistry
	factories *FactoryRegistry
	loader    *PluginLoader
	discovery *PluginDiscovery
	
	mu     sync.RWMutex
	status map[string]PluginStatus
}

// PluginStatus represents the current status of a plugin
type PluginStatus struct {
	State   PluginState `json:"state"`
	Message string      `json:"message,omitempty"`
	Error   error       `json:"-"`
}

// PluginState represents plugin lifecycle states
type PluginState string

const (
	PluginStateUnknown     PluginState = "unknown"
	PluginStateDiscovered  PluginState = "discovered"
	PluginStateLoading     PluginState = "loading"
	PluginStateLoaded      PluginState = "loaded"
	PluginStateStarting    PluginState = "starting"
	PluginStateRunning     PluginState = "running"
	PluginStateStopping    PluginState = "stopping"
	PluginStateStopped     PluginState = "stopped"
	PluginStateError       PluginState = "error"
	PluginStateUnloading   PluginState = "unloading"
)

// NewPluginManager creates a new plugin manager
func NewPluginManager() *PluginManager {
	registry := NewPluginRegistry()
	factories := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factories)
	discovery := NewPluginDiscovery()
	
	return &PluginManager{
		registry:  registry,
		factories: factories,
		loader:    loader,
		discovery: discovery,
		status:    make(map[string]PluginStatus),
	}
}

// RegisterFactory registers a plugin factory
func (m *PluginManager) RegisterFactory(pluginType string, factory PluginFactory) error {
	return m.factories.RegisterFactory(pluginType, factory)
}

// LoadPlugin loads a single plugin with state tracking
func (m *PluginManager) LoadPlugin(config PluginConfig) error {
	m.setPluginStatus(config.Name, PluginStateLoading, "Loading plugin", nil)
	
	if err := m.loader.LoadPlugin(config); err != nil {
		m.setPluginStatus(config.Name, PluginStateError, "Failed to load", err)
		return err
	}
	
	m.setPluginStatus(config.Name, PluginStateLoaded, "Plugin loaded successfully", nil)
	return nil
}

// StartPlugin starts a loaded plugin
func (m *PluginManager) StartPlugin(ctx context.Context, name string) error {
	plugin, exists := m.registry.GetPlugin(name)
	if !exists {
		err := fmt.Errorf("%w: plugin %s", ErrPluginNotFound, name)
		m.setPluginStatus(name, PluginStateError, "Plugin not found", err)
		return err
	}
	
	m.setPluginStatus(name, PluginStateStarting, "Starting plugin", nil)
	
	if err := plugin.Start(ctx); err != nil {
		m.setPluginStatus(name, PluginStateError, "Failed to start", err)
		return NewPluginError(name, "start", err)
	}
	
	m.setPluginStatus(name, PluginStateRunning, "Plugin running", nil)
	return nil
}

// StopPlugin stops a running plugin
func (m *PluginManager) StopPlugin(ctx context.Context, name string) error {
	plugin, exists := m.registry.GetPlugin(name)
	if !exists {
		err := fmt.Errorf("%w: plugin %s", ErrPluginNotFound, name)
		m.setPluginStatus(name, PluginStateError, "Plugin not found", err)
		return err
	}
	
	m.setPluginStatus(name, PluginStateStopping, "Stopping plugin", nil)
	
	if err := plugin.Stop(ctx); err != nil {
		m.setPluginStatus(name, PluginStateError, "Failed to stop", err)
		return NewPluginError(name, "stop", err)
	}
	
	m.setPluginStatus(name, PluginStateStopped, "Plugin stopped", nil)
	return nil
}

// UnloadPlugin unloads a plugin completely
func (m *PluginManager) UnloadPlugin(ctx context.Context, name string) error {
	m.setPluginStatus(name, PluginStateUnloading, "Unloading plugin", nil)
	
	if err := m.loader.UnloadPlugin(name); err != nil {
		m.setPluginStatus(name, PluginStateError, "Failed to unload", err)
		return err
	}
	
	// Remove from status tracking
	m.mu.Lock()
	delete(m.status, name)
	m.mu.Unlock()
	
	return nil
}

// GetPluginStatus returns the current status of a plugin
func (m *PluginManager) GetPluginStatus(name string) (PluginStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status, exists := m.status[name]
	return status, exists
}

// ListPluginStatuses returns the status of all tracked plugins
func (m *PluginManager) ListPluginStatuses() map[string]PluginStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	statuses := make(map[string]PluginStatus, len(m.status))
	for name, status := range m.status {
		statuses[name] = status
	}
	
	return statuses
}

// DiscoverAndLoadPlugins discovers and loads all plugins from a directory
func (m *PluginManager) DiscoverAndLoadPlugins(ctx context.Context, dir string) (int, error) {
	configs, err := m.discovery.DiscoverPlugins(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to discover plugins: %w", err)
	}
	
	loaded := 0
	for _, config := range configs {
		if !config.Enabled {
			m.setPluginStatus(config.Name, PluginStateDiscovered, "Plugin disabled", nil)
			continue
		}
		
		m.setPluginStatus(config.Name, PluginStateDiscovered, "Plugin discovered", nil)
		
		if err := m.LoadPlugin(config); err != nil {
			fmt.Printf("Warning: failed to load plugin %s: %v\n", config.Name, err)
			continue
		}
		
		// Auto-start plugins after loading
		if err := m.StartPlugin(ctx, config.Name); err != nil {
			fmt.Printf("Warning: failed to start plugin %s: %v\n", config.Name, err)
			continue
		}
		
		loaded++
	}
	
	return loaded, nil
}

// Shutdown gracefully shuts down all plugins
func (m *PluginManager) Shutdown(ctx context.Context) error {
	plugins := m.registry.ListPlugins()
	
	var errors []error
	for _, plugin := range plugins {
		name := plugin.GetMetadata().Name
		if err := m.StopPlugin(ctx, name); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to stop %d plugins: %v", len(errors), errors)
	}
	
	return nil
}

// setPluginStatus updates the status of a plugin (thread-safe)
func (m *PluginManager) setPluginStatus(name string, state PluginState, message string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.status[name] = PluginStatus{
		State:   state,
		Message: message,
		Error:   err,
	}
}