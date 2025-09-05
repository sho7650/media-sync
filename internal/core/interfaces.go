package core

import (
    "context"
    "fmt"
    "time"
)

// InputService defines the contract for input plugins
type InputService interface {
    FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error)
    GetMetadata() PluginMetadata
}

// OutputService defines the contract for output plugins
type OutputService interface {
    SendMedia(ctx context.Context, media []MediaItem, config map[string]interface{}) error
    GetMetadata() PluginMetadata
}

// MediaItem represents a media item to be synchronized
type MediaItem struct {
    ID          string                 `json:"id"`
    URL         string                 `json:"url"`
    ContentType string                 `json:"content_type"`
    Metadata    map[string]interface{} `json:"metadata"`
    CreatedAt   time.Time              `json:"created_at"`
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Type        string `json:"type"` // "input" or "output"
    Description string `json:"description"`
}

// Validate checks if PluginMetadata is valid
func (p *PluginMetadata) Validate() error {
    if p.Name == "" {
        return fmt.Errorf("plugin name cannot be empty")
    }
    if p.Type != "input" && p.Type != "output" {
        return fmt.Errorf("plugin type must be 'input' or 'output'")
    }
    if p.Version == "" {
        return fmt.Errorf("plugin version cannot be empty")
    }
    return nil
}

// ServiceConfig represents service configuration
type ServiceConfig struct {
    Name     string                 `json:"name"`
    Type     string                 `json:"type"`     // "input" or "output"
    Plugin   string                 `json:"plugin"`
    Enabled  bool                   `json:"enabled"`
    Settings map[string]interface{} `json:"settings"`
}

// Validate checks if MediaItem is valid
func (m *MediaItem) Validate() error {
    if m.ID == "" {
        return fmt.Errorf("media item ID cannot be empty")
    }
    if m.URL == "" {
        return fmt.Errorf("media item URL cannot be empty")
    }
    return nil
}

// Validate checks if ServiceConfig is valid
func (s *ServiceConfig) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("name cannot be empty")
    }
    if s.Type != "input" && s.Type != "output" {
        return fmt.Errorf("type must be 'input' or 'output'")
    }
    if s.Plugin == "" {
        return fmt.Errorf("plugin cannot be empty")
    }
    return nil
}