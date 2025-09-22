// Package grpc provides circuit breaker functionality for gRPC clients
package grpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitBreakerClosed allows requests to pass through
	CircuitBreakerClosed CircuitBreakerState = iota
	// CircuitBreakerOpen blocks requests and returns errors immediately
	CircuitBreakerOpen
	// CircuitBreakerHalfOpen allows a limited number of requests to test if the service has recovered
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "CLOSED"
	case CircuitBreakerOpen:
		return "OPEN"
	case CircuitBreakerHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig contains configuration for circuit breaker
type CircuitBreakerConfig struct {
	// Enabled indicates whether circuit breaker is enabled
	Enabled bool `json:"enabled"`
	// FailureThreshold is the number of failures that triggers the circuit breaker
	FailureThreshold int `json:"failure_threshold"`
	// RecoveryTimeout is the time to wait before transitioning from OPEN to HALF_OPEN
	RecoveryTimeout time.Duration `json:"recovery_timeout"`
	// SuccessThreshold is the number of successes needed in HALF_OPEN to transition to CLOSED
	SuccessThreshold int `json:"success_threshold"`
	// Timeout is the maximum time to wait for a request
	Timeout time.Duration `json:"timeout"`
	// MaxConcurrentRequests is the maximum number of concurrent requests allowed in HALF_OPEN state
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled:               false,
		FailureThreshold:      5,
		RecoveryTimeout:       30 * time.Second,
		SuccessThreshold:      3,
		Timeout:               10 * time.Second,
		MaxConcurrentRequests: 10,
	}
}

// CircuitBreaker implements the circuit breaker pattern for gRPC clients
type CircuitBreaker struct {
	config             *CircuitBreakerConfig
	state              CircuitBreakerState
	failures           int
	successes          int
	lastFailureTime    time.Time
	lastStateChange    time.Time
	concurrentRequests int
	mu                 sync.RWMutex
	metrics            *ClientMetrics
	serviceName        string
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(serviceName string, config *CircuitBreakerConfig, metrics *ClientMetrics) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config:          config,
		state:           CircuitBreakerClosed,
		lastStateChange: time.Now(),
		metrics:         metrics,
		serviceName:     serviceName,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.config.Enabled {
		return fn(ctx)
	}

	// Check if request should be allowed
	if !cb.allowRequest() {
		if cb.metrics != nil {
			cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
		}
		return fmt.Errorf("circuit breaker is OPEN for service %s", cb.serviceName)
	}

	// Create context with timeout
	var cancel context.CancelFunc
	if cb.config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()
	}

	// Execute the function
	err := fn(ctx)

	// Record the result
	cb.recordResult(err)

	return err
}

// allowRequest determines if a request should be allowed based on circuit breaker state
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		return true

	case CircuitBreakerOpen:
		// Check if recovery timeout has passed
		if now.Sub(cb.lastFailureTime) >= cb.config.RecoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.lastStateChange = now
			cb.concurrentRequests = 0
			cb.successes = 0
			if cb.metrics != nil {
				cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
			}
			return true
		}
		return false

	case CircuitBreakerHalfOpen:
		// Allow limited concurrent requests
		if cb.concurrentRequests < cb.config.MaxConcurrentRequests {
			cb.concurrentRequests++
			return true
		}
		return false

	default:
		return false
	}
}

// recordResult records the result of a request and updates circuit breaker state
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	if cb.state == CircuitBreakerHalfOpen {
		cb.concurrentRequests--
	}

	if cb.isFailure(err) {
		cb.failures++
		cb.lastFailureTime = now

		switch cb.state {
		case CircuitBreakerClosed:
			if cb.failures >= cb.config.FailureThreshold {
				cb.state = CircuitBreakerOpen
				cb.lastStateChange = now
				if cb.metrics != nil {
					cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
				}
			}

		case CircuitBreakerHalfOpen:
			// Any failure in half-open state transitions back to open
			cb.state = CircuitBreakerOpen
			cb.lastStateChange = now
			cb.failures = 1 // Reset failure count
			if cb.metrics != nil {
				cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
			}
		default:

		}
	} else {
		// Success
		switch cb.state {
		case CircuitBreakerClosed:
			// Reset failure count on success
			cb.failures = 0

		case CircuitBreakerHalfOpen:
			cb.successes++
			if cb.successes >= cb.config.SuccessThreshold {
				cb.state = CircuitBreakerClosed
				cb.lastStateChange = now
				cb.failures = 0
				cb.successes = 0
				if cb.metrics != nil {
					cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
				}
			}
		default:

		}
	}
}

