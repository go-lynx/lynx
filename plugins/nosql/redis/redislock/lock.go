package redislock

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	lynx "github.com/go-lynx/lynx/plugins/nosql/redis"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockNotHeld 表示尝试释放未持有的锁
	ErrLockNotHeld = errors.New("lock not held")
	// ErrLockAcquireFailed 表示获取锁失败
	ErrLockAcquireFailed = errors.New("failed to acquire lock")
	// ErrRedisClientNotFound 表示未找到 Redis 客户端
	ErrRedisClientNotFound = errors.New("redis client not found")
	// ErrMaxRetriesExceeded 表示超过最大重试次数
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	// ErrLockFnRequired 表示锁保护的函数不能为空
	ErrLockFnRequired = errors.New("lock function is required")
	// ErrLockRenewalFailed 表示锁续期失败
	ErrLockRenewalFailed = errors.New("lock renewal failed")
)

// RedisLock 实现了基于 Redis 的分布式锁
type RedisLock struct {
	client     *redis.Client // Redis 客户端
	key        string        // 锁的键名
	value      string        // 锁的值（用于识别持有者）
	expiration time.Duration // 锁的过期时间
	expiresAt  atomic.Int64  // 锁的过期时间点（Unix 级别的时间戳）
}

// RetryStrategy 定义锁重试策略
type RetryStrategy struct {
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试间隔
}

// DefaultRetryStrategy 默认的重试策略
var DefaultRetryStrategy = RetryStrategy{
	MaxRetries: 3,
	RetryDelay: 100 * time.Millisecond,
}

// Lock 获取锁并执行函数
func Lock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	return LockWithRetry(ctx, key, expiration, fn, RetryStrategy{MaxRetries: 0})
}

// LockWithRetry 获取锁并执行函数，支持重试
func LockWithRetry(ctx context.Context, key string, expiration time.Duration, fn func() error, strategy RetryStrategy) error {
	if fn == nil {
		return ErrLockFnRequired
	}

	// 从应用程序获取 Redis 客户端
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}

	// 生成锁值
	value := generateLockValue()

	// 创建新的锁实例
	lock := &RedisLock{
		client:     client,
		key:        key,
		value:      value,
		expiration: expiration,
	}

	// 尝试获取锁
	for retries := 0; ; retries++ {
		// 检查是否超过最大重试次数
		if strategy.MaxRetries > 0 && retries >= strategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}

		// 如果不是第一次尝试，等待重试间隔
		if retries > 0 {
			select {
			case <-time.After(strategy.RetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// 执行加锁脚本
		result, err := lock.client.Eval(ctx, lockScript, []string{lock.key},
			lock.value, lock.expiration.Milliseconds()).Result()
		if err != nil {
			return err
		}

		// 检查是否成功获取锁
		switch result {
		case "OK":
			// 设置锁的过期时间点
			lock.expiresAt.Store(time.Now().Add(lock.expiration).UnixNano())
			// 添加到全局锁管理器
			globalLockManager.locks.Store(key, lock)
			// 启动续期服务
			globalLockManager.startRenewalService()

			// 使用 defer 确保锁会被释放
			var err error
			defer func() {
				if unlockErr := Unlock(ctx, key); unlockErr != nil {
					log.Error(ctx, "failed to unlock", "error", unlockErr)
					// 如果原始错误为空，则返回解锁错误
					if err == nil {
						err = unlockErr
					}
				}
			}()

			// 执行用户函数
			err = fn()
			return err
		case "LOCKED":
			// 如果不需要重试，直接返回错误
			if strategy.MaxRetries == 0 {
				return ErrLockAcquireFailed
			}
			// 否则继续重试
			continue
		default:
			return fmt.Errorf("unknown lock result: %v", result)
		}
	}
}

// Unlock 释放指定键的分布式锁
func Unlock(ctx context.Context, key string) error {
	// 获取锁实例
	value, exists := globalLockManager.locks.LoadAndDelete(key)
	if !exists {
		return ErrLockNotHeld
	}
	lock := value.(*RedisLock)

	// 执行解锁脚本
	result, err := lock.client.Eval(ctx, unlockScript, []string{lock.key}, lock.value).Result()
	if err != nil {
		return err
	}

	// 检查结果类型并处理解锁状态
	val, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unexpected result type: %T, value: %v", result, result)
	}

	// 检查是否成功释放锁
	switch val {
	case 1: // 成功释放锁
		return nil
	case 0: // 锁不存在
		return ErrLockNotHeld
	case -1: // 锁存在但不是当前持有者
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %v", val)
	}
}

// IsLocked 检查锁是否被当前实例持有
func (rl *RedisLock) IsLocked(ctx context.Context) (bool, error) {
	value, err := rl.client.Get(ctx, rl.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == rl.value, nil
}
