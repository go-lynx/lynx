package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	lynx "github.com/go-lynx/lynx/plugins/nosql/redis"
)

// Lock 获取锁并执行函数（保持向后兼容）
func Lock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	options := DefaultLockOptions
	options.Expiration = expiration
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithRetry 获取锁并执行函数，支持重试（保持向后兼容）
func LockWithRetry(ctx context.Context, key string, expiration time.Duration, fn func() error, strategy RetryStrategy) error {
	options := DefaultLockOptions
	options.Expiration = expiration
	options.RetryStrategy = strategy
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithOptions 使用配置选项获取锁
func LockWithOptions(ctx context.Context, key string, options LockOptions, fn func() error) error {
	if fn == nil {
		return ErrLockFnRequired
	}

	// 验证配置选项
	if err := options.Validate(); err != nil {
		return newLockError(ErrCodeLockFnRequired, "invalid lock options", err)
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
		client:           client,
		key:              key,
		value:            value,
		expiration:       options.Expiration,
		renewalThreshold: options.RenewalThreshold,
		acquiredAt:       time.Now(),
	}

	// 尝试获取锁
	for retries := 0; ; retries++ {
		// 检查是否超过最大重试次数
		if options.RetryStrategy.MaxRetries > 0 && retries >= options.RetryStrategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}

		// 如果不是第一次尝试，等待重试间隔
		if retries > 0 {
			select {
			case <-time.After(options.RetryStrategy.RetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// 执行加锁脚本
		result, err := lock.client.Eval(ctx, lockScript, []string{lock.key},
			lock.value, lock.expiration.Milliseconds()).Result()
		if err != nil {
			return fmt.Errorf("lock script execution failed: %w", err)
		}

		// 检查是否成功获取锁
		switch result {
		case "OK":
			// 使用同一个时间戳，避免重复调用 time.Now()
			now := time.Now()
			lock.expiresAt = now.Add(lock.expiration)
			lock.acquiredAt = now

			// 触发获取锁回调
			globalCallback.OnLockAcquired(key, lock.expiration)

			// 如果启用续期，添加到全局锁管理器
			if options.RenewalEnabled {
				globalLockManager.mutex.Lock()
				globalLockManager.locks[key] = lock
				atomic.AddInt64(&globalLockManager.stats.TotalLocks, 1)
				atomic.AddInt64(&globalLockManager.stats.ActiveLocks, 1)
				globalLockManager.mutex.Unlock()
				// 启动续期服务
				globalLockManager.startRenewalService(options)
			}

			// 使用 defer 确保锁会被释放
			var err error
			defer func() {
				if unlockErr := Unlock(ctx, key); unlockErr != nil {
					log.ErrorCtx(ctx, "failed to unlock", "error", unlockErr)
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
			// 触发获取锁失败回调
			globalCallback.OnLockAcquireFailed(key, ErrLockAcquireFailed)
			// 如果不需要重试，直接返回错误
			if options.RetryStrategy.MaxRetries == 0 {
				return ErrLockAcquireFailed
			}
			// 否则继续重试
			continue
		default:
			return fmt.Errorf("unknown lock result: %v", result)
		}
	}
}

// TryLock 尝试获取锁，不阻塞
func TryLock(ctx context.Context, key string, expiration time.Duration) (*RedisLock, error) {
	client := lynx.GetRedis()
	if client == nil {
		return nil, ErrRedisClientNotFound
	}

	value := generateLockValue()
	lock := &RedisLock{
		client:           client,
		key:              key,
		value:            value,
		expiration:       expiration,
		renewalThreshold: DefaultLockOptions.RenewalThreshold,
		acquiredAt:       time.Now(),
	}

	result, err := lock.client.Eval(ctx, lockScript, []string{lock.key},
		lock.value, lock.expiration.Milliseconds()).Result()
	if err != nil {
		return nil, fmt.Errorf("lock script execution failed: %w", err)
	}

	switch result {
	case "OK":
		now := time.Now()
		lock.expiresAt = now.Add(lock.expiration)
		lock.acquiredAt = now
		return lock, nil
	case "LOCKED":
		return nil, ErrLockAcquireFailed
	default:
		return nil, fmt.Errorf("unknown lock result: %v", result)
	}
}

// Unlock 释放指定键的分布式锁
func Unlock(ctx context.Context, key string) error {
	// 获取锁实例
	globalLockManager.mutex.Lock()
	lock, exists := globalLockManager.locks[key]
	if !exists {
		globalLockManager.mutex.Unlock()
		return ErrLockNotHeld
	}
	delete(globalLockManager.locks, key)
	atomic.AddInt64(&globalLockManager.stats.ActiveLocks, -1)
	globalLockManager.mutex.Unlock()

	// 执行解锁脚本
	result, err := lock.client.Eval(ctx, unlockScript, []string{lock.key}, lock.value).Result()
	if err != nil {
		return err
	}

	// 检查是否成功释放锁
	switch result.(int64) {
	case 1: // 成功释放锁
		return nil
	case 0: // 锁不存在
		return ErrLockNotHeld
	case -1: // 锁存在但不是当前持有者
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %v", result)
	}
}
