package kafka

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig retry configuration
type RetryConfig struct {
	MaxRetries  int           // Maximum retry count
	BackoffTime time.Duration // Initial backoff time
	MaxBackoff  time.Duration // Maximum backoff time
}

// RetryHandler retry handler
type RetryHandler struct {
	config RetryConfig
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config RetryConfig) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

// DoWithRetry executes operation with retry
func (rh *RetryHandler) DoWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error
	backoff := rh.config.BackoffTime

	for attempt := 0; attempt <= rh.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
			if attempt == rh.config.MaxRetries {
				break
			}

			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff
			backoff *= 2
			if backoff > rh.config.MaxBackoff {
				backoff = rh.config.MaxBackoff
			}
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", rh.config.MaxRetries, lastErr)
}

// DefaultRetryConfig default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BackoffTime: time.Second,
		MaxBackoff:  30 * time.Second,
	}
}