// isFailure determines if an error should be considered a failure for circuit breaker purposes
func (cb *CircuitBreaker) isFailure(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation or timeout - these are not service failures
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check gRPC status codes
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.OK:
			return false
		case codes.Canceled:
			return false
		case codes.InvalidArgument:
			return false // Client error, not service failure
		case codes.NotFound:
			return false // Client error, not service failure
		case codes.AlreadyExists:
			return false // Client error, not service failure
		case codes.PermissionDenied:
			return false // Client error, not service failure
		case codes.Unauthenticated:
			return false // Client error, not service failure
		case codes.FailedPrecondition:
			return false // Client error, not service failure
		case codes.OutOfRange:
			return false // Client error, not service failure
		case codes.Unimplemented:
			return false // Client error, not service failure
		default:
			// All other codes are considered service failures
			return true
		}
	}

	// Default to considering it a failure
	return true
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"enabled":                 cb.config.Enabled,
		"service_name":            cb.serviceName,
		"state":                   cb.state.String(),
		"failures":                cb.failures,
		"successes":               cb.successes,
		"last_failure_time":       cb.lastFailureTime,
		"last_state_change":       cb.lastStateChange,
		"concurrent_requests":     cb.concurrentRequests,
		"failure_threshold":       cb.config.FailureThreshold,
		"recovery_timeout":        cb.config.RecoveryTimeout.String(),
		"success_threshold":       cb.config.SuccessThreshold,
		"timeout":                 cb.config.Timeout.String(),
		"max_concurrent_requests": cb.config.MaxConcurrentRequests,
	}
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitBreakerClosed
	cb.failures = 0
	cb.successes = 0
	cb.concurrentRequests = 0
	cb.lastStateChange = time.Now()

	if cb.metrics != nil {
		cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
	}
}

// UpdateConfig updates the circuit breaker configuration
func (cb *CircuitBreaker) UpdateConfig(config *CircuitBreakerConfig) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.config = config

	// If circuit breaker is disabled, reset to closed state
	if !config.Enabled && cb.state != CircuitBreakerClosed {
		cb.state = CircuitBreakerClosed
		cb.failures = 0
		cb.successes = 0
		cb.concurrentRequests = 0
		cb.lastStateChange = time.Now()

		if cb.metrics != nil {
			cb.metrics.RecordCircuitBreakerState(cb.serviceName, cb.state.String())
		}
	}
}

// CircuitBreakerManager manages multiple circuit breakers for different services
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	metrics  *ClientMetrics
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(metrics *ClientMetrics) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		metrics:  metrics,
	}
}

// GetCircuitBreaker gets or creates a circuit breaker for the given service
func (cbm *CircuitBreakerManager) GetCircuitBreaker(serviceName string, config *CircuitBreakerConfig) *CircuitBreaker {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if cb, exists := cbm.breakers[serviceName]; exists {
		// Update configuration if provided
		if config != nil {
			cb.UpdateConfig(config)
		}
		return cb
	}

	// Create new circuit breaker
	cb := NewCircuitBreaker(serviceName, config, cbm.metrics)
	cbm.breakers[serviceName] = cb
	return cb
}

// RemoveCircuitBreaker removes a circuit breaker for the given service
func (cbm *CircuitBreakerManager) RemoveCircuitBreaker(serviceName string) {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	delete(cbm.breakers, serviceName)
}

// GetAllStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetAllStats() map[string]interface{} {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	stats := make(map[string]interface{})
	for serviceName, cb := range cbm.breakers {
		stats[serviceName] = cb.GetStats()
	}
	return stats
}

// ResetAll resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	for _, cb := range cbm.breakers {
		cb.Reset()
	}
}

// Close closes all circuit breakers and cleans up resources
func (cbm *CircuitBreakerManager) Close() {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	cbm.breakers = make(map[string]*CircuitBreaker)
}
