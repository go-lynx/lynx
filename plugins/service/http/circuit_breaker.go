// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app/log"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitBreakerClosed - normal operation
	CircuitBreakerClosed CircuitBreakerState = iota
	// CircuitBreakerOpen - circuit is open, requests are rejected
	CircuitBreakerOpen
	// CircuitBreakerHalfOpen - testing if service has recovered
	CircuitBreakerHalfOpen
)

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the maximum number of failures before opening the circuit
	MaxFailures int32
	// Timeout is how long to wait before attempting to close the circuit
	Timeout time.Duration
	// MaxRequests is the maximum number of requests allowed in half-open state
	MaxRequests int32
	// FailureThreshold is the failure rate threshold (0.0 to 1.0)
	FailureThreshold float64
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	config       CircuitBreakerConfig
	state        CircuitBreakerState
	failures     int32
	requests     int32
	successes    int32
	lastFailTime time.Time
	mutex        sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	// Set defaults if not provided
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 10
	}
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 0.5
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
	}
}

// Allow checks if a request should be allowed through
func (cb *CircuitBreaker) Allow() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			cb.state = CircuitBreakerHalfOpen
			cb.requests = 0
			cb.successes = 0
			log.Infof("Circuit breaker transitioning to half-open state")
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return cb.requests < cb.config.MaxRequests
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successes++
	cb.requests++

	if cb.state == CircuitBreakerHalfOpen {
		// Check if we should close the circuit
		if cb.successes >= cb.config.MaxRequests {
			cb.state = CircuitBreakerClosed
			cb.failures = 0
			cb.requests = 0
			cb.successes = 0
			log.Infof("Circuit breaker closed - service recovered")
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.requests++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		// Check if we should open the circuit
		if cb.failures >= cb.config.MaxFailures {
			cb.state = CircuitBreakerOpen
			log.Warnf("Circuit breaker opened - too many failures (%d)", cb.failures)
		}
	case CircuitBreakerHalfOpen:
		// Return to open state
		cb.state = CircuitBreakerOpen
		log.Warnf("Circuit breaker returned to open state - failure in half-open")
	default:

	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() (int32, int32, int32, CircuitBreakerState) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failures, cb.requests, cb.successes, cb.state
}

// circuitBreakerMiddleware creates a circuit breaker middleware for HTTP requests
func (h *ServiceHttp) circuitBreakerMiddleware() middleware.Middleware {
	// Skip if circuit breaker is disabled
	if h.conf.CircuitBreaker == nil || !h.conf.CircuitBreaker.Enabled {
		return func(handler middleware.Handler) middleware.Handler {
			return handler
		}
	}

	// Create circuit breaker with configuration if not already created
	if h.circuitBreaker == nil {
		config := CircuitBreakerConfig{
			MaxFailures:      h.conf.CircuitBreaker.MaxFailures,
			Timeout:          h.conf.CircuitBreaker.Timeout.AsDuration(),
			MaxRequests:      h.conf.CircuitBreaker.MaxRequests,
			FailureThreshold: h.conf.CircuitBreaker.FailureThreshold,
		}
		h.circuitBreaker = NewCircuitBreaker(config)
	}
	cb := h.circuitBreaker

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Check if request should be allowed
			if !cb.Allow() {
				// Circuit is open, reject request
				method := "unknown"
				path := "unknown"
				if tr, ok := transport.FromServerContext(ctx); ok {
					method = tr.RequestHeader().Get("X-HTTP-Method")
					if method == "" {
						method = "POST"
					}
					path = tr.Operation()
				}

				// Record circuit breaker rejection
				if h.errorCounter != nil {
					h.errorCounter.WithLabelValues(method, path, "circuit_breaker_open").Inc()
				}

				return nil, fmt.Errorf("circuit breaker is open - service unavailable")
			}

			// Execute the request
			reply, err = handler(ctx, req)

			// Record result
			if err != nil {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}

			return reply, err
		}
	}
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (h *ServiceHttp) GetCircuitBreakerStats() map[string]interface{} {
	if h.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	failures, requests, successes, state := h.circuitBreaker.GetStats()

	return map[string]interface{}{
		"enabled":   true,
		"state":     state,
		"failures":  failures,
		"requests":  requests,
		"successes": successes,
	}
}
