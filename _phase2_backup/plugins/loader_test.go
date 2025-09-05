package plugins

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginLoader_LoadPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	// Register a factory
	factory := &mockPluginFactory{
		pluginType: "input",
		createFunc: func(config PluginConfig) (Plugin, error) {
			return &mockPlugin{
				metadata: PluginMetadata{
					Name:    config.Name,
					Type:    config.Type,
					Version: config.Version,
				},
			}, nil
		},
	}
	require.NoError(t, factoryRegistry.RegisterFactory("input", factory))
	
	// Test loading a plugin
	config := PluginConfig{
		Name:    "test-input",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"url": "https://example.com",
		},
	}
	
	err := loader.LoadPlugin(config)
	require.NoError(t, err)
	
	// Verify plugin was registered
	plugin, exists := registry.GetPlugin("test-input")
	assert.True(t, exists)
	assert.NotNil(t, plugin)
	assert.Equal(t, "test-input", plugin.GetMetadata().Name)
}

func TestPluginLoader_LoadPlugin_DisabledPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	config := PluginConfig{
		Name:    "disabled-plugin",
		Type:    "input",
		Enabled: false,
	}
	
	err := loader.LoadPlugin(config)
	require.NoError(t, err)
	
	// Verify plugin was not registered
	_, exists := registry.GetPlugin("disabled-plugin")
	assert.False(t, exists)
}

func TestPluginLoader_LoadPlugin_MissingFactory(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "unknown",
		Version: "1.0.0",
		Enabled: true,
	}
	
	err := loader.LoadPlugin(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid plugin type: unknown")
}

func TestPluginLoader_LoadPlugin_InitializationError(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	// Register a factory that returns an error
	factory := &mockPluginFactory{
		pluginType: "input",
		createFunc: func(config PluginConfig) (Plugin, error) {
			return nil, fmt.Errorf("initialization failed")
		},
	}
	require.NoError(t, factoryRegistry.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "failing-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	err := loader.LoadPlugin(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initialization failed")
}

func TestPluginLoader_LoadFromDirectory(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	// Register factories
	inputFactory := &mockPluginFactory{
		pluginType: "input",
		createFunc: func(config PluginConfig) (Plugin, error) {
			return &mockPlugin{
				metadata: PluginMetadata{
					Name:    config.Name,
					Type:    "input",
					Version: config.Version,
				},
			}, nil
		},
	}
	outputFactory := &mockPluginFactory{
		pluginType: "output",
		createFunc: func(config PluginConfig) (Plugin, error) {
			return &mockPlugin{
				metadata: PluginMetadata{
					Name:    config.Name,
					Type:    "output",
					Version: config.Version,
				},
			}, nil
		},
	}
	
	require.NoError(t, factoryRegistry.RegisterFactory("input", inputFactory))
	require.NoError(t, factoryRegistry.RegisterFactory("output", outputFactory))
	
	// Create a test directory with plugin configs
	testDir := t.TempDir()
	createTestPluginConfig(t, testDir, "input-plugin.yaml", PluginConfig{
		Name:    "test-input",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	})
	createTestPluginConfig(t, testDir, "output-plugin.yaml", PluginConfig{
		Name:    "test-output",
		Type:    "output",
		Version: "1.0.0",
		Enabled: true,
	})
	createTestPluginConfig(t, testDir, "disabled-plugin.yaml", PluginConfig{
		Name:    "disabled",
		Type:    "input",
		Version: "1.0.0",
		Enabled: false,
	})
	
	// Load plugins from directory
	loaded, err := loader.LoadFromDirectory(testDir)
	require.NoError(t, err)
	assert.Equal(t, 2, loaded) // Only enabled plugins
	
	// Verify plugins were registered
	_, exists := registry.GetPlugin("test-input")
	assert.True(t, exists)
	_, exists = registry.GetPlugin("test-output")
	assert.True(t, exists)
	_, exists = registry.GetPlugin("disabled")
	assert.False(t, exists)
}

func TestPluginLoader_UnloadPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	factoryRegistry := NewFactoryRegistry()
	loader := NewPluginLoader(registry, factoryRegistry)
	
	// First load a plugin
	factory := &mockPluginFactory{
		pluginType: "input",
	}
	require.NoError(t, factoryRegistry.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	require.NoError(t, loader.LoadPlugin(config))
	
	// Verify plugin exists
	plugin, exists := registry.GetPlugin("test-plugin")
	require.True(t, exists)
	
	// Cast to check if it implements lifecycle methods
	if lifecycle, ok := plugin.(interface {
		Stop(context.Context) error
	}); ok {
		// Plugin should be stopped during unload
		_ = lifecycle
	}
	
	// Unload the plugin
	err := loader.UnloadPlugin("test-plugin")
	require.NoError(t, err)
	
	// Verify plugin was removed
	_, exists = registry.GetPlugin("test-plugin")
	assert.False(t, exists)
}

