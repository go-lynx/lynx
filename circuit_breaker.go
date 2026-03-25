package lynx

import (
	"sync"
	"time"
)

// CircuitBreaker provides error handling and recovery.
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        CircuitState
	failureCount int
	successCount int
	lastFailure  time.Time
	threshold    int
	timeout      time.Duration
}

// CircuitState represents the state of circuit breaker.
type CircuitState int

const (
	CircuitStateClosed CircuitState = iota
	CircuitStateOpen
	CircuitStateHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker with the provided threshold and timeout.
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	return &CircuitBreaker{
		state:     CircuitStateClosed,
		threshold: threshold,
		timeout:   timeout,
	}
}

// CanExecute checks if the circuit breaker allows execution.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitStateHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordResult records the result of an operation.
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()

		if cb.state == CircuitStateClosed && cb.failureCount >= cb.threshold {
			cb.state = CircuitStateOpen
		} else if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateOpen
		}
	} else {
		cb.successCount++

		if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateClosed
			cb.resetCounters()
		}
	}
}

func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
