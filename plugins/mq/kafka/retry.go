package kafka

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries  int           // 最大重试次数
	BackoffTime time.Duration // 初始退避时间
	MaxBackoff  time.Duration // 最大退避时间
}

// RetryHandler 重试处理器
type RetryHandler struct {
	config RetryConfig
}

// NewRetryHandler 创建新的重试处理器
func NewRetryHandler(config RetryConfig) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

// DoWithRetry 执行带重试的操作
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

			// 等待后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// 指数退避
			backoff *= 2
			if backoff > rh.config.MaxBackoff {
				backoff = rh.config.MaxBackoff
			}
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", rh.config.MaxRetries, lastErr)
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BackoffTime: time.Second,
		MaxBackoff:  30 * time.Second,
	}
}
