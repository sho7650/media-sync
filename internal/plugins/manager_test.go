package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginManager_LoadAndStartPlugin(t *testing.T) {
	manager := NewPluginManager()
	
	// Register a factory
	factory := &mockPluginFactory{
		pluginType: "input",
	}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	
	// Load plugin
	err := manager.LoadPlugin(config)
	require.NoError(t, err)
	
	// Check status
	status, exists := manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateLoaded, status.State)
	
	// Start plugin
	err = manager.StartPlugin(ctx, "test-plugin")
	require.NoError(t, err)
	
	// Check status after start
	status, exists = manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
}

func TestPluginManager_EnableHotReload(t *testing.T) {
	manager := NewPluginManager()
	configDir := t.TempDir()
	
	err := manager.EnableHotReload(configDir)
	assert.NoError(t, err)
	
	// Verify hot reload is enabled
	assert.True(t, manager.IsHotReloadEnabled())
}

func TestPluginManager_DisableHotReload(t *testing.T) {
	manager := NewPluginManager()
	configDir := t.TempDir()
	
	// Enable first
	require.NoError(t, manager.EnableHotReload(configDir))
	assert.True(t, manager.IsHotReloadEnabled())
	
	// Then disable
	err := manager.DisableHotReload()
	assert.NoError(t, err)
	assert.False(t, manager.IsHotReloadEnabled())
}

func TestPluginManager_DiscoverAndLoadPlugins(t *testing.T) {
	manager := NewPluginManager()
	
	// Register factories
	require.NoError(t, manager.RegisterFactory("input", &mockPluginFactory{pluginType: "input"}))
	require.NoError(t, manager.RegisterFactory("output", &mockPluginFactory{pluginType: "output"}))
	
	// Create test directory
	testDir := t.TempDir()
	createTestPluginConfig(t, testDir, "input.yaml", PluginConfig{
		Name:    "test-input",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	})
	createTestPluginConfig(t, testDir, "output.yaml", PluginConfig{
		Name:    "test-output",
		Type:    "output",
		Version: "1.0.0",
		Enabled: true,
	})
	createTestPluginConfig(t, testDir, "disabled.yaml", PluginConfig{
		Name:    "disabled",
		Type:    "input",
		Version: "1.0.0",
		Enabled: false,
	})
	
	ctx := context.Background()
	loaded, err := manager.DiscoverAndLoadPlugins(ctx, testDir)
	require.NoError(t, err)
	assert.Equal(t, 2, loaded)
	
	// Verify plugin statuses
	statuses := manager.ListPluginStatuses()
	assert.Len(t, statuses, 3) // Including disabled plugin
	
	assert.Equal(t, PluginStateRunning, statuses["test-input"].State)
	assert.Equal(t, PluginStateRunning, statuses["test-output"].State)
	assert.Equal(t, PluginStateDiscovered, statuses["disabled"].State)
}

func TestPluginManager_Shutdown(t *testing.T) {
	manager := NewPluginManager()
	
	// Register factory and load plugins
	require.NoError(t, manager.RegisterFactory("input", &mockPluginFactory{pluginType: "input"}))
	
	configs := []PluginConfig{
		{Name: "plugin1", Type: "input", Version: "1.0.0", Enabled: true},
		{Name: "plugin2", Type: "input", Version: "1.0.0", Enabled: true},
	}
	
	ctx := context.Background()
	for _, config := range configs {
		require.NoError(t, manager.LoadPlugin(config))
		require.NoError(t, manager.StartPlugin(ctx, config.Name))
	}
	
	// Shutdown all plugins
	err := manager.Shutdown(ctx)
	require.NoError(t, err)
	
	// Verify all plugins are stopped
	statuses := manager.ListPluginStatuses()
	for _, status := range statuses {
		assert.Equal(t, PluginStateStopped, status.State)
	}
}

func TestPluginManager_UnloadPlugin(t *testing.T) {
	manager := NewPluginManager()
	
	// Load a plugin
	require.NoError(t, manager.RegisterFactory("input", &mockPluginFactory{pluginType: "input"}))
	
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "test-plugin"))
	
	// Verify plugin exists
	_, exists := manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	
	// Unload plugin
	err := manager.UnloadPlugin(ctx, "test-plugin")
	require.NoError(t, err)
	
	// Verify plugin status is removed
	_, exists = manager.GetPluginStatus("test-plugin")
	assert.False(t, exists)
	
	// Verify plugin is removed from registry
	_, exists = manager.registry.GetPlugin("test-plugin")
	assert.False(t, exists)
}