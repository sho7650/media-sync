package plugins

import (
	"fmt"
	"strings"
)

// PluginConfig defines the configuration for a plugin
type PluginConfig struct {
	Name        string                 `yaml:"name" json:"name"`
	Type        string                 `yaml:"type" json:"type"`
	Version     string                 `yaml:"version" json:"version"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Settings    map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// Validate checks if the PluginConfig is valid
func (c *PluginConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("plugin name is required")
	}
	
	if strings.TrimSpace(c.Type) == "" {
		return fmt.Errorf("plugin type is required")
	}
	
	// Validate plugin type
	validTypes := map[string]bool{
		"input":     true,
		"output":    true,
		"transform": true,
	}
	
	if !validTypes[c.Type] {
		return fmt.Errorf("invalid plugin type: %s (must be input, output, or transform)", c.Type)
	}
	
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("plugin version is required")
	}
	
	return nil
}

// Clone creates a deep copy of the PluginConfig
func (c *PluginConfig) Clone() PluginConfig {
	clone := PluginConfig{
		Name:        c.Name,
		Type:        c.Type,
		Version:     c.Version,
		Description: c.Description,
		Enabled:     c.Enabled,
	}
	
	// Deep copy settings
	if c.Settings != nil {
		clone.Settings = make(map[string]interface{})
		for k, v := range c.Settings {
			clone.Settings[k] = v
		}
	}
	
	return clone
}