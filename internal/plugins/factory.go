package plugins

import (
	"fmt"
	"sync"
)

// PluginFactory creates plugin instances
type PluginFactory interface {
	CreatePlugin(config PluginConfig) (Plugin, error)
	GetType() string
}

// PluginFactoryFunc is a function adapter for PluginFactory
type PluginFactoryFunc func(PluginConfig) (Plugin, error)

// CreatePlugin implements PluginFactory interface for function type
func (f PluginFactoryFunc) CreatePlugin(config PluginConfig) (Plugin, error) {
	return f(config)
}

// GetType returns "generic" for function-based factories
func (f PluginFactoryFunc) GetType() string {
	return "generic"
}

// FactoryRegistry manages plugin factories
type FactoryRegistry struct {
	mu        sync.RWMutex
	factories map[string]PluginFactory
}

// NewFactoryRegistry creates a new factory registry
func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{
		factories: make(map[string]PluginFactory),
	}
}

// RegisterFactory registers a plugin factory for a specific type
func (r *FactoryRegistry) RegisterFactory(pluginType string, factory PluginFactory) error {
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}
	
	if pluginType == "" {
		return fmt.Errorf("plugin type cannot be empty")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.factories[pluginType]; exists {
		return fmt.Errorf("%w: factory for type '%s' already registered", ErrFactoryExists, pluginType)
	}
	
	r.factories[pluginType] = factory
	return nil
}

// GetFactory retrieves a factory by plugin type
func (r *FactoryRegistry) GetFactory(pluginType string) (PluginFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	factory, exists := r.factories[pluginType]
	if !exists {
		return nil, fmt.Errorf("%w: factory for type '%s' not found", ErrFactoryNotFound, pluginType)
	}
	
	return factory, nil
}

// ListFactoryTypes returns all registered factory types
func (r *FactoryRegistry) ListFactoryTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	types := make([]string, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	
	return types
}

// UnregisterFactory removes a factory from the registry
func (r *FactoryRegistry) UnregisterFactory(pluginType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.factories[pluginType]; !exists {
		return fmt.Errorf("factory for type '%s' not found", pluginType)
	}
	
	delete(r.factories, pluginType)
	return nil
}