package plugins

import (
	"fmt"
	"strings"

	"github.com/sho/media-sync/pkg/core/interfaces"
)

// Plugin interface extending core.Service
type Plugin interface {
	interfaces.Service
	GetMetadata() PluginMetadata
	Configure(config map[string]interface{}) error
}

// PluginMetadata contains plugin information and validation
type PluginMetadata struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
}

// Validate checks if PluginMetadata is valid
func (m *PluginMetadata) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	validTypes := map[string]bool{
		"input":     true,
		"output":    true,
		"transform": true,
	}
	if !validTypes[m.Type] {
		return fmt.Errorf("plugin type must be input, output, or transform")
	}

	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("plugin version cannot be empty")
	}

	return nil
}