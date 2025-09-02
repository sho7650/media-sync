package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPluginDiscovery_DiscoverPlugins(t *testing.T) {
	discovery := NewPluginDiscovery()
	
	// Create test directory with plugin configs
	testDir := t.TempDir()
	
	// Create valid plugin config
	validConfig := PluginConfig{
		Name:        "test-input",
		Type:        "input",
		Version:     "1.0.0",
		Description: "Test input plugin",
		Enabled:     true,
		Settings: map[string]interface{}{
			"url":     "https://example.com",
			"timeout": 30,
		},
	}
	createTestPluginConfig(t, testDir, "input.yaml", validConfig)
	
	// Create another valid config
	outputConfig := PluginConfig{
		Name:    "test-output",
		Type:    "output",
		Version: "2.0.0",
		Enabled: false,
		Settings: map[string]interface{}{
			"path": "/tmp/output",
		},
	}
	createTestPluginConfig(t, testDir, "output.yml", outputConfig)
	
	// Create invalid YAML file
	invalidFile := filepath.Join(testDir, "invalid.yaml")
	err := os.WriteFile(invalidFile, []byte("invalid: yaml: content:"), 0644)
	require.NoError(t, err)
	
	// Create non-YAML file (should be ignored)
	nonYamlFile := filepath.Join(testDir, "readme.txt")
	err = os.WriteFile(nonYamlFile, []byte("This is not a YAML file"), 0644)
	require.NoError(t, err)
	
	// Discover plugins
	configs, err := discovery.DiscoverPlugins(testDir)
	require.NoError(t, err)
	assert.Len(t, configs, 2) // Only valid YAML configs
	
	// Verify discovered configs
	var foundInput, foundOutput bool
	for _, config := range configs {
		if config.Name == "test-input" {
			foundInput = true
			assert.Equal(t, "input", config.Type)
			assert.Equal(t, "1.0.0", config.Version)
			assert.True(t, config.Enabled)
		}
		if config.Name == "test-output" {
			foundOutput = true
			assert.Equal(t, "output", config.Type)
			assert.Equal(t, "2.0.0", config.Version)
			assert.False(t, config.Enabled)
		}
	}
	assert.True(t, foundInput)
	assert.True(t, foundOutput)
}

func TestPluginDiscovery_ValidatePluginConfig(t *testing.T) {
	discovery := NewPluginDiscovery()
	
	tests := []struct {
		name    string
		config  PluginConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: PluginConfig{
				Name:    "valid-plugin",
				Type:    "input",
				Version: "1.0.0",
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: PluginConfig{
				Type:    "input",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name is required",
		},
		{
			name: "missing type",
			config: PluginConfig{
				Name:    "test",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin type is required",
		},
		{
			name: "invalid type",
			config: PluginConfig{
				Name:    "test",
				Type:    "invalid",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "invalid plugin type",
		},
		{
			name: "missing version",
			config: PluginConfig{
				Name: "test",
				Type: "input",
			},
			wantErr: true,
			errMsg:  "plugin version is required",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := discovery.ValidatePluginConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPluginDiscovery_DiscoverPluginsRecursive(t *testing.T) {
	discovery := NewPluginDiscovery()
	
	// Create nested directory structure
	testDir := t.TempDir()
	inputDir := filepath.Join(testDir, "inputs")
	outputDir := filepath.Join(testDir, "outputs")
	nestedDir := filepath.Join(inputDir, "social")
	
	require.NoError(t, os.MkdirAll(inputDir, 0755))
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	
	// Create plugin configs in different directories
	createTestPluginConfig(t, inputDir, "rss.yaml", PluginConfig{
		Name:    "rss-input",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	})
	
	createTestPluginConfig(t, nestedDir, "tumblr.yaml", PluginConfig{
		Name:    "tumblr-input",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	})
	
	createTestPluginConfig(t, outputDir, "filesystem.yaml", PluginConfig{
		Name:    "fs-output",
		Type:    "output",
		Version: "1.0.0",
		Enabled: true,
	})
	
	// Discover plugins recursively
	configs, err := discovery.DiscoverPluginsRecursive(testDir)
	require.NoError(t, err)
	assert.Len(t, configs, 3)
	
	// Verify all plugins were discovered
	names := make([]string, len(configs))
	for i, config := range configs {
		names[i] = config.Name
	}
	assert.Contains(t, names, "rss-input")
	assert.Contains(t, names, "tumblr-input")
	assert.Contains(t, names, "fs-output")
}

func TestPluginDiscovery_LoadPluginConfig(t *testing.T) {
	discovery := NewPluginDiscovery()
	
	// Create a test config file
	testDir := t.TempDir()
	configPath := filepath.Join(testDir, "plugin.yaml")
	
	config := PluginConfig{
		Name:        "test-plugin",
		Type:        "transform",
		Version:     "1.2.3",
		Description: "A test plugin",
		Enabled:     true,
		Settings: map[string]interface{}{
			"format":     "json",
			"batch_size": 100,
			"filters": []string{
				"resize",
				"watermark",
			},
		},
	}
	
	createTestPluginConfig(t, testDir, "plugin.yaml", config)
	
	// Load the config
	loaded, err := discovery.LoadPluginConfig(configPath)
	require.NoError(t, err)
	
	assert.Equal(t, config.Name, loaded.Name)
	assert.Equal(t, config.Type, loaded.Type)
	assert.Equal(t, config.Version, loaded.Version)
	assert.Equal(t, config.Description, loaded.Description)
	assert.Equal(t, config.Enabled, loaded.Enabled)
	assert.Equal(t, config.Settings["format"], loaded.Settings["format"])
	assert.Equal(t, 100, loaded.Settings["batch_size"])
}

// Helper function to create test plugin config files
func createTestPluginConfig(t *testing.T, dir, filename string, config PluginConfig) {
	t.Helper()
	
	data, err := yaml.Marshal(config)
	require.NoError(t, err)
	
	filepath := filepath.Join(dir, filename)
	err = os.WriteFile(filepath, data, 0644)
	require.NoError(t, err)
}