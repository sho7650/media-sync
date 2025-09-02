package config

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager handles configuration loading, validation, and hot reload
type ConfigManager struct {
	currentConfig *Config
	mutex         sync.RWMutex
	watchers      map[string]chan ConfigChangeEvent
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		watchers: make(map[string]chan ConfigChangeEvent),
	}
}

// LoadFromFile loads configuration from a YAML file
func (cm *ConfigManager) LoadFromFile(ctx context.Context, filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	content := cm.substituteEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate configuration
	if err := cm.ValidateConfig(ctx, &config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Store as current config
	cm.mutex.Lock()
	cm.currentConfig = &config
	cm.mutex.Unlock()

	return &config, nil
}

// ValidateConfig validates the entire configuration
func (cm *ConfigManager) ValidateConfig(ctx context.Context, config *Config) error {
	// Validate global configuration
	if err := config.Global.Validate(); err != nil {
		return fmt.Errorf("global config validation failed: %w", err)
	}

	// Validate each service
	for name, service := range config.Services {
		if err := service.Validate(); err != nil {
			return fmt.Errorf("service '%s' validation failed: %w", name, err)
		}
	}

	return nil
}

// GetCurrentConfig returns the currently loaded configuration
func (cm *ConfigManager) GetCurrentConfig() *Config {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.currentConfig
}

// WatchForChanges starts watching for configuration file changes
func (cm *ConfigManager) WatchForChanges(ctx context.Context, filePath string, changeChan chan ConfigChangeEvent) error {
	// Store the channel for notifications
	cm.mutex.Lock()
	cm.watchers[filePath] = changeChan
	cm.mutex.Unlock()

	// Start file watcher in goroutine
	go cm.watchFile(ctx, filePath)

	return nil
}

// watchFile monitors a configuration file for changes (simplified implementation)
func (cm *ConfigManager) watchFile(ctx context.Context, filePath string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if stat, err := os.Stat(filePath); err == nil {
		lastModTime = stat.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stat, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			if stat.ModTime().After(lastModTime) {
				lastModTime = stat.ModTime()
				cm.handleConfigChange(ctx, filePath)
			}
		}
	}
}

// handleConfigChange processes configuration file changes
func (cm *ConfigManager) handleConfigChange(ctx context.Context, filePath string) {
	cm.mutex.RLock()
	changeChan, exists := cm.watchers[filePath]
	cm.mutex.RUnlock()

	if !exists {
		return
	}

	// Try to load new configuration
	newConfig, err := cm.LoadFromFile(ctx, filePath)
	if err != nil {
		// Send error event
		select {
		case changeChan <- ConfigChangeEvent{
			Type:  "config_error",
			Path:  filePath,
			Error: err.Error(),
		}:
		default:
		}
		return
	}

	// Send success event
	select {
	case changeChan <- ConfigChangeEvent{
		Type: "config_updated",
		Path: filePath,
	}:
	default:
	}

	_ = newConfig // Configuration is already stored in LoadFromFile
}

// substituteEnvVars replaces ${VAR} patterns with environment variables
func (cm *ConfigManager) substituteEnvVars(content string) string {
	envVarPattern := regexp.MustCompile(`\$\{([^}]+)\}`)

	return envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name
		varName := match[2 : len(match)-1] // Remove ${ and }

		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}

		// Return original if not found
		return match
	})
}
