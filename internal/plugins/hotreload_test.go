package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPluginManager_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewPluginManager()
	
	// Register a test factory
	factory := &mockPluginFactory{pluginType: "input"}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	// Enable hot reload on the temp directory
	require.NoError(t, manager.EnableHotReload(tmpDir))
	defer manager.DisableHotReload()
	
	// Create initial plugin configuration
	configPath := filepath.Join(tmpDir, "test-plugin.yaml")
	initialConfig := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"timeout": 5000,
		},
	}
	
	// Write initial configuration
	data, err := yaml.Marshal(initialConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	
	// Wait for plugin to be loaded
	time.Sleep(200 * time.Millisecond)
	
	// Verify plugin was loaded
	status, exists := manager.GetPluginStatus("test-plugin")
	assert.True(t, exists, "Plugin should be loaded")
	assert.Equal(t, PluginStateLoaded, status.State)
	
	// Start the plugin
	ctx := context.Background()
	require.NoError(t, manager.StartPlugin(ctx, "test-plugin"))
	
	// Verify plugin is running
	status, exists = manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
	
	// Modify configuration
	updatedConfig := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "2.0.0", // Version change
		Enabled: true,
		Settings: map[string]interface{}{
			"timeout": 10000, // Setting change
			"debug":   true,  // New setting
		},
	}
	
	// Write updated configuration
	data, err = yaml.Marshal(updatedConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	
	// Wait for hot reload to complete
	time.Sleep(300 * time.Millisecond)
	
	// Verify plugin was reloaded and is running
	status, exists = manager.GetPluginStatus("test-plugin")
	assert.True(t, exists, "Plugin should still exist after reload")
	assert.Equal(t, PluginStateRunning, status.State, "Plugin should be running after reload")
	
	// Verify new configuration was applied
	plugin, exists := manager.registry.GetPlugin("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, "2.0.0", plugin.GetMetadata().Version, "Version should be updated")
	
	// Delete the configuration file
	require.NoError(t, os.Remove(configPath))
	
	// Wait for plugin to be unloaded
	time.Sleep(200 * time.Millisecond)
	
	// Verify plugin was unloaded
	_, exists = manager.GetPluginStatus("test-plugin")
	assert.False(t, exists, "Plugin should be unloaded after config deletion")
}

func TestPluginManager_HotReloadWithError(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewPluginManager()
	
	// Register a test factory that fails on version 2.0.0
	factory := &mockFailingPluginFactory{
		failOnVersion: "2.0.0",
		pluginType:    "input",
	}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	// Enable hot reload
	require.NoError(t, manager.EnableHotReload(tmpDir))
	defer manager.DisableHotReload()
	
	// Create initial plugin configuration
	configPath := filepath.Join(tmpDir, "fail-plugin.yaml")
	initialConfig := PluginConfig{
		Name:    "fail-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	// Write initial configuration
	data, err := yaml.Marshal(initialConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	
	// Wait for plugin to be loaded
	time.Sleep(200 * time.Millisecond)
	
	// Start the plugin
	ctx := context.Background()
	require.NoError(t, manager.StartPlugin(ctx, "fail-plugin"))
	
	// Verify plugin is running with version 1.0.0
	status, exists := manager.GetPluginStatus("fail-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
	
	// Update to failing version
	failingConfig := PluginConfig{
		Name:    "fail-plugin",
		Type:    "input",
		Version: "2.0.0", // This version will fail
		Enabled: true,
	}
	
	// Write failing configuration
	data, err = yaml.Marshal(failingConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	
	// Wait for hot reload attempt
	time.Sleep(300 * time.Millisecond)
	
	// Plugin should remain in error state or be rolled back
	// The exact behavior depends on the error handling strategy
	status, exists = manager.GetPluginStatus("fail-plugin")
	assert.True(t, exists, "Plugin should still exist after failed reload")
	// Plugin might be in error state or stopped state after failed reload
	assert.NotEqual(t, PluginStateRunning, status.State, "Plugin should not be running after failed reload")
}

// mockFailingPluginFactory is a factory that fails for specific versions
type mockFailingPluginFactory struct {
	failOnVersion string
	pluginType    string
}

func (f *mockFailingPluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	if config.Version == f.failOnVersion {
		return nil, fmt.Errorf("intentional failure for version %s", config.Version)
	}
	
	return &mockPlugin{
		metadata: PluginMetadata{
			Name:    config.Name,
			Type:    f.pluginType,
			Version: config.Version,
		},
	}, nil
}

func (f *mockFailingPluginFactory) GetType() string {
	return f.pluginType
}