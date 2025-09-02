package config

import (
	"fmt"
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
	Global   GlobalConfig             `yaml:"global"`
}

// ServiceConfig represents configuration for a single service
type ServiceConfig struct {
	Name     string                 `yaml:"name"`
	Type     string                 `yaml:"type"`
	Plugin   string                 `yaml:"plugin"`
	Enabled  bool                   `yaml:"enabled"`
	Settings map[string]interface{} `yaml:"settings"`
}

// GlobalConfig represents global application settings
type GlobalConfig struct {
	Database DatabaseConfig `yaml:"database"`
	Workers  int            `yaml:"workers"`
	Timeout  string         `yaml:"timeout"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ConfigChangeEvent represents a configuration change event
type ConfigChangeEvent struct {
	Type  string
	Path  string
	Error string
}

// Validate checks if ServiceConfig is valid
func (s *ServiceConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if s.Type != "input" && s.Type != "output" {
		return fmt.Errorf("service type must be 'input' or 'output', got: %s", s.Type)
	}

	if s.Plugin == "" {
		return fmt.Errorf("service plugin cannot be empty")
	}

	return nil
}

// Validate checks if GlobalConfig is valid
func (g *GlobalConfig) Validate() error {
	if g.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if g.Workers <= 0 {
		return fmt.Errorf("workers must be greater than 0, got: %d", g.Workers)
	}

	if g.Timeout != "" {
		if _, err := time.ParseDuration(g.Timeout); err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	return nil
}
