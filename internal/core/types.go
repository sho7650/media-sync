package core

import (
	"fmt"
	"time"
)

// MediaItem represents a media item for validation in the application
// This is a simplified version used for core validation, different from storage.MediaItem
type MediaItem struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Validate checks if MediaItem has required fields
func (m *MediaItem) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("media item ID cannot be empty")
	}
	if m.URL == "" {
		return fmt.Errorf("media item URL cannot be empty")
	}
	if m.ContentType == "" {
		return fmt.Errorf("media item ContentType cannot be empty")
	}
	return nil
}

// ServiceConfig represents configuration for a service
type ServiceConfig struct {
	Name     string                 `yaml:"name" json:"name"`
	Type     string                 `yaml:"type" json:"type"`
	Plugin   string                 `yaml:"plugin" json:"plugin"`
	Enabled  bool                   `yaml:"enabled" json:"enabled"`
	Settings map[string]interface{} `yaml:"settings" json:"settings"`
}

// Validate checks if ServiceConfig is valid
func (s *ServiceConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if s.Type != "input" && s.Type != "output" && s.Type != "transform" {
		return fmt.Errorf("service type must be 'input', 'output', or 'transform'")
	}
	if s.Plugin == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	return nil
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Validate checks if PluginMetadata is valid
func (p *PluginMetadata) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if p.Version == "" {
		return fmt.Errorf("plugin version cannot be empty")
	}
	if p.Type != "input" && p.Type != "output" && p.Type != "transform" {
		return fmt.Errorf("plugin type must be 'input', 'output', or 'transform'")
	}
	return nil
}