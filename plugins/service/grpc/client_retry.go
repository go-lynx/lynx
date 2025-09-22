package grpc

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryHandler handles retry logic for gRPC client requests
type RetryHandler struct {
	maxRetries        int
	retryBackoff      time.Duration
	maxBackoff        time.Duration
	backoffMultiplier float64
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler() *RetryHandler {
	return &RetryHandler{
		maxRetries:        3,
		retryBackoff:      time.Second,
		maxBackoff:        30 * time.Second,
		backoffMultiplier: 2.0,
	}
}

// Initialize initializes the retry handler with configuration
func (r *RetryHandler) Initialize(maxRetries int, retryBackoff time.Duration) {
	r.maxRetries = maxRetries
	r.retryBackoff = retryBackoff
}

// ExecuteWithRetry executes a request with retry logic
func (r *RetryHandler) ExecuteWithRetry(ctx context.Context, handler func(context.Context, interface{}) (interface{}, error), req interface{}) (interface{}, error) {
	var lastErr error
	backoff := r.retryBackoff

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		// Execute the request
		resp, err := handler(ctx, req)

		// If successful, return the response
		if err == nil {
			if attempt > 0 {
				log.Infof("Request succeeded after %d retries", attempt)
			}
			return resp, nil
		}

		lastErr = err

		// Check if the error is retryable
		if !r.isRetryableError(err) {
			log.Debugf("Error is not retryable: %v", err)
			return nil, err
		}

		// If this was the last attempt, return the error
		if attempt == r.maxRetries {
			log.Errorf("Request failed after %d retries: %v", r.maxRetries, err)
			return nil, err
		}

		// Log retry attempt
		log.Warnf("Request failed (attempt %d/%d), retrying in %v: %v",
			attempt+1, r.maxRetries+1, backoff, err)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}

		// Calculate next backoff duration
		backoff = time.Duration(float64(backoff) * r.backoffMultiplier)
		if backoff > r.maxBackoff {
			backoff = r.maxBackoff
		}
	}

	return nil, lastErr
}

// isRetryableError determines if an error is retryable
func (r *RetryHandler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check gRPC status codes
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
			return true
		case codes.Unauthenticated, codes.PermissionDenied, codes.InvalidArgument:
			return false
		case codes.NotFound, codes.AlreadyExists, codes.FailedPrecondition:
			return false
		case codes.Aborted, codes.OutOfRange:
			return true
		case codes.Internal, codes.DataLoss:
			return true
		default:
			return false
		}
	}

	// Check for context errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	// Default to retryable for unknown errors
	return true
}

// GetRetryConfig returns the current retry configuration
func (r *RetryHandler) GetRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        r.maxRetries,
		RetryBackoff:      r.retryBackoff,
		MaxBackoff:        r.maxBackoff,
		BackoffMultiplier: r.backoffMultiplier,
	}
}

// SetRetryConfig updates the retry configuration
func (r *RetryHandler) SetRetryConfig(config RetryConfig) {
	r.maxRetries = config.MaxRetries
	r.retryBackoff = config.RetryBackoff
	r.maxBackoff = config.MaxBackoff
	r.backoffMultiplier = config.BackoffMultiplier
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxRetries        int
	RetryBackoff      time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		RetryBackoff:      time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// ExponentialBackoff calculates exponential backoff duration
func ExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration, multiplier float64) time.Duration {
	delay := time.Duration(float64(baseDelay) * math.Pow(multiplier, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// Jitter adds random jitter to backoff duration
func Jitter(delay time.Duration, jitterPercent float64) time.Duration {
	if jitterPercent <= 0 || jitterPercent >= 1 {
		return delay
	}

	jitter := time.Duration(float64(delay) * jitterPercent * (2*rand.Float64() - 1))
	return delay + jitter
}
