package plugins

import (
	"fmt"
	"sync"
)

// PluginRegistry provides thread-safe plugin registration and lookup
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
	}
}

// RegisterPlugin registers a plugin in the registry
func (r *PluginRegistry) RegisterPlugin(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	metadata := plugin.GetMetadata()
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("invalid plugin metadata: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := metadata.Name
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = plugin
	return nil
}

// GetPlugin retrieves a plugin by name
func (r *PluginRegistry) GetPlugin(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

// UnregisterPlugin removes a plugin from the registry
func (r *PluginRegistry) UnregisterPlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	delete(r.plugins, name)
	return nil
}

// ListPlugins returns all registered plugins
func (r *PluginRegistry) ListPlugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// ListPluginsByType returns plugins filtered by type
func (r *PluginRegistry) ListPluginsByType(pluginType string) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Plugin
	for _, plugin := range r.plugins {
		if plugin.GetMetadata().Type == pluginType {
			filtered = append(filtered, plugin)
		}
	}
	return filtered
}