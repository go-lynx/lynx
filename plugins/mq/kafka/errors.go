package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrProducerNotInitialized  = errors.New("kafka producer not initialized")
	ErrConsumerNotInitialized  = errors.New("kafka consumer not initialized")
	ErrInvalidConfiguration    = errors.New("invalid kafka configuration")
	ErrNoBrokersConfigured     = errors.New("no kafka brokers configured")
	ErrConsumerNotEnabled      = errors.New("kafka consumer is not enabled")
	ErrProducerNotEnabled      = errors.New("kafka producer is not enabled")
	ErrInvalidCompression      = errors.New("invalid compression type")
	ErrInvalidSASLMechanism    = errors.New("invalid SASL mechanism")
	ErrInvalidStartOffset      = errors.New("invalid start offset")
	ErrNoTopicsSpecified       = errors.New("no topics specified for subscription")
	ErrNoGroupID               = errors.New("consumer group ID is required")
	ErrConnectionFailed        = errors.New("failed to connect to kafka brokers")
	ErrMessageProcessingFailed = errors.New("failed to process message")
	ErrOffsetCommitFailed      = errors.New("failed to commit offsets")

	// New error types
	ErrCircuitBreakerOpen     = errors.New("circuit breaker is open")
	ErrMessageTooLarge        = errors.New("message size exceeds limit")
	ErrTopicNotFound          = errors.New("topic not found")
	ErrPartitionNotFound      = errors.New("partition not found")
	ErrAuthenticationFailed   = errors.New("authentication failed")
	ErrAuthorizationFailed    = errors.New("authorization failed")
	ErrNetworkTimeout         = errors.New("network timeout")
	ErrBrokerUnavailable      = errors.New("broker unavailable")
	ErrMessageSerialization   = errors.New("message serialization failed")
	ErrMessageDeserialization = errors.New("message deserialization failed")
)

// ErrorType error type
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeConfiguration
	ErrorTypeAuthentication
	ErrorTypeAuthorization
	ErrorTypeSerialization
	ErrorTypeBusiness
	ErrorTypeSystem
)

// Error enhanced error type
type Error struct {
	Type    ErrorType
	Message string
	Cause   error
	Time    time.Time
	Context map[string]interface{}
}

// Error implements error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the original error
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError creates a new error
func NewError(errType ErrorType, message string, cause error) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Cause:   cause,
		Time:    time.Now(),
		Context: make(map[string]interface{}),
	}
}

// CircuitBreaker circuit breaker
type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitBreakerState
	failureCount    int
	lastFailureTime time.Time
	lastSuccessTime time.Time
	threshold       int
	timeout         time.Duration
	halfOpenLimit   int
	halfOpenCount   int
}

// CircuitBreakerState circuit breaker state
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         CircuitBreakerClosed,
		threshold:     threshold,
		timeout:       timeout,
		halfOpenLimit: 5,
	}
}

// Call executes a call with circuit breaker
func (cb *CircuitBreaker) Call(operation func() error) error {
	if !cb.canExecute() {
		return ErrCircuitBreakerOpen
	}

	err := operation()
	cb.recordResult(err)
	return err
}

// canExecute checks if execution is allowed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenCount = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return cb.halfOpenCount < cb.halfOpenLimit
	default:
		return false
	}
}

// recordResult records the execution result
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.state == CircuitBreakerClosed && cb.failureCount >= cb.threshold {
			cb.state = CircuitBreakerOpen
		} else if cb.state == CircuitBreakerHalfOpen {
			cb.state = CircuitBreakerOpen
		}
	} else {
		cb.failureCount = 0
		cb.lastSuccessTime = time.Now()

		if cb.state == CircuitBreakerHalfOpen {
			cb.state = CircuitBreakerClosed
		}
	}

	if cb.state == CircuitBreakerHalfOpen {
		cb.halfOpenCount++
	}
}

// GetState gets the circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats gets circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":           cb.state,
		"failure_count":   cb.failureCount,
		"threshold":       cb.threshold,
		"timeout":         cb.timeout.String(),
		"last_failure":    cb.lastFailureTime,
		"last_success":    cb.lastSuccessTime,
		"half_open_count": cb.halfOpenCount,
		"half_open_limit": cb.halfOpenLimit,
	}
}

// RetryStrategy retry strategy
type RetryStrategy struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
	Jitter         bool
}

// NewRetryStrategy creates a new retry strategy
func NewRetryStrategy(maxRetries int, initialBackoff, maxBackoff time.Duration) *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:     maxRetries,
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		BackoffFactor:  2.0,
		Jitter:         true,
	}
}

// ExecuteWithRetry executes an operation with retry
func (rs *RetryStrategy) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error
	backoff := rs.InitialBackoff

	for attempt := 0; attempt <= rs.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
			if attempt == rs.MaxRetries {
				break
			}

			// Calculate backoff time
			if rs.Jitter {
				backoff = rs.addJitter(backoff)
			}

			// Wait and retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff
			backoff = time.Duration(float64(backoff) * rs.BackoffFactor)
			if backoff > rs.MaxBackoff {
				backoff = rs.MaxBackoff
			}
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", rs.MaxRetries, lastErr)
}

// addJitter adds jitter
func (rs *RetryStrategy) addJitter(backoff time.Duration) time.Duration {
	// Simple jitter implementation: add ±25% random time
	jitter := time.Duration(float64(backoff) * 0.25)
	return backoff + time.Duration(float64(jitter)*(0.5-0.5*float64(time.Now().UnixNano()%2)))
}

// ErrorHandler error handler
type ErrorHandler struct {
	circuitBreaker *CircuitBreaker
	retryStrategy  *RetryStrategy
	errorCallbacks map[ErrorType][]func(*Error)
	mu             sync.RWMutex
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
		retryStrategy:  NewRetryStrategy(3, time.Second, 30*time.Second),
		errorCallbacks: make(map[ErrorType][]func(*Error)),
	}
}

// HandleError handles an error
func (eh *ErrorHandler) HandleError(err *Error) {
	eh.mu.RLock()
	callbacks := eh.errorCallbacks[err.Type]
	eh.mu.RUnlock()

	for _, callback := range callbacks {
		callback(err)
	}
}

// RegisterErrorCallback registers an error callback
func (eh *ErrorHandler) RegisterErrorCallback(errType ErrorType, callback func(*Error)) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.errorCallbacks[errType] = append(eh.errorCallbacks[errType], callback)
}

// ExecuteWithErrorHandling executes an operation with error handling
func (eh *ErrorHandler) ExecuteWithErrorHandling(ctx context.Context, operation func() error) error {
	return eh.circuitBreaker.Call(func() error {
		return eh.retryStrategy.ExecuteWithRetry(ctx, operation)
	})
}

// GetCircuitBreaker gets the circuit breaker
func (eh *ErrorHandler) GetCircuitBreaker() *CircuitBreaker {
	return eh.circuitBreaker
}

// GetRetryStrategy gets the retry strategy
func (eh *ErrorHandler) GetRetryStrategy() *RetryStrategy {
	return eh.retryStrategy
}
