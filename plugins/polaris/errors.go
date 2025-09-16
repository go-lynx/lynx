package polaris

import (
	"fmt"
	"strings"
)

// ErrorCode error code type
type ErrorCode string

// Error code constants
const (
	// ErrCodeConfigInvalid Configuration related errors
	ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigMissing    ErrorCode = "CONFIG_MISSING"
	ErrCodeConfigValidation ErrorCode = "CONFIG_VALIDATION"

	// ErrCodeInitFailed Initialization related errors
	ErrCodeInitFailed         ErrorCode = "INIT_FAILED"
	ErrCodeAlreadyInitialized ErrorCode = "ALREADY_INITIALIZED"
	ErrCodeNotInitialized     ErrorCode = "NOT_INITIALIZED"

	// ErrCodeSDKContextFailed SDK related errors
	ErrCodeSDKContextFailed ErrorCode = "SDK_CONTEXT_FAILED"
	ErrCodeAPIInitFailed    ErrorCode = "API_INIT_FAILED"
	ErrCodeSDKDestroyed     ErrorCode = "SDK_DESTROYED"

	// ErrCodeServiceNotFound Service related errors
	ErrCodeServiceNotFound       ErrorCode = "SERVICE_NOT_FOUND"
	ErrCodeServiceUnavailable    ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeServiceRegistration   ErrorCode = "SERVICE_REGISTRATION"
	ErrCodeServiceDeregistration ErrorCode = "SERVICE_DEREGISTRATION"

	// ErrCodeConfigNotFound Configuration management related errors
	ErrCodeConfigNotFound    ErrorCode = "CONFIG_NOT_FOUND"
	ErrCodeConfigGetFailed   ErrorCode = "CONFIG_GET_FAILED"
	ErrCodeConfigWatchFailed ErrorCode = "CONFIG_WATCH_FAILED"

	// ErrCodeRateLimitExceeded Rate limiting related errors
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeRateLimitFailed   ErrorCode = "RATE_LIMIT_FAILED"

	// ErrCodeHealthCheckFailed Health check related errors
	ErrCodeHealthCheckFailed  ErrorCode = "HEALTH_CHECK_FAILED"
	ErrCodeHealthCheckTimeout ErrorCode = "HEALTH_CHECK_TIMEOUT"

	// ErrCodeNetworkError Network related errors
	ErrCodeNetworkError     ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout          ErrorCode = "TIMEOUT"
	ErrCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"

	// ErrCodeRetryExhausted Retry related errors
	ErrCodeRetryExhausted     ErrorCode = "RETRY_EXHAUSTED"
	ErrCodeCircuitBreakerOpen ErrorCode = "CIRCUIT_BREAKER_OPEN"

	// ErrCodeWatcherFailed Watcher related errors
	ErrCodeWatcherFailed  ErrorCode = "WATCHER_FAILED"
	ErrCodeWatcherTimeout ErrorCode = "WATCHER_TIMEOUT"

	// ErrCodeMetricsFailed Metrics related errors
	ErrCodeMetricsFailed ErrorCode = "METRICS_FAILED"

	// ErrCodeShutdownFailed Graceful shutdown related errors
	ErrCodeShutdownFailed  ErrorCode = "SHUTDOWN_FAILED"
	ErrCodeShutdownTimeout ErrorCode = "SHUTDOWN_TIMEOUT"
)

// PolarisError Polaris plugin error
type PolarisError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]interface{}
}

// NewPolarisError creates new Polaris error
func NewPolarisError(code ErrorCode, message string) *PolarisError {
	return &PolarisError{
		Code:    code,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WithCause sets error cause
func (e *PolarisError) WithCause(cause error) *PolarisError {
	e.Cause = cause
	return e
}

// WithContext adds context information
func (e *PolarisError) WithContext(key string, value interface{}) *PolarisError {
	e.Context[key] = value
	return e
}

// Error implements error interface
func (e *PolarisError) Error() string {
	var parts []string

	// Add error code
	parts = append(parts, fmt.Sprintf("[%s]", e.Code))

	// Add error message
	parts = append(parts, e.Message)

	// Add cause
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Cause))
	}

	// Add context
	if len(e.Context) > 0 {
		var contextParts []string
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("context: {%s}", strings.Join(contextParts, ", ")))
	}

	return strings.Join(parts, " ")
}

