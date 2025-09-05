package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginDiscovery finds and validates plugins
type PluginDiscovery struct {
	// Could add configuration options here
}

// NewPluginDiscovery creates a new plugin discovery instance
func NewPluginDiscovery() *PluginDiscovery {
	return &PluginDiscovery{}
}

// DiscoverPlugins scans a directory for plugin configuration files
func (d *PluginDiscovery) DiscoverPlugins(dir string) ([]PluginConfig, error) {
	var configs []PluginConfig
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		
		path := filepath.Join(dir, name)
		config, err := d.LoadPluginConfig(path)
		if err != nil {
			// Skip invalid configs but log warning
			fmt.Printf("Warning: failed to load config from %s: %v\n", path, err)
			continue
		}
		
		// Validate the config
		if err := d.ValidatePluginConfig(config); err != nil {
			fmt.Printf("Warning: invalid config in %s: %v\n", path, err)
			continue
		}
		
		configs = append(configs, config)
	}
	
	return configs, nil
}

// DiscoverPluginsRecursive recursively scans directories for plugin configs
func (d *PluginDiscovery) DiscoverPluginsRecursive(root string) ([]PluginConfig, error) {
	var configs []PluginConfig
	
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		
		config, err := d.LoadPluginConfig(path)
		if err != nil {
			// Skip invalid configs but log warning
			fmt.Printf("Warning: failed to load config from %s: %v\n", path, err)
			return nil
		}
		
		// Validate the config
		if err := d.ValidatePluginConfig(config); err != nil {
			fmt.Printf("Warning: invalid config in %s: %v\n", path, err)
			return nil
		}
		
		configs = append(configs, config)
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}
	
	return configs, nil
}

// LoadPluginConfig loads a plugin configuration from a file
func (d *PluginDiscovery) LoadPluginConfig(path string) (PluginConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginConfig{}, fmt.Errorf("failed to read file: %w", err)
	}
	
	var config PluginConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return PluginConfig{}, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	return config, nil
}

// ValidatePluginConfig validates a plugin configuration
func (d *PluginDiscovery) ValidatePluginConfig(config PluginConfig) error {
	if strings.TrimSpace(config.Name) == "" {
		return fmt.Errorf("plugin name is required")
	}
	
	if strings.TrimSpace(config.Type) == "" {
		return fmt.Errorf("plugin type is required")
	}
	
	validTypes := map[string]bool{
		"input":     true,
		"output":    true,
		"transform": true,
	}
	
	if !validTypes[config.Type] {
		return fmt.Errorf("invalid plugin type: %s (must be input, output, or transform)", config.Type)
	}
	
	if strings.TrimSpace(config.Version) == "" {
		return fmt.Errorf("plugin version is required")
	}
	
	return nil
}

// FindPluginConfigs finds all plugin configuration files matching a pattern
func (d *PluginDiscovery) FindPluginConfigs(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	
	var configs []string
	for _, match := range matches {
		if strings.HasSuffix(match, ".yaml") || strings.HasSuffix(match, ".yml") {
			configs = append(configs, match)
		}
	}
	
	return configs, nil
}

