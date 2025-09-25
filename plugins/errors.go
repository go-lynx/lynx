package plugins

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents a specific error type for better categorization
type ErrorCode string

const (
	// Plugin lifecycle errors
	ErrorCodePluginNotFound        ErrorCode = "PLUGIN_NOT_FOUND"
	ErrorCodePluginAlreadyExists   ErrorCode = "PLUGIN_ALREADY_EXISTS"
	ErrorCodePluginNotInitialized  ErrorCode = "PLUGIN_NOT_INITIALIZED"
	ErrorCodePluginNotActive       ErrorCode = "PLUGIN_NOT_ACTIVE"
	ErrorCodePluginAlreadyActive   ErrorCode = "PLUGIN_ALREADY_ACTIVE"
	
	// Configuration errors
	ErrorCodeInvalidPluginID       ErrorCode = "INVALID_PLUGIN_ID"
	ErrorCodeInvalidPluginVersion  ErrorCode = "INVALID_PLUGIN_VERSION"
	ErrorCodeInvalidPluginConfig   ErrorCode = "INVALID_PLUGIN_CONFIG"
	ErrorCodeInvalidConfiguration  ErrorCode = "INVALID_CONFIGURATION"
	
	// Dependency errors
	ErrorCodePluginDependencyNotMet ErrorCode = "PLUGIN_DEPENDENCY_NOT_MET"
	
	// Upgrade errors
	ErrorCodePluginUpgradeNotSupported ErrorCode = "PLUGIN_UPGRADE_NOT_SUPPORTED"
	ErrorCodePluginUpgradeFailed       ErrorCode = "PLUGIN_UPGRADE_FAILED"
	
	// Resource errors
	ErrorCodePluginResourceNotFound ErrorCode = "PLUGIN_RESOURCE_NOT_FOUND"
	ErrorCodePluginResourceInvalid  ErrorCode = "PLUGIN_RESOURCE_INVALID"
	
	// Operation errors
	ErrorCodePluginOperationTimeout   ErrorCode = "PLUGIN_OPERATION_TIMEOUT"
	ErrorCodePluginOperationCancelled ErrorCode = "PLUGIN_OPERATION_CANCELLED"
	
	// Health and security errors
	ErrorCodePluginHealthCheckFailed ErrorCode = "PLUGIN_HEALTH_CHECK_FAILED"
	ErrorCodePluginSecurityViolation ErrorCode = "PLUGIN_SECURITY_VIOLATION"
)

