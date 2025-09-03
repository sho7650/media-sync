package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestPluginConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  PluginConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input plugin config",
			config: PluginConfig{
				Name:    "tumblr-input",
				Type:    "input",
				Version: "1.0.0",
				Enabled: true,
				Settings: map[string]interface{}{
					"api_key": "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "valid output plugin config",
			config: PluginConfig{
				Name:        "filesystem-output",
				Type:        "output",
				Version:     "2.1.0",
				Description: "Saves media to filesystem",
				Enabled:     false,
				Settings: map[string]interface{}{
					"path": "/var/media",
				},
			},
			wantErr: false,
		},
		{
			name: "valid transform plugin config",
			config: PluginConfig{
				Name:    "image-processor",
				Type:    "transform",
				Version: "1.0.0-beta",
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			config: PluginConfig{
				Name:    "",
				Type:    "input",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name is required",
		},
		{
			name: "whitespace only name",
			config: PluginConfig{
				Name:    "   ",
				Type:    "input",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin name is required",
		},
		{
			name: "empty type",
			config: PluginConfig{
				Name:    "test",
				Type:    "",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "plugin type is required",
		},
		{
			name: "invalid type",
			config: PluginConfig{
				Name:    "test",
				Type:    "processor",
				Version: "1.0.0",
			},
			wantErr: true,
			errMsg:  "invalid plugin type: processor",
		},
		{
			name: "empty version",
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
			err := tt.config.Validate()
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

func TestPluginConfig_YAMLMarshaling(t *testing.T) {
	config := PluginConfig{
		Name:        "test-plugin",
		Type:        "input",
		Version:     "1.2.3",
		Description: "A test plugin for YAML marshaling",
		Enabled:     true,
		Settings: map[string]interface{}{
			"url":      "https://api.example.com",
			"timeout":  30,
			"retries":  3,
			"features": []string{"auth", "pagination"},
		},
	}
	
	// Marshal to YAML
	data, err := yaml.Marshal(&config)
	assert.NoError(t, err)
	
	// Unmarshal back
	var unmarshaled PluginConfig
	err = yaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	
	// Verify fields
	assert.Equal(t, config.Name, unmarshaled.Name)
	assert.Equal(t, config.Type, unmarshaled.Type)
	assert.Equal(t, config.Version, unmarshaled.Version)
	assert.Equal(t, config.Description, unmarshaled.Description)
	assert.Equal(t, config.Enabled, unmarshaled.Enabled)
	assert.Equal(t, config.Settings["url"], unmarshaled.Settings["url"])
	assert.Equal(t, 30, unmarshaled.Settings["timeout"])
	assert.Equal(t, 3, unmarshaled.Settings["retries"])
}

func TestPluginConfig_Clone(t *testing.T) {
	original := PluginConfig{
		Name:        "original",
		Type:        "input",
		Version:     "1.0.0",
		Description: "Original plugin",
		Enabled:     true,
		Settings: map[string]interface{}{
			"key":   "value",
			"count": 42,
			"nested": map[string]interface{}{
				"deep": "value",
			},
		},
	}
	
	cloned := original.Clone()
	
	// Verify clone is equal
	assert.Equal(t, original.Name, cloned.Name)
	assert.Equal(t, original.Type, cloned.Type)
	assert.Equal(t, original.Version, cloned.Version)
	assert.Equal(t, original.Description, cloned.Description)
	assert.Equal(t, original.Enabled, cloned.Enabled)
	
	// Verify deep copy of settings
	assert.Equal(t, original.Settings["key"], cloned.Settings["key"])
	
	// Modify clone and verify original is unchanged
	cloned.Name = "modified"
	cloned.Settings["key"] = "modified"
	
	assert.Equal(t, "original", original.Name)
	assert.Equal(t, "value", original.Settings["key"])
}