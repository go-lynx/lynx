package rabbitmq

import (
	"context"
	"time"
)

// RetryConfig defines retry configuration
type RetryConfig struct {
	MaxRetries  int
	BackoffTime time.Duration
	MaxBackoff  time.Duration
}

// RetryHandler handles retry logic
type RetryHandler struct {
	config RetryConfig
}

// NewRetryHandler creates a new RetryHandler
func NewRetryHandler(config RetryConfig) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

// DoWithRetry executes operation with retry
func (r *RetryHandler) DoWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error
	backoff := r.config.BackoffTime

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute operation
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// If this is the last attempt, don't wait
		if attempt == r.config.MaxRetries {
			break
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff with max limit
		backoff *= 2
		if backoff > r.config.MaxBackoff {
			backoff = r.config.MaxBackoff
		}
	}

	return WrapError(lastErr, "max retries exceeded")
}

// GetRetryConfig returns the retry configuration
func (r *RetryHandler) GetRetryConfig() RetryConfig {
	return r.config
}

// SetRetryConfig sets the retry configuration
func (r *RetryHandler) SetRetryConfig(config RetryConfig) {
	r.config = config
}