// Unwrap returns error cause
func (e *PolarisError) Unwrap() error {
	return e.Cause
}

// Is checks error type
func (e *PolarisError) Is(target error) bool {
	if targetError, ok := target.(*PolarisError); ok {
		return e.Code == targetError.Code
	}
	return false
}

// Convenient error creation functions

// NewConfigError creates configuration error
func NewConfigError(message string) *PolarisError {
	return NewPolarisError(ErrCodeConfigInvalid, message)
}

// NewInitError creates initialization error
func NewInitError(message string) *PolarisError {
	return NewPolarisError(ErrCodeInitFailed, message)
}

// NewServiceError creates service error
func NewServiceError(code ErrorCode, message string) *PolarisError {
	return NewPolarisError(code, message)
}

// NewNetworkError creates network error
func NewNetworkError(message string) *PolarisError {
	return NewPolarisError(ErrCodeNetworkError, message)
}

// NewTimeoutError creates timeout error
func NewTimeoutError(operation string) *PolarisError {
	return NewPolarisError(ErrCodeTimeout, fmt.Sprintf("operation '%s' timed out", operation))
}

// NewRetryError creates retry error
func NewRetryError(message string) *PolarisError {
	return NewPolarisError(ErrCodeRetryExhausted, message)
}

// NewHealthCheckError creates health check error
func NewHealthCheckError(message string) *PolarisError {
	return NewPolarisError(ErrCodeHealthCheckFailed, message)
}

// Error checking functions

// IsConfigError checks if it's a configuration error
func IsConfigError(err error) bool {
	return isErrorCode(err, ErrCodeConfigInvalid, ErrCodeConfigMissing, ErrCodeConfigValidation)
}

// IsInitError checks if it's an initialization error
func IsInitError(err error) bool {
	return isErrorCode(err, ErrCodeInitFailed, ErrCodeAlreadyInitialized, ErrCodeNotInitialized)
}

// IsServiceError checks if it's a service error
func IsServiceError(err error) bool {
	return isErrorCode(err, ErrCodeServiceNotFound, ErrCodeServiceUnavailable, ErrCodeServiceRegistration, ErrCodeServiceDeregistration)
}

// IsNetworkError checks if it's a network error
func IsNetworkError(err error) bool {
	return isErrorCode(err, ErrCodeNetworkError, ErrCodeTimeout, ErrCodeConnectionFailed)
}

// IsRetryError checks if it's a retry error
func IsRetryError(err error) bool {
	return isErrorCode(err, ErrCodeRetryExhausted, ErrCodeCircuitBreakerOpen)
}

// isErrorCode checks error code
func isErrorCode(err error, codes ...ErrorCode) bool {
	if polarisErr, ok := err.(*PolarisError); ok {
		for _, code := range codes {
			if polarisErr.Code == code {
				return true
			}
		}
	}
	return false
}

// Error wrapping functions

// WrapError wraps error
func WrapError(err error, code ErrorCode, message string) *PolarisError {
	return NewPolarisError(code, message).WithCause(err)
}

// WrapConfigError wraps configuration error
func WrapConfigError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeConfigInvalid, message)
}

// WrapInitError wraps initialization error
func WrapInitError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeInitFailed, message)
}

// WrapServiceError wraps service error
func WrapServiceError(err error, code ErrorCode, message string) *PolarisError {
	return WrapError(err, code, message)
}

// WrapNetworkError wraps network error
func WrapNetworkError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeNetworkError, message)
}