// Common error variables for plugin-related operations
var (
	// ErrPluginNotFound indicates that a requested plugin could not be found in the system
	// This error occurs when attempting to access or operate on a non-existent plugin
	ErrPluginNotFound = NewStandardError(ErrorCodePluginNotFound, "plugin not found", "The requested plugin does not exist in the system registry")

	// ErrPluginAlreadyExists indicates an attempt to register a plugin with an ID that is already in use
	// This error helps maintain unique plugin identifiers across the system
	ErrPluginAlreadyExists = NewStandardError(ErrorCodePluginAlreadyExists, "plugin already exists", "A plugin with this ID is already registered in the system")

	// ErrPluginNotInitialized indicates an attempt to use a plugin that hasn't been properly initialized
	// Operations on uninitialized plugins are not allowed to prevent undefined behavior
	ErrPluginNotInitialized = NewStandardError(ErrorCodePluginNotInitialized, "plugin not initialized", "The plugin must be initialized before performing this operation")

	// ErrPluginNotActive indicates an attempt to use a plugin that is not in the active state
	// The plugin must be in StatusActive to perform the requested operation
	ErrPluginNotActive = NewStandardError(ErrorCodePluginNotActive, "plugin not active", "The plugin must be in active state to perform this operation")

	// ErrPluginAlreadyActive indicates an attempt to start an already active plugin
	// Prevents duplicate activation of plugins
	ErrPluginAlreadyActive = NewStandardError(ErrorCodePluginAlreadyActive, "plugin already active", "The plugin is already in active state")

	// ErrInvalidPluginID indicates that the provided plugin ID is invalid
	// Plugin IDs must follow specific formatting rules and be non-empty
	ErrInvalidPluginID = NewStandardError(ErrorCodeInvalidPluginID, "invalid plugin ID", "Plugin ID must be non-empty and follow naming conventions")

	// ErrInvalidPluginVersion indicates that the provided plugin version is invalid
	// Version strings must follow semantic versioning format
	ErrInvalidPluginVersion = NewStandardError(ErrorCodeInvalidPluginVersion, "invalid plugin version", "Plugin version must follow semantic versioning format (e.g., 1.0.0)")

	// ErrInvalidPluginConfig indicates that the provided plugin configuration is invalid
	// Configuration must meet the plugin's specific requirements
	ErrInvalidPluginConfig = NewStandardError(ErrorCodeInvalidPluginConfig, "invalid plugin configuration", "The provided configuration does not meet plugin requirements")

	// ErrInvalidConfiguration indicates that the provided configuration is not of the expected type
	// This error occurs when attempting to configure a plugin with an incompatible configuration type
	ErrInvalidConfiguration = NewStandardError(ErrorCodeInvalidConfiguration, "invalid configuration type", "Configuration type does not match expected plugin configuration interface")

	// ErrPluginDependencyNotMet indicates that one or more plugin dependencies are not satisfied
	// All required dependencies must be available and properly configured
	ErrPluginDependencyNotMet = NewStandardError(ErrorCodePluginDependencyNotMet, "plugin dependency not met", "One or more required plugin dependencies are missing or not properly configured")

	// ErrPluginUpgradeNotSupported indicates that the plugin does not support the requested upgrade operation
	// The plugin must implement the Upgradable interface and support the specific upgrade capability
	ErrPluginUpgradeNotSupported = NewStandardError(ErrorCodePluginUpgradeNotSupported, "plugin upgrade not supported", "The plugin does not implement upgrade capabilities")

	// ErrPluginUpgradeFailed indicates that the plugin upgrade process failed
	// Contains details about the specific failure in upgrade process
	ErrPluginUpgradeFailed = NewStandardError(ErrorCodePluginUpgradeFailed, "plugin upgrade failed", "The plugin upgrade process encountered an error")

	// ErrPluginResourceNotFound indicates that a requested plugin resource is not available
	// The resource must be registered before it can be accessed
	ErrPluginResourceNotFound = NewStandardError(ErrorCodePluginResourceNotFound, "plugin resource not found", "The requested plugin resource is not available or not registered")

	// ErrPluginResourceInvalid indicates that a plugin resource is in an invalid state
	// The resource must be properly initialized and maintained
	ErrPluginResourceInvalid = NewStandardError(ErrorCodePluginResourceInvalid, "plugin resource invalid", "The plugin resource is in an invalid or corrupted state")

	// ErrPluginOperationTimeout indicates that a plugin operation exceeded its time limit
	// Operations must complete within their specified timeout period
	ErrPluginOperationTimeout = NewStandardError(ErrorCodePluginOperationTimeout, "plugin operation timeout", "The plugin operation exceeded the specified timeout period")

	// ErrPluginOperationCancelled indicates that a plugin operation was cancelled
	// The operation was terminated before completion, either by user request or system action
	ErrPluginOperationCancelled = NewStandardError(ErrorCodePluginOperationCancelled, "plugin operation cancelled", "The plugin operation was cancelled before completion")

	// ErrPluginHealthCheckFailed indicates that the plugin's health check failed
	// The plugin is in an unhealthy state and may need attention
	ErrPluginHealthCheckFailed = NewStandardError(ErrorCodePluginHealthCheckFailed, "plugin health check failed", "The plugin health check indicates an unhealthy state")

	// ErrPluginSecurityViolation indicates a security-related violation in the plugin
	// Security policies or constraints have been breached
	ErrPluginSecurityViolation = NewStandardError(ErrorCodePluginSecurityViolation, "plugin security violation", "A security policy or constraint has been violated")
)

