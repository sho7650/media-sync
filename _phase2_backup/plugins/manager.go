package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

// PluginManager coordinates plugin lifecycle operations
type PluginManager struct {
	registry  *PluginRegistry
	factories *FactoryRegistry
	loader    *PluginLoader
	discovery *PluginDiscovery
	
	mu     sync.RWMutex
	status map[string]PluginStatus
	
	// Health monitoring
	healthMonitor   *HealthMonitor
	lifecycleHooks  map[string][]LifecycleHook
	resourceTracker *ResourceTracker
}

// PluginStatus represents the current status of a plugin
type PluginStatus struct {
	State   PluginState `json:"state"`
	Message string      `json:"message,omitempty"`
	Error   error       `json:"-"`
}

// Health monitoring types
type HealthMonitor struct {
	interval     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	eventChan    chan<- HealthEvent
	autoRecover  bool
	running      bool
}

type LifecycleHook func(ctx context.Context, pluginName string) error

type ResourceTracker struct {
	usage map[string]ResourceUsage
	mu    sync.RWMutex
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
	// Execute pre-start hooks
	if err := m.executeLifecycleHooks(ctx, "pre-start", name); err != nil {
		return fmt.Errorf("pre-start hook failed for plugin %s: %w", name, err)
	}
	
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
	
	// Track initial resource usage
	m.trackPluginResources(name, ResourceUsage{
		MemoryBytes: 1024 * 1024, // 1MB default
		Connections: 2,           // 2 connections default
	})
	
	m.setPluginStatus(name, PluginStateRunning, "Plugin running", nil)
	
	// Execute post-start hooks
	if err := m.executeLifecycleHooks(ctx, "post-start", name); err != nil {
		fmt.Printf("Warning: post-start hook failed for plugin %s: %v\n", name, err)
	}
	
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
	
	// Clear resource tracking
	m.trackPluginResources(name, ResourceUsage{
		MemoryBytes: 0,
		Connections: 0,
	})
	
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

// StartHealthMonitoring starts the health monitoring system
func (m *PluginManager) StartHealthMonitoring(ctx context.Context, eventChan chan<- HealthEvent, interval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.healthMonitor != nil && m.healthMonitor.running {
		return fmt.Errorf("health monitoring already running")
	}
	
	monitorCtx, cancel := context.WithCancel(ctx)
	m.healthMonitor = &HealthMonitor{
		interval:  interval,
		ctx:       monitorCtx,
		cancel:    cancel,
		eventChan: eventChan,
		running:   true,
	}
	
	go m.runHealthMonitoring()
	return nil
}

// StartHealthMonitoringWithRecovery starts health monitoring with auto-recovery
func (m *PluginManager) StartHealthMonitoringWithRecovery(ctx context.Context, eventChan chan<- HealthEvent, interval time.Duration) error {
	if err := m.StartHealthMonitoring(ctx, eventChan, interval); err != nil {
		return err
	}
	
	m.healthMonitor.autoRecover = true
	return nil
}

// StopHealthMonitoring stops the health monitoring system
func (m *PluginManager) StopHealthMonitoring() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.healthMonitor == nil || !m.healthMonitor.running {
		return nil
	}
	
	m.healthMonitor.cancel()
	m.healthMonitor.running = false
	return nil
}

// RegisterLifecycleHook registers a hook for lifecycle events
func (m *PluginManager) RegisterLifecycleHook(event string, hook LifecycleHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.lifecycleHooks == nil {
		m.lifecycleHooks = make(map[string][]LifecycleHook)
	}
	
	m.lifecycleHooks[event] = append(m.lifecycleHooks[event], hook)
}

// GetPluginResourceUsage returns resource usage for a plugin
func (m *PluginManager) GetPluginResourceUsage(pluginName string) (ResourceUsage, error) {
	if m.resourceTracker == nil {
		m.resourceTracker = &ResourceTracker{
			usage: make(map[string]ResourceUsage),
		}
	}
	
	m.resourceTracker.mu.RLock()
	defer m.resourceTracker.mu.RUnlock()
	
	usage, exists := m.resourceTracker.usage[pluginName]
	if !exists {
		return ResourceUsage{}, fmt.Errorf("resource usage not tracked for plugin %s", pluginName)
	}
	
	return usage, nil
}

// GracefulShutdown performs graceful shutdown with timeout
func (m *PluginManager) GracefulShutdown(ctx context.Context, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	if err := m.StopHealthMonitoring(); err != nil {
		return fmt.Errorf("failed to stop health monitoring: %w", err)
	}
	
	plugins := m.registry.ListPlugins()
	
	var errors []error
	for _, plugin := range plugins {
		name := plugin.GetMetadata().Name
		if err := m.StopPlugin(shutdownCtx, name); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to stop %d plugins during graceful shutdown: %v", len(errors), errors)
	}
	
	return nil
}

// runHealthMonitoring is the background health monitoring loop
func (m *PluginManager) runHealthMonitoring() {
	ticker := time.NewTicker(m.healthMonitor.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.healthMonitor.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthChecks()
		}
	}
}

// performHealthChecks checks health of all running plugins
func (m *PluginManager) performHealthChecks() {
	plugins := m.registry.ListPlugins()
	
	for _, plugin := range plugins {
		name := plugin.GetMetadata().Name
		health := plugin.Health()
		
		event := HealthEvent{
			PluginName: name,
			Health:     health,
		}
		
		if health.Status == interfaces.StatusError && m.healthMonitor.autoRecover {
			if err := m.attemptAutoRecovery(name); err == nil {
				event.AutoRecoveryAttempted = true
				event.RecoverySuccess = true
				event.Health = plugin.Health()
			} else {
				event.AutoRecoveryAttempted = true
				event.RecoverySuccess = false
			}
		}
		
		select {
		case m.healthMonitor.eventChan <- event:
		default:
		}
	}
}

// attemptAutoRecovery tries to restart a failed plugin
func (m *PluginManager) attemptAutoRecovery(pluginName string) error {
	ctx := context.Background()
	
	if err := m.StopPlugin(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to stop plugin during recovery: %w", err)
	}
	
	if err := m.StartPlugin(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to restart plugin during recovery: %w", err)
	}
	
	return nil
}

// executeLifecycleHooks executes hooks for a lifecycle event
func (m *PluginManager) executeLifecycleHooks(ctx context.Context, event, pluginName string) error {
	m.mu.RLock()
	hooks := m.lifecycleHooks[event]
	m.mu.RUnlock()
	
	for _, hook := range hooks {
		if err := hook(ctx, pluginName); err != nil {
			return err
		}
	}
	
	return nil
}

// trackPluginResources tracks resource usage for a plugin
func (m *PluginManager) trackPluginResources(pluginName string, usage ResourceUsage) {
	if m.resourceTracker == nil {
		m.resourceTracker = &ResourceTracker{
			usage: make(map[string]ResourceUsage),
		}
	}
	
	m.resourceTracker.mu.Lock()
	defer m.resourceTracker.mu.Unlock()
	
	m.resourceTracker.usage[pluginName] = usage
}
