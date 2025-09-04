package redislock

import (
	"errors"
	"fmt"
)

// Error code definitions
const (
	ErrCodeLockNotHeld           = "LOCK_NOT_HELD"
	ErrCodeLockAcquireFailed     = "LOCK_ACQUIRE_FAILED"
	ErrCodeLockAcquireTimeout    = "LOCK_ACQUIRE_TIMEOUT"
	ErrCodeLockAcquireConflict   = "LOCK_ACQUIRE_CONFLICT"
	ErrCodeRedisClientNotFound   = "REDIS_CLIENT_NOT_FOUND"
	ErrCodeMaxRetriesExceeded    = "MAX_RETRIES_EXCEEDED"
	ErrCodeLockFnRequired        = "LOCK_FN_REQUIRED"
	ErrCodeLockRenewalFailed     = "LOCK_RENEWAL_FAILED"
	ErrCodeRenewalServiceStopped = "RENEWAL_SERVICE_STOPPED"
	ErrCodeInvalidOptions        = "INVALID_OPTIONS"
)

// LockError custom lock error type
type LockError struct {
	Code    string
	Message string
	Err     error
}

func (e *LockError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

// Helper function to create lock errors
func newLockError(code, message string, err error) *LockError {
	return &LockError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

var (
	// ErrLockNotHeld indicates attempting to release a lock not held
	ErrLockNotHeld = newLockError(ErrCodeLockNotHeld, "lock not held", nil)
	// ErrLockAcquireFailed indicates lock acquisition failure
	ErrLockAcquireFailed = newLockError(ErrCodeLockAcquireFailed, "failed to acquire lock", nil)
	// ErrLockAcquireTimeout indicates lock acquisition timeout
	ErrLockAcquireTimeout = newLockError(ErrCodeLockAcquireTimeout, "lock acquire timeout", nil)
	// ErrLockAcquireConflict indicates lock acquisition conflict
	ErrLockAcquireConflict = newLockError(ErrCodeLockAcquireConflict, "lock acquire conflict", nil)
	// ErrRedisClientNotFound indicates Redis client not found
	ErrRedisClientNotFound = newLockError(ErrCodeRedisClientNotFound, "redis client not found", nil)
	// ErrMaxRetriesExceeded indicates exceeding maximum retry attempts
	ErrMaxRetriesExceeded = newLockError(ErrCodeMaxRetriesExceeded, "max retries exceeded", nil)
	// ErrLockFnRequired indicates lock protected function cannot be empty
	ErrLockFnRequired = newLockError(ErrCodeLockFnRequired, "lock function is required", nil)
	// ErrLockRenewalFailed indicates lock renewal failure
	ErrLockRenewalFailed = newLockError(ErrCodeLockRenewalFailed, "lock renewal failed", nil)
	// ErrRenewalServiceStopped indicates renewal service has stopped
	ErrRenewalServiceStopped = newLockError(ErrCodeRenewalServiceStopped, "renewal service stopped", nil)
	// ErrInvalidOptions indicates invalid configuration options
	ErrInvalidOptions = newLockError(ErrCodeInvalidOptions, "invalid options", nil)
)

// Error message internationalization mapping (can be extended as needed)
var errorMessages = map[string]map[string]string{
	"en": {
		ErrCodeLockNotHeld:           "Lock is not held by current instance",
		ErrCodeLockAcquireFailed:     "Failed to acquire lock",
		ErrCodeLockAcquireTimeout:    "Lock acquisition timeout",
		ErrCodeLockAcquireConflict:   "Lock acquisition conflict",
		ErrCodeRedisClientNotFound:   "Redis client not found",
		ErrCodeMaxRetriesExceeded:    "Maximum retries exceeded",
		ErrCodeLockFnRequired:        "Lock function is required",
		ErrCodeLockRenewalFailed:     "Lock renewal failed",
		ErrCodeRenewalServiceStopped: "Renewal service stopped",
		ErrCodeInvalidOptions:        "Invalid options provided",
	},
	"zh": {
		ErrCodeLockNotHeld:           "Lock is not held by current instance",
		ErrCodeLockAcquireFailed:     "Failed to acquire lock",
		ErrCodeLockAcquireTimeout:    "Lock acquisition timeout",
		ErrCodeLockAcquireConflict:   "Lock acquisition conflict",
		ErrCodeRedisClientNotFound:   "Redis client not found",
		ErrCodeMaxRetriesExceeded:    "Maximum retries exceeded",
		ErrCodeLockFnRequired:        "Lock function is required",
		ErrCodeLockRenewalFailed:     "Lock renewal failed",
		ErrCodeRenewalServiceStopped: "Renewal service stopped",
		ErrCodeInvalidOptions:        "Invalid options provided",
	},
}

// GetErrorMessage gets internationalized error message
func GetErrorMessage(code, lang string) string {
	if messages, ok := errorMessages[lang]; ok {
		if msg, ok := messages[code]; ok {
			return msg
		}
	}
	// Default to English
	if messages, ok := errorMessages["en"]; ok {
		if msg, ok := messages[code]; ok {
			return msg
		}
	}
	return code
}

// Error checking helper function
func IsLockError(err error, code string) bool {
	var lockErr *LockError
	if errors.As(err, &lockErr) {
		return lockErr.Code == code
	}
	return false
}
