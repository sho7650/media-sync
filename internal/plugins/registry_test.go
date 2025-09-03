package plugins

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

func TestPlugin_InterfaceCompliance(t *testing.T) {
	// Test that Plugin interface can be implemented
	var _ Plugin = (*mockPlugin)(nil)
	var _ interfaces.Service = (*mockPlugin)(nil)
}

func TestPluginRegistry_RegisterPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	
	plugin := &mockPlugin{
		metadata: PluginMetadata{
			Name:        "test-plugin",
			Type:        "input",
			Version:     "1.0.0",
			Description: "Test plugin for unit testing",
		},
	}
	
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	
	// Test duplicate registration prevention
	err = registry.RegisterPlugin(plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestPluginRegistry_GetPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	
	plugin := &mockPlugin{
		metadata: PluginMetadata{
			Name:    "test-plugin",
			Type:    "input",
			Version: "1.0.0",
		},
	}
	
	// Test getting non-existent plugin
	retrieved, exists := registry.GetPlugin("non-existent")
	assert.False(t, exists)
	assert.Nil(t, retrieved)
	
	// Register and retrieve plugin
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	
	retrieved, exists = registry.GetPlugin("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, plugin, retrieved)
}

func TestPluginRegistry_UnregisterPlugin(t *testing.T) {
	registry := NewPluginRegistry()
	
	plugin := &mockPlugin{
		metadata: PluginMetadata{
			Name: "test-plugin",
			Type: "input",
			Version: "1.0.0",
		},
	}
	
	// Register plugin
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)
	
	// Unregister plugin
	err = registry.UnregisterPlugin("test-plugin")
	require.NoError(t, err)
	
	// Verify plugin is removed
	_, exists := registry.GetPlugin("test-plugin")
	assert.False(t, exists)
	
	// Test unregistering non-existent plugin
	err = registry.UnregisterPlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPluginRegistry_ListPlugins(t *testing.T) {
	registry := NewPluginRegistry()
	
	// Test empty registry
	plugins := registry.ListPlugins()
	assert.Empty(t, plugins)
	
	// Register multiple plugins
	plugin1 := &mockPlugin{
		metadata: PluginMetadata{Name: "plugin1", Type: "input", Version: "1.0.0"},
	}
	plugin2 := &mockPlugin{
		metadata: PluginMetadata{Name: "plugin2", Type: "output", Version: "1.0.0"},
	}
	
	var err error
	err = registry.RegisterPlugin(plugin1)
	require.NoError(t, err)
	err = registry.RegisterPlugin(plugin2)
	require.NoError(t, err)
	
	plugins = registry.ListPlugins()
	assert.Len(t, plugins, 2)
}

func TestPluginRegistry_ListPluginsByType(t *testing.T) {
	registry := NewPluginRegistry()
	
	inputPlugin := &mockPlugin{
		metadata: PluginMetadata{Name: "input1", Type: "input", Version: "1.0.0"},
	}
	outputPlugin := &mockPlugin{
		metadata: PluginMetadata{Name: "output1", Type: "output", Version: "1.0.0"},
	}
	
	var err error
	err = registry.RegisterPlugin(inputPlugin)
	require.NoError(t, err)
	err = registry.RegisterPlugin(outputPlugin)
	require.NoError(t, err)
	
	// Test filtering by type
	inputPlugins := registry.ListPluginsByType("input")
	assert.Len(t, inputPlugins, 1)
	assert.Equal(t, "input1", inputPlugins[0].GetMetadata().Name)
	
	outputPlugins := registry.ListPluginsByType("output")
	assert.Len(t, outputPlugins, 1)
	assert.Equal(t, "output1", outputPlugins[0].GetMetadata().Name)
}

func TestPluginMetadata_Validation(t *testing.T) {
	tests := []struct {
		name     string
		metadata PluginMetadata
		wantErr  string
	}{
		{
			name: "valid metadata",
			metadata: PluginMetadata{
				Name:        "test-plugin",
				Type:        "input",
				Version:     "1.0.0",
				Description: "Test plugin",
			},
			wantErr: "",
		},
		{
			name: "empty name",
			metadata: PluginMetadata{
				Type:    "input",
				Version: "1.0.0",
			},
			wantErr: "name cannot be empty",
		},
		{
			name: "invalid type",
			metadata: PluginMetadata{
				Name:    "test",
				Type:    "invalid",
				Version: "1.0.0",
			},
			wantErr: "type must be input, output, or transform",
		},
		{
			name: "empty version",
			metadata: PluginMetadata{
				Name: "test",
				Type: "input",
			},
			wantErr: "version cannot be empty",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPluginRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewPluginRegistry()
	
	// Test concurrent registration
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			plugin := &mockPlugin{
				metadata: PluginMetadata{
					Name:    fmt.Sprintf("plugin-%d", id),
					Type:    "input",
					Version: "1.0.0",
				},
			}
			err := registry.RegisterPlugin(plugin)
			assert.NoError(t, err)
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// Verify all plugins were registered
	plugins := registry.ListPlugins()
	assert.Len(t, plugins, numGoroutines)
}