// StandardError represents a standard error with enhanced information
type StandardError struct {
	Code        ErrorCode `json:"code"`
	Message     string    `json:"message"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// Error implements the error interface for StandardError
func (e *StandardError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Description)
}

// NewStandardError creates a new StandardError with the given details
func NewStandardError(code ErrorCode, message, description string) *StandardError {
	return &StandardError{
		Code:        code,
		Message:     message,
		Description: description,
		Timestamp:   time.Now(),
	}
}

// PluginError represents a detailed error that occurred during plugin operations
type PluginError struct {
	// PluginID identifies the plugin where the error occurred
	PluginID string `json:"plugin_id"`

	// Operation describes the action that was being performed when the error occurred
	Operation string `json:"operation"`

	// Message provides a detailed description of the error
	Message string `json:"message"`

	// Err is the underlying error that caused this PluginError
	Err error `json:"-"`

	// Code represents the error type for better categorization
	Code ErrorCode `json:"code,omitempty"`

	// Context provides additional context information
	Context map[string]interface{} `json:"context,omitempty"`

	// Timestamp when the error occurred
	Timestamp time.Time `json:"timestamp"`

	// StackTrace provides debugging information
	StackTrace string `json:"stack_trace,omitempty"`
}

// Error implements the error interface for PluginError
// Returns a formatted error message including plugin ID, operation, and details
func (e *PluginError) Error() string {
	var parts []string
	
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.Code))
	}
	
	if e.PluginID != "" {
		parts = append(parts, fmt.Sprintf("plugin '%s'", e.PluginID))
	}
	
	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation '%s'", e.Operation))
	}
	
	parts = append(parts, "failed")
	
	if e.Message != "" {
		parts = append(parts, fmt.Sprintf(": %s", e.Message))
	}
	
	if e.Err != nil {
		parts = append(parts, fmt.Sprintf(" (caused by: %v)", e.Err))
	}
	
	return strings.Join(parts, " ")
}

// Unwrap implements the errors unwrap interface
// Returns the underlying error for error chain handling
func (e *PluginError) Unwrap() error {
	return e.Err
}

// WithContext adds context information to the error
func (e *PluginError) WithContext(key string, value interface{}) *PluginError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithStackTrace adds stack trace information to the error
func (e *PluginError) WithStackTrace() *PluginError {
	e.StackTrace = getStackTrace()
	return e
}

// NewPluginError creates a new PluginError with the given details
// Provides a convenient way to create structured plugin errors
func NewPluginError(pluginID, operation, message string, err error) *PluginError {
	return &PluginError{
		PluginID:  pluginID,
		Operation: operation,
		Message:   message,
		Err:       err,
		Timestamp: time.Now(),
	}
}

// NewPluginErrorWithCode creates a new PluginError with error code
func NewPluginErrorWithCode(code ErrorCode, pluginID, operation, message string, err error) *PluginError {
	return &PluginError{
		PluginID:  pluginID,
		Operation: operation,
		Message:   message,
		Err:       err,
		Code:      code,
		Timestamp: time.Now(),
	}
}

// getStackTrace captures the current stack trace for debugging
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	
	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		
		// Skip runtime and internal frames
		if strings.Contains(frame.File, "runtime/") {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))
	}
	
	return sb.String()
}

// IsPluginError checks if an error is a PluginError
func IsPluginError(err error) bool {
	_, ok := err.(*PluginError)
	return ok
}

// GetPluginError extracts PluginError from error chain
func GetPluginError(err error) *PluginError {
	var pluginErr *PluginError
	if errors.As(err, &pluginErr) {
		return pluginErr
	}
	return nil
}

// FormatErrorForUser formats an error message for end-user display
func FormatErrorForUser(err error) string {
	if pluginErr := GetPluginError(err); pluginErr != nil {
		if pluginErr.PluginID != "" && pluginErr.Operation != "" {
			return fmt.Sprintf("Plugin '%s' failed during '%s': %s", 
				pluginErr.PluginID, pluginErr.Operation, pluginErr.Message)
		}
		return pluginErr.Message
	}
	
	return err.Error()
}

// FormatErrorForDeveloper formats an error message for developer debugging
func FormatErrorForDeveloper(err error) string {
	if pluginErr := GetPluginError(err); pluginErr != nil {
		var sb strings.Builder
		sb.WriteString(pluginErr.Error())
		
		if len(pluginErr.Context) > 0 {
			sb.WriteString("\nContext:")
			for k, v := range pluginErr.Context {
				sb.WriteString(fmt.Sprintf("\n  %s: %v", k, v))
			}
		}
		
		if pluginErr.StackTrace != "" {
			sb.WriteString("\nStack Trace:\n")
			sb.WriteString(pluginErr.StackTrace)
		}
		
		return sb.String()
	}
	
	return err.Error()
}
