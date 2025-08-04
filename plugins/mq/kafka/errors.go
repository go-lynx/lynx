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

	// 新增错误类型
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

// ErrorType 错误类型
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

// Error 增强的错误类型
type Error struct {
	Type    ErrorType
	Message string
	Cause   error
	Time    time.Time
	Context map[string]interface{}
}

// Error 实现 error 接口
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap 返回原始错误
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError 创建新的错误
func NewError(errType ErrorType, message string, cause error) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Cause:   cause,
		Time:    time.Now(),
		Context: make(map[string]interface{}),
	}
}

// CircuitBreaker 熔断器
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

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         CircuitBreakerClosed,
		threshold:     threshold,
		timeout:       timeout,
		halfOpenLimit: 5,
	}
}

// Call 执行带熔断器的调用
func (cb *CircuitBreaker) Call(operation func() error) error {
	if !cb.canExecute() {
		return ErrCircuitBreakerOpen
	}

	err := operation()
	cb.recordResult(err)
	return err
}

// canExecute 检查是否可以执行
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

// recordResult 记录执行结果
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

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取熔断器统计信息
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

// RetryStrategy 重试策略
type RetryStrategy struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
	Jitter         bool
}

// NewRetryStrategy 创建新的重试策略
func NewRetryStrategy(maxRetries int, initialBackoff, maxBackoff time.Duration) *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:     maxRetries,
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		BackoffFactor:  2.0,
		Jitter:         true,
	}
}

// ExecuteWithRetry 执行带重试的操作
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

			// 计算退避时间
			if rs.Jitter {
				backoff = rs.addJitter(backoff)
			}

			// 等待后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// 指数退避
			backoff = time.Duration(float64(backoff) * rs.BackoffFactor)
			if backoff > rs.MaxBackoff {
				backoff = rs.MaxBackoff
			}
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", rs.MaxRetries, lastErr)
}

// addJitter 添加抖动
func (rs *RetryStrategy) addJitter(backoff time.Duration) time.Duration {
	// 简单的抖动实现：添加 ±25% 的随机时间
	jitter := time.Duration(float64(backoff) * 0.25)
	return backoff + time.Duration(float64(jitter)*(0.5-0.5*float64(time.Now().UnixNano()%2)))
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	circuitBreaker *CircuitBreaker
	retryStrategy  *RetryStrategy
	errorCallbacks map[ErrorType][]func(*Error)
	mu             sync.RWMutex
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
		retryStrategy:  NewRetryStrategy(3, time.Second, 30*time.Second),
		errorCallbacks: make(map[ErrorType][]func(*Error)),
	}
}

// HandleError 处理错误
func (eh *ErrorHandler) HandleError(err *Error) {
	eh.mu.RLock()
	callbacks := eh.errorCallbacks[err.Type]
	eh.mu.RUnlock()

	for _, callback := range callbacks {
		callback(err)
	}
}

// RegisterErrorCallback 注册错误回调
func (eh *ErrorHandler) RegisterErrorCallback(errType ErrorType, callback func(*Error)) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.errorCallbacks[errType] = append(eh.errorCallbacks[errType], callback)
}

// ExecuteWithErrorHandling 执行带错误处理的操作
func (eh *ErrorHandler) ExecuteWithErrorHandling(ctx context.Context, operation func() error) error {
	return eh.circuitBreaker.Call(func() error {
		return eh.retryStrategy.ExecuteWithRetry(ctx, operation)
	})
}

// GetCircuitBreaker 获取熔断器
func (eh *ErrorHandler) GetCircuitBreaker() *CircuitBreaker {
	return eh.circuitBreaker
}

// GetRetryStrategy 获取重试策略
func (eh *ErrorHandler) GetRetryStrategy() *RetryStrategy {
	return eh.retryStrategy
}
