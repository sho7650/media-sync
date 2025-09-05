package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginLoader handles dynamic plugin loading
type PluginLoader struct {
	registry  *PluginRegistry
	factories *FactoryRegistry
	discovery *PluginDiscovery
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader(registry *PluginRegistry, factories *FactoryRegistry) *PluginLoader {
	return &PluginLoader{
		registry:  registry,
		factories: factories,
		discovery: NewPluginDiscovery(),
	}
}

// LoadPlugin loads a plugin from configuration
func (l *PluginLoader) LoadPlugin(config PluginConfig) error {
	// Skip disabled plugins
	if !config.Enabled {
		return nil
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return NewPluginError(config.Name, "validation", err)
	}
	
	// Get factory for plugin type
	factory, err := l.factories.GetFactory(config.Type)
	if err != nil {
		return NewPluginError(config.Name, "factory lookup", err)
	}
	
	// Create plugin instance
	plugin, err := factory.CreatePlugin(config)
	if err != nil {
		return NewPluginError(config.Name, "creation", err)
	}
	
	// Configure plugin if it has settings
	if len(config.Settings) > 0 {
		if err := plugin.Configure(config.Settings); err != nil {
			return NewPluginError(config.Name, "configuration", err)
		}
	}
	
	// Register plugin
	if err := l.registry.RegisterPlugin(plugin); err != nil {
		return NewPluginError(config.Name, "registration", err)
	}
	
	return nil
}

// LoadFromDirectory discovers and loads plugins from a directory
func (l *PluginLoader) LoadFromDirectory(dir string) (int, error) {
	configs, err := l.discovery.DiscoverPlugins(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to discover plugins: %w", err)
	}
	
	// Filter enabled plugins first
	enabledConfigs := make([]PluginConfig, 0, len(configs))
	for _, config := range configs {
		if config.Enabled {
			enabledConfigs = append(enabledConfigs, config)
		}
	}
	
	loaded := 0
	for _, config := range enabledConfigs {
		if err := l.LoadPlugin(config); err != nil {
			// Log error but continue loading other plugins
			fmt.Printf("Warning: failed to load plugin %s: %v\n", config.Name, err)
			continue
		}
		loaded++
	}
	
	return loaded, nil
}

// UnloadPlugin stops and removes a plugin from the registry
func (l *PluginLoader) UnloadPlugin(name string) error {
	plugin, exists := l.registry.GetPlugin(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	
	// Stop plugin if it implements lifecycle methods
	if lifecycle, ok := plugin.(interface {
		Stop(context.Context) error
	}); ok {
		ctx := context.Background()
		if err := lifecycle.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop plugin %s: %w", name, err)
		}
	}
	
	// Unregister plugin
	if err := l.registry.UnregisterPlugin(name); err != nil {
		return fmt.Errorf("failed to unregister plugin %s: %w", name, err)
	}
	
	return nil
}

// ReloadPlugin reloads a plugin with new configuration
func (l *PluginLoader) ReloadPlugin(config PluginConfig) error {
	// Unload existing plugin if it exists
	if _, exists := l.registry.GetPlugin(config.Name); exists {
		if err := l.UnloadPlugin(config.Name); err != nil {
			return fmt.Errorf("failed to unload plugin for reload: %w", err)
		}
	}
	
	// Load plugin with new configuration
	return l.LoadPlugin(config)
}

// LoadConfigFromFile loads a plugin configuration from a YAML file
func (l *PluginLoader) LoadConfigFromFile(path string) (PluginConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginConfig{}, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config PluginConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return PluginConfig{}, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	return config, nil
}

// LoadPluginsFromPattern loads all plugins matching a file pattern
func (l *PluginLoader) LoadPluginsFromPattern(pattern string) (int, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid pattern: %w", err)
	}
	
	loaded := 0
	for _, path := range matches {
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		
		config, err := l.LoadConfigFromFile(path)
		if err != nil {
			fmt.Printf("Warning: failed to load config from %s: %v\n", path, err)
			continue
		}
		
		if !config.Enabled {
			continue
		}
		
		if err := l.LoadPlugin(config); err != nil {
			fmt.Printf("Warning: failed to load plugin from %s: %v\n", path, err)
			continue
		}
		loaded++
	}
	
	return loaded, nil
}