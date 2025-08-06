package polaris

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// RetryManager 重试管理器
// 提供指数退避重试机制
type RetryManager struct {
	maxRetries    int
	retryInterval time.Duration
	backoffFactor float64
}

// NewRetryManager 创建新的重试管理器
func NewRetryManager(maxRetries int, retryInterval time.Duration) *RetryManager {
	return &RetryManager{
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
		backoffFactor: 2.0, // 指数退避因子
	}
}

// DoWithRetry 执行带重试的操作
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
				// 计算退避时间
				backoffTime := r.calculateBackoff(attempt)
				log.Warnf("Operation failed (attempt %d/%d): %v, retrying in %v",
					attempt+1, r.maxRetries+1, err, backoffTime)
				time.Sleep(backoffTime)
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %w", r.maxRetries+1, lastErr)
}

// DoWithRetryContext 执行带重试的操作（支持上下文）
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

// calculateBackoff 计算退避时间
func (r *RetryManager) calculateBackoff(attempt int) time.Duration {
	// 指数退避：base * factor^attempt
	backoffSeconds := float64(r.retryInterval) * math.Pow(r.backoffFactor, float64(attempt))

	// 限制最大退避时间为 30 秒
	maxBackoff := 30 * time.Second
	if time.Duration(backoffSeconds) > maxBackoff {
		return maxBackoff
	}

	return time.Duration(backoffSeconds)
}

// CircuitBreaker 熔断器
// 实现简单的熔断保护机制
type CircuitBreaker struct {
	threshold    float64
	failureCount int
	successCount int
	lastFailure  time.Time
	state        CircuitState
	mu           chan struct{} // 用作互斥锁
}

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitStateClosed   CircuitState = iota // 关闭状态：正常
	CircuitStateOpen                         // 开启状态：熔断
	CircuitStateHalfOpen                     // 半开状态：尝试恢复
)

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(threshold float64) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		state:     CircuitStateClosed,
		mu:        make(chan struct{}, 1), // 缓冲通道用作互斥锁
	}
}

// Do 执行带熔断保护的操作
func (cb *CircuitBreaker) Do(operation func() error) error {
	// 获取锁
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()

	// 检查熔断器状态
	switch cb.state {
	case CircuitStateOpen:
		// 检查是否应该尝试恢复
		if time.Since(cb.lastFailure) > 30*time.Second {
			cb.state = CircuitStateHalfOpen
			log.Infof("Circuit breaker transitioning to half-open state")
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	case CircuitStateHalfOpen:
		// 半开状态，允许一次尝试
		log.Infof("Circuit breaker in half-open state, allowing one attempt")
	}

	// 执行操作
	err := operation()

	// 更新状态
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// recordFailure 记录失败
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()

	// 计算失败率
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

// recordSuccess 记录成功
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	if cb.state == CircuitStateHalfOpen {
		// 半开状态下成功，重置为关闭状态
		cb.state = CircuitStateClosed
		cb.resetCounters()
		log.Infof("Circuit breaker closed after successful attempt")
	}
}

// resetCounters 重置计数器
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	return cb.state
}

// GetFailureRate 获取失败率
func (cb *CircuitBreaker) GetFailureRate() float64 {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()

	total := cb.failureCount + cb.successCount
	if total == 0 {
		return 0
	}
	return float64(cb.failureCount) / float64(total)
}

// ForceOpen 强制开启熔断器
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	cb.state = CircuitStateOpen
	log.Warnf("Circuit breaker forced open")
}

// ForceClose 强制关闭熔断器
func (cb *CircuitBreaker) ForceClose() {
	cb.mu <- struct{}{}
	defer func() { <-cb.mu }()
	cb.state = CircuitStateClosed
	cb.resetCounters()
	log.Infof("Circuit breaker forced closed")
}
