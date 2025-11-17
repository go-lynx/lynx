package nacos

import (
	"fmt"
)

// Error types for Nacos plugin
var (
	ErrNotInitialized = fmt.Errorf("nacos plugin not initialized")
	ErrSDKInitFailed  = fmt.Errorf("failed to initialize nacos SDK")
	ErrConfigInvalid  = fmt.Errorf("invalid nacos configuration")
	ErrServiceNotFound = fmt.Errorf("service not found")
	ErrConfigNotFound  = fmt.Errorf("configuration not found")
)

// WrapInitError wraps initialization errors with context
func WrapInitError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// WrapOperationError wraps operation errors with context
func WrapOperationError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("nacos %s failed: %w", operation, err)
}

