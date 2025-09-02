package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager_LoadFromYAML(t *testing.T) {
	ctx := context.Background()

	t.Run("Load valid configuration", func(t *testing.T) {
		// Create temporary config file
		configContent := `
services:
  tumblr:
    name: "tumblr-input"
    type: "input"
    plugin: "tumblr"
    enabled: true
    settings:
      username: "testuser"
      fetch_limit: 100

  local_storage:
    name: "local-output"
    type: "output"
    plugin: "filesystem"
    enabled: true
    settings:
      base_path: "/tmp/media"

global:
  database:
    path: "./media-sync.db"
  workers: 5
  timeout: "30s"
`
		configFile := createTempConfigFile(t, configContent)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()

		manager := NewConfigManager()
		config, err := manager.LoadFromFile(ctx, configFile)

		require.NoError(t, err, "LoadFromFile should not return error for valid config")
		require.NotNil(t, config, "Config should not be nil")

		// Verify services
		assert.Len(t, config.Services, 2, "Should load 2 services")

		tumblrService, exists := config.Services["tumblr"]
		require.True(t, exists, "Tumblr service should exist")
		assert.Equal(t, "tumblr-input", tumblrService.Name)
		assert.Equal(t, "input", tumblrService.Type)
		assert.True(t, tumblrService.Enabled)

		// Verify global settings
		assert.Equal(t, "./media-sync.db", config.Global.Database.Path)
		assert.Equal(t, 5, config.Global.Workers)
		assert.Equal(t, "30s", config.Global.Timeout)
	})

	t.Run("Load configuration with environment variables", func(t *testing.T) {
		// Set environment variable
		err := os.Setenv("MEDIA_SYNC_DB_PATH", "/custom/db/path.db")
		require.NoError(t, err)
		defer func() {
			if err := os.Unsetenv("MEDIA_SYNC_DB_PATH"); err != nil {
				t.Logf("Failed to unset env var: %v", err)
			}
		}()

		configContent := `
global:
  database:
    path: "${MEDIA_SYNC_DB_PATH}"
  workers: 3
`
		configFile := createTempConfigFile(t, configContent)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()

		manager := NewConfigManager()
		config, err := manager.LoadFromFile(ctx, configFile)

		require.NoError(t, err)
		assert.Equal(t, "/custom/db/path.db", config.Global.Database.Path)
	})

	t.Run("Fail on invalid YAML", func(t *testing.T) {
		configContent := `
invalid: yaml: content
  - missing: quotes
    broken: [
`
		configFile := createTempConfigFile(t, configContent)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()

		manager := NewConfigManager()
		_, err := manager.LoadFromFile(ctx, configFile)

		assert.Error(t, err, "Should return error for invalid YAML")
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("Fail on missing file", func(t *testing.T) {
		manager := NewConfigManager()
		_, err := manager.LoadFromFile(ctx, "/nonexistent/config.yaml")

		assert.Error(t, err, "Should return error for missing file")
	})
}

func TestConfigManager_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid configuration passes validation", func(t *testing.T) {
		config := &Config{
			Services: map[string]ServiceConfig{
				"test": {
					Name:    "test-service",
					Type:    "input",
					Plugin:  "tumblr",
					Enabled: true,
					Settings: map[string]interface{}{
						"username": "testuser",
					},
				},
			},
			Global: GlobalConfig{
				Database: DatabaseConfig{
					Path: "./test.db",
				},
				Workers: 3,
				Timeout: "30s",
			},
		}

		manager := NewConfigManager()
		err := manager.ValidateConfig(ctx, config)

		assert.NoError(t, err, "Valid config should pass validation")
	})

	t.Run("Invalid service configuration fails", func(t *testing.T) {
		tests := []struct {
			name    string
			service ServiceConfig
			wantErr string
		}{
			{
				name: "empty name",
				service: ServiceConfig{
					Type:   "input",
					Plugin: "tumblr",
				},
				wantErr: "name cannot be empty",
			},
			{
				name: "invalid type",
				service: ServiceConfig{
					Name:   "test",
					Type:   "invalid",
					Plugin: "tumblr",
				},
				wantErr: "type must be 'input' or 'output'",
			},
			{
				name: "empty plugin",
				service: ServiceConfig{
					Name: "test",
					Type: "input",
				},
				wantErr: "plugin cannot be empty",
			},
		}

		manager := NewConfigManager()

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &Config{
					Services: map[string]ServiceConfig{
						"test": tt.service,
					},
					Global: GlobalConfig{
						Database: DatabaseConfig{
							Path: "./test.db",
						},
						Workers: 3,
						Timeout: "30s",
					},
				}

				err := manager.ValidateConfig(ctx, config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})
}

