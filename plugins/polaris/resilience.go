package polaris

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// RetryManager retry manager
// Provides exponential backoff retry mechanism
type RetryManager struct {
	maxRetries    int
	retryInterval time.Duration
	backoffFactor float64
}

// NewRetryManager creates new retry manager
func NewRetryManager(maxRetries int, retryInterval time.Duration) *RetryManager {
	return &RetryManager{
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
		backoffFactor: 2.0, // Exponential backoff factor
	}
}

// DoWithRetry executes operation with retry
func (r *RetryManager) DoWithRetry(operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if err := operation(); err == nil {
			if attempt > 0 {
				log.Infof("Operation succeeded after %d retries", attempt)
			}
			return nil
		} else {
			lastErr = err
			if attempt < r.maxRetries {
				// Calculate backoff time
				backoffTime := r.calculateBackoff(attempt)
				log.Warnf("Operation failed (attempt %d/%d): %v, retrying in %v",
					attempt+1, r.maxRetries+1, err, backoffTime)
				time.Sleep(backoffTime)
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %w", r.maxRetries+1, lastErr)
}

// DoWithRetryContext executes operation with retry (supports context)
func (r *RetryManager) DoWithRetryContext(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		if err := operation(); err == nil {
			if attempt > 0 {
				log.Infof("Operation succeeded after %d retries", attempt)
			}
			return nil
		} else {
			lastErr = err
			if attempt < r.maxRetries {
				backoffTime := r.calculateBackoff(attempt)
				log.Warnf("Operation failed (attempt %d/%d): %v, retrying in %v",
					attempt+1, r.maxRetries+1, err, backoffTime)

				select {
				case <-time.After(backoffTime):
				case <-ctx.Done():
					return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
				}
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %w", r.maxRetries+1, lastErr)
}

// calculateBackoff calculates backoff time
func (r *RetryManager) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base * factor^attempt
	backoffSeconds := float64(r.retryInterval) * math.Pow(r.backoffFactor, float64(attempt))

	// Limit maximum backoff time to 30 seconds
	maxBackoff := 30 * time.Second
	if time.Duration(backoffSeconds) > maxBackoff {
		return maxBackoff
	}

	return time.Duration(backoffSeconds)
}

// CircuitBreaker circuit breaker
// Implements simple circuit breaker protection mechanism
type CircuitBreaker struct {
	threshold    float64
	failureCount int
	successCount int
	lastFailure  time.Time
	state        CircuitState
	mu           chan struct{} // Used as mutex lock
}

// CircuitState circuit breaker state
type CircuitState int

const (
	CircuitStateClosed   CircuitState = iota // Closed state: normal
	CircuitStateOpen                         // Open state: circuit broken
	CircuitStateHalfOpen                     // Half-open state: attempting recovery
)

// NewCircuitBreaker creates new circuit breaker
func NewCircuitBreaker(threshold float64) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		state:     CircuitStateClosed,
		mu:        make(chan struct{}, 1), // Buffered channel used as mutex lock
	}
}

// Do executes operation with circuit breaker protection
func (cb *CircuitBreaker) Do(operation func() error) error {
	// Acquire lock
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()

	// Check circuit breaker state
	switch cb.state {
	case CircuitStateOpen:
		// Check if should attempt recovery
		if time.Since(cb.lastFailure) > 30*time.Second {
			cb.state = CircuitStateHalfOpen
			log.Infof("Circuit breaker transitioning to half-open state")
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	case CircuitStateHalfOpen:
		// Half-open state, allow one attempt
		log.Infof("Circuit breaker in half-open state, allowing one attempt")
	case CircuitStateClosed:
		// Closed state: allow normal operation
		// No state change needed here
	default:
		return fmt.Errorf("invalid circuit breaker state: %v", cb.state)
	}

	// Execute operation
	err := operation()

	// Update state
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// recordFailure records failure
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()

	// Calculate failure rate
	failureRate := float64(cb.failureCount) / float64(cb.failureCount+cb.successCount)

	if cb.state == CircuitStateClosed && failureRate >= cb.threshold {
		cb.state = CircuitStateOpen
		log.Warnf("Circuit breaker opened: failure rate %.2f >= threshold %.2f",
			failureRate, cb.threshold)
	} else if cb.state == CircuitStateHalfOpen {
		cb.state = CircuitStateOpen
		log.Warnf("Circuit breaker reopened after failed attempt")
	}
}

// recordSuccess records success
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	if cb.state == CircuitStateHalfOpen {
		// Success in half-open state, reset to closed state
		cb.state = CircuitStateClosed
		cb.resetCounters()
		log.Infof("Circuit breaker closed after successful attempt")
	}
}

// resetCounters resets counters
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState gets circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	return cb.state
}

// GetFailureRate gets failure rate
func (cb *CircuitBreaker) GetFailureRate() float64 {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()

	total := cb.failureCount + cb.successCount
	if total == 0 {
		return 0
	}
	return float64(cb.failureCount) / float64(total)
}

// ForceOpen forces circuit breaker to open
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	cb.state = CircuitStateOpen
	log.Warnf("Circuit breaker forced open")
}

// ForceClose forces circuit breaker to close
func (cb *CircuitBreaker) ForceClose() {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	cb.state = CircuitStateClosed
	cb.resetCounters()
	log.Infof("Circuit breaker forced closed")
}
