package plugins

import (
	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

// HealthEvent represents a health monitoring event for a plugin
type HealthEvent struct {
	PluginName            string
	Health                interfaces.ServiceHealth
	AutoRecoveryAttempted bool
	RecoverySuccess       bool
}

// ResourceUsage tracks resource consumption for a plugin
type ResourceUsage struct {
	MemoryBytes int64
	Connections int
	FileHandles int
}