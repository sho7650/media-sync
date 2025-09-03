package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactoryRegistry_RegisterFactory(t *testing.T) {
	registry := NewFactoryRegistry()
	
	factory := &mockPluginFactory{pluginType: "input"}
	err := registry.RegisterFactory("input", factory)
	require.NoError(t, err)
	
	// Test duplicate registration
	err = registry.RegisterFactory("input", factory)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "factory for type 'input' already registered")
}

func TestFactoryRegistry_GetFactory(t *testing.T) {
	registry := NewFactoryRegistry()
	
	factory := &mockPluginFactory{pluginType: "input"}
	err := registry.RegisterFactory("input", factory)
	require.NoError(t, err)
	
	// Test retrieving existing factory
	retrieved, err := registry.GetFactory("input")
	require.NoError(t, err)
	assert.Equal(t, factory, retrieved)
	
	// Test retrieving non-existent factory
	_, err = registry.GetFactory("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "factory for type 'nonexistent' not found")
}

func TestPluginFactory_CreatePlugin(t *testing.T) {
	factory := &mockPluginFactory{
		pluginType: "input",
		createFunc: func(config PluginConfig) (Plugin, error) {
			plugin := &mockPlugin{
				metadata: PluginMetadata{
					Name:    config.Name,
					Type:    config.Type,
					Version: config.Version,
				},
			}
			return plugin, nil
		},
	}
	
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"key": "value",
		},
	}
	
	plugin, err := factory.CreatePlugin(config)
	require.NoError(t, err)
	assert.NotNil(t, plugin)
	
	metadata := plugin.GetMetadata()
	assert.Equal(t, "test-plugin", metadata.Name)
	assert.Equal(t, "input", metadata.Type)
	assert.Equal(t, "1.0.0", metadata.Version)
}

func TestPluginFactoryFunc_Adapter(t *testing.T) {
	called := false
	factoryFunc := PluginFactoryFunc(func(config PluginConfig) (Plugin, error) {
		called = true
		return &mockPlugin{
			metadata: PluginMetadata{
				Name: config.Name,
				Type: "input",
			},
		}, nil
	})
	
	config := PluginConfig{
		Name: "test-plugin",
		Type: "input",
	}
	
	plugin, err := factoryFunc.CreatePlugin(config)
	require.NoError(t, err)
	assert.NotNil(t, plugin)
	assert.True(t, called)
	assert.Equal(t, "test-plugin", plugin.GetMetadata().Name)
}

func TestFactoryRegistry_ListFactories(t *testing.T) {
	registry := NewFactoryRegistry()
	
	// Register multiple factories
	inputFactory := &mockPluginFactory{pluginType: "input"}
	outputFactory := &mockPluginFactory{pluginType: "output"}
	transformFactory := &mockPluginFactory{pluginType: "transform"}
	
	require.NoError(t, registry.RegisterFactory("input", inputFactory))
	require.NoError(t, registry.RegisterFactory("output", outputFactory))
	require.NoError(t, registry.RegisterFactory("transform", transformFactory))
	
	types := registry.ListFactoryTypes()
	assert.Len(t, types, 3)
	assert.Contains(t, types, "input")
	assert.Contains(t, types, "output")
	assert.Contains(t, types, "transform")
}

