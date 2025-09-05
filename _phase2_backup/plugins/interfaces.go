package plugins

import (
	"github.com/sho7650/media-sync/internal/core"
)

// Plugin interface for Phase 1 - simplified to match roadmap
type Plugin interface {
	// Core plugin methods
	GetMetadata() core.PluginMetadata
	Configure(config map[string]interface{}) error
	
	// Plugin lifecycle (simplified Phase 1 version)
	Initialize() error
	Cleanup() error
}

// InputPlugin interface for input plugins
type InputPlugin interface {
	Plugin
	core.InputService
}

// OutputPlugin interface for output plugins  
type OutputPlugin interface {
	Plugin
	core.OutputService
}