func TestConfigManager_HotReload(t *testing.T) {
	ctx := context.Background()

	t.Run("Watch for configuration changes", func(t *testing.T) {
		configContent := `
services:
  test:
    name: "test-service"
    type: "input"
    plugin: "tumblr"
    enabled: true
global:
  database:
    path: "./test.db"
  workers: 3
  timeout: "30s"
`
		configFile := createTempConfigFile(t, configContent)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()

		manager := NewConfigManager()
		_, err := manager.LoadFromFile(ctx, configFile)
		require.NoError(t, err)

		// Start watching for changes
		changeChan := make(chan ConfigChangeEvent, 1)
		err = manager.WatchForChanges(ctx, configFile, changeChan)
		require.NoError(t, err)

		// Modify the config file
		updatedContent := `
services:
  test:
    name: "test-service-updated"
    type: "input"
    plugin: "tumblr"
    enabled: false
global:
  database:
    path: "./test.db"
  workers: 3
  timeout: "30s"
`
		err = os.WriteFile(configFile, []byte(updatedContent), 0644)
		require.NoError(t, err)

		// Wait for change notification
		select {
		case event := <-changeChan:
			assert.Equal(t, "config_updated", event.Type)
			assert.Equal(t, configFile, event.Path)
		case <-time.After(2 * time.Second):
			t.Error("Should receive config change notification within 2 seconds")
		}

		// Verify config was reloaded
		newConfig := manager.GetCurrentConfig()
		require.NotNil(t, newConfig)
		assert.Equal(t, "test-service-updated", newConfig.Services["test"].Name)
		assert.False(t, newConfig.Services["test"].Enabled)
	})

	t.Run("Rollback on invalid configuration", func(t *testing.T) {
		// Start with valid config
		validContent := `
services:
  test:
    name: "test-service"
    type: "input"
    plugin: "tumblr"
    enabled: true
global:
  database:
    path: "./test.db"
  workers: 3
  timeout: "30s"
`
		configFile := createTempConfigFile(t, validContent)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()

		manager := NewConfigManager()
		originalConfig, err := manager.LoadFromFile(ctx, configFile)
		require.NoError(t, err)

		changeChan := make(chan ConfigChangeEvent, 1)
		err = manager.WatchForChanges(ctx, configFile, changeChan)
		require.NoError(t, err)

		// Write invalid config
		invalidContent := `
services:
  test:
    name: ""  # Invalid: empty name
    type: "input"
    plugin: "tumblr"
global:
  database:
    path: "./test.db"
  workers: 3
`
		err = os.WriteFile(configFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		// Wait for change notification
		select {
		case event := <-changeChan:
			assert.Equal(t, "config_error", event.Type)
			assert.Contains(t, event.Error, "validation failed")
		case <-time.After(2 * time.Second):
			t.Error("Should receive config error notification")
		}

		// Verify original config is preserved
		currentConfig := manager.GetCurrentConfig()
		assert.Equal(t, originalConfig.Services["test"].Name, currentConfig.Services["test"].Name)
		assert.True(t, currentConfig.Services["test"].Enabled)
	})
}

// Helper functions
func createTempConfigFile(t *testing.T, content string) string {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(content), 0644)
	require.NoError(t, err)

	return configFile
}
