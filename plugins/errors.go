package plugins

import (
	"errors"
	"fmt"
)

// Common error variables for plugin-related operations
var (
	// ErrPluginNotFound indicates that a requested plugin could not be found in the system
	// This error occurs when attempting to access or operate on a non-existent plugin
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrPluginAlreadyExists indicates an attempt to register a plugin with an ID that is already in use
	// This error helps maintain unique plugin identifiers across the system
	ErrPluginAlreadyExists = errors.New("plugin already exists")

	// ErrPluginNotInitialized indicates an attempt to use a plugin that hasn't been properly initialized
	// Operations on uninitialized plugins are not allowed to prevent undefined behavior
	ErrPluginNotInitialized = errors.New("plugin not initialized")

	// ErrPluginNotActive indicates an attempt to use a plugin that is not in the active state
	// The plugin must be in StatusActive to perform the requested operation
	ErrPluginNotActive = errors.New("plugin not active")

	// ErrPluginAlreadyActive indicates an attempt to start an already active plugin
	// Prevents duplicate activation of plugins
	ErrPluginAlreadyActive = errors.New("plugin already active")

	// ErrInvalidPluginID indicates that the provided plugin ID is invalid
	// Plugin IDs must follow specific formatting rules and be non-empty
	ErrInvalidPluginID = errors.New("invalid plugin ID")

	// ErrInvalidPluginVersion indicates that the provided plugin version is invalid
	// Version strings must follow semantic versioning format
	ErrInvalidPluginVersion = errors.New("invalid plugin version")

	// ErrInvalidPluginConfig indicates that the provided plugin configuration is invalid
	// Configuration must meet the plugin's specific requirements
	ErrInvalidPluginConfig = errors.New("invalid plugin configuration")

	// ErrInvalidConfiguration indicates that the provided configuration is not of the expected type
	// This error occurs when attempting to configure a plugin with an incompatible configuration type
	ErrInvalidConfiguration = errors.New("invalid configuration type")

	// ErrPluginDependencyNotMet indicates that one or more plugin dependencies are not satisfied
	// All required dependencies must be available and properly configured
	ErrPluginDependencyNotMet = errors.New("plugin dependency not met")

	// ErrPluginUpgradeNotSupported indicates that the plugin does not support the requested upgrade operation
	// The plugin must implement the Upgradable interface and support the specific upgrade capability
	ErrPluginUpgradeNotSupported = errors.New("plugin upgrade not supported")

	// ErrPluginUpgradeFailed indicates that the plugin upgrade process failed
	// Contains details about the specific failure in upgrade process
	ErrPluginUpgradeFailed = errors.New("plugin upgrade failed")

	// ErrPluginResourceNotFound indicates that a requested plugin resource is not available
	// The resource must be registered before it can be accessed
	ErrPluginResourceNotFound = errors.New("plugin resource not found")

	// ErrPluginResourceInvalid indicates that a plugin resource is in an invalid state
	// The resource must be properly initialized and maintained
	ErrPluginResourceInvalid = errors.New("plugin resource invalid")

	// ErrPluginOperationTimeout indicates that a plugin operation exceeded its time limit
	// Operations must complete within their specified timeout period
	ErrPluginOperationTimeout = errors.New("plugin operation timeout")

	// ErrPluginOperationCancelled indicates that a plugin operation was cancelled
	// The operation was terminated before completion, either by user request or system action
	ErrPluginOperationCancelled = errors.New("plugin operation cancelled")

	// ErrPluginHealthCheckFailed indicates that the plugin's health check failed
	// The plugin is in an unhealthy state and may need attention
	ErrPluginHealthCheckFailed = errors.New("plugin health check failed")

	// ErrPluginSecurityViolation indicates a security-related violation in the plugin
	// Security policies or constraints have been breached
	ErrPluginSecurityViolation = errors.New("plugin security violation")
)

// PluginError represents a detailed error that occurred during plugin operations
type PluginError struct {
	// PluginID identifies the plugin where the error occurred
	PluginID string

	// Operation describes the action that was being performed when the error occurred
	Operation string

	// Message provides a detailed description of the error
	Message string

	// Err is the underlying error that caused this PluginError
	Err error
}

// Error implements the error interface for PluginError
// Returns a formatted error message including plugin ID, operation, and details
func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("plugin %s: %s failed: %s (%v)", e.PluginID, e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s failed: %s", e.PluginID, e.Operation, e.Message)
}

// Unwrap implements the errors unwrap interface
// Returns the underlying error for error chain handling
func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a new PluginError with the given details
// Provides a convenient way to create structured plugin errors
func NewPluginError(pluginID, operation, message string, err error) *PluginError {
	return &PluginError{
		PluginID:  pluginID,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}
