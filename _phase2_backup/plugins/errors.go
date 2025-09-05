package plugins

import (
	"fmt"
)

// Error types for plugin operations
var (
	ErrPluginNotFound      = fmt.Errorf("plugin not found")
	ErrPluginAlreadyExists = fmt.Errorf("plugin already exists")
	ErrFactoryNotFound     = fmt.Errorf("factory not found")
	ErrFactoryExists       = fmt.Errorf("factory already exists")
	ErrInvalidConfig       = fmt.Errorf("invalid configuration")
	ErrPluginDisabled      = fmt.Errorf("plugin is disabled")
)

// PluginError represents a plugin-specific error
type PluginError struct {
	PluginName string
	Operation  string
	Cause      error
}

func (e *PluginError) Error() string {
	if e.PluginName != "" {
		return fmt.Sprintf("plugin %s: %s failed: %v", e.PluginName, e.Operation, e.Cause)
	}
	return fmt.Sprintf("%s failed: %v", e.Operation, e.Cause)
}

func (e *PluginError) Unwrap() error {
	return e.Cause
}

// NewPluginError creates a new plugin error
func NewPluginError(pluginName, operation string, cause error) *PluginError {
	return &PluginError{
		PluginName: pluginName,
		Operation:  operation,
		Cause:      cause,
	}
}

// IsPluginError checks if an error is a plugin error
func IsPluginError(err error) bool {
	_, ok := err.(*PluginError)
	return ok
}