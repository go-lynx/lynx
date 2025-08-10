package redislock

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// GetKey 获取锁的键名
func (rl *RedisLock) GetKey() string {
	return rl.key
}

// GetExpiration 获取锁的过期时间
func (rl *RedisLock) GetExpiration() time.Duration {
	return rl.expiration
}

// GetExpiresAt 获取锁的过期时间点
func (rl *RedisLock) GetExpiresAt() time.Time {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.expiresAt
}

// GetAcquiredAt 获取锁的获取时间
func (rl *RedisLock) GetAcquiredAt() time.Time {
	return rl.acquiredAt
}

// GetToken 返回最近一次首次获取到的 fencing token（非重入时生成）。
// 若为 0 表示尚未在本进程首次获取成功（或仅发生了重入）。
func (rl *RedisLock) GetToken() int64 {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.token
}

// GetRemainingTime 获取锁的剩余时间
func (rl *RedisLock) GetRemainingTime() time.Duration {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Until(rl.expiresAt)
}

// GetStatus 获取锁的当前状态信息（避免重复时间计算）
func (rl *RedisLock) GetStatus() (remainingTime time.Duration, isExpired bool) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	now := time.Now()
	remainingTime = rl.expiresAt.Sub(now)
	isExpired = now.After(rl.expiresAt)
	return
}

// IsExpired 检查锁是否已过期
func (rl *RedisLock) IsExpired() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Now().After(rl.expiresAt)
}

// Renew 手动续期锁
func (rl *RedisLock) Renew(ctx context.Context, newExpiration time.Duration) error {
	// 为单次调用设置可选超时
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	start := time.Now()
	// 执行续期脚本（避免持锁进行网络调用）
	result, err := renewScript.Run(runCtx, rl.client, []string{rl.ownerKey, rl.countKey},
		rl.value, newExpiration.Milliseconds()).Result()
	if cancel != nil {
		cancel()
	}
	observeScriptLatency("renew", time.Since(start))
	if err != nil {
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return fmt.Errorf("renewal script execution failed: %w", err)
	}

	n, ok := result.(int64)
	if !ok {
		err := fmt.Errorf("unknown renewal result type: %T", result)
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return err
	}
	switch n {
	case 1: // 续期成功
		now := time.Now()
		rl.mutex.Lock()
		rl.expiration = newExpiration
		rl.expiresAt = now.Add(newExpiration)
		rl.mutex.Unlock()
		globalCallback.OnLockRenewed(rl.key, newExpiration)
		return nil
	case 0, -1, -2: // 锁不存在或不是当前持有者
		globalCallback.OnLockRenewalFailed(rl.key, ErrLockRenewalFailed)
		return ErrLockRenewalFailed
	default:
		err := fmt.Errorf("unknown renewal result: %d", n)
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return err
	}
}

// Release 释放锁
func (rl *RedisLock) Release(ctx context.Context) error {
	// 为单次调用设置可选超时
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	start := time.Now()
	// 执行解锁脚本（统一语义：部分释放不刷新 TTL，传 0）
	result, err := unlockScript.Run(runCtx, rl.client, []string{rl.ownerKey, rl.countKey}, rl.value, int64(0)).Result()
	if cancel != nil {
		cancel()
	}
	observeScriptLatency("unlock", time.Since(start))
	if err != nil {
		return fmt.Errorf("unlock script execution failed: %w", err)
	}

	// 检查是否成功释放锁
	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown unlock result type: %T", result)
	}
	switch n {
	case 2: // 部分释放（仍持有）
		return nil
	case 1: // 完全释放锁
		duration := time.Since(rl.acquiredAt)
		globalCallback.OnLockReleased(rl.key, duration)
		return nil
	case 0: // 锁不存在
		return ErrLockNotHeld
	case -1: // 锁存在但不是当前持有者
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %d", n)
	}
}

// IsLocked 检查锁是否被当前实例持有
func (rl *RedisLock) IsLocked(ctx context.Context) (bool, error) {
	// 为单次调用设置可选超时
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	value, err := rl.client.Get(runCtx, rl.ownerKey).Result()
	if cancel != nil {
		cancel()
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == rl.value, nil
}

// Acquire 尝试基于当前 RedisLock 实例获取（或重入）锁
// 若同实例再次调用，因 value 不变，Lua 脚本会将其视为可重入并续期
func (rl *RedisLock) Acquire(ctx context.Context) error {
	// 为单次调用设置可选超时
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	start := time.Now()
	result, err := lockScript.Run(runCtx, rl.client, []string{rl.ownerKey, rl.countKey}, rl.value, rl.expiration.Milliseconds()).Result()
	if cancel != nil {
		cancel()
	}
	observeScriptLatency("acquire", time.Since(start))
	if err != nil {
		return fmt.Errorf("lock script execution failed: %w", err)
	}

	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown lock result type: %T", result)
	}
	if n > 0 {
		now := time.Now()
		rl.mutex.Lock()
		rl.acquiredAt = now
		rl.expiresAt = now.Add(rl.expiration)
		rl.mutex.Unlock()
		// 如果是首次获取（非重入），生成并记录 fencing token
		if n == 1 {
			// 独立一次命令即可，因已持有锁期间不会有其他持有者
			// 可选超时
			tctx := ctx
			var tcancel context.CancelFunc
			if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
				tctx, tcancel = context.WithTimeout(ctx, to)
			}
			token, err := rl.client.Incr(tctx, rl.tokenKey).Result()
			if tcancel != nil {
				tcancel()
			}
			if err == nil {
				rl.mutex.Lock()
				rl.token = token
				rl.mutex.Unlock()
			}
		}
		globalCallback.OnLockAcquired(rl.key, rl.expiration)
		return nil
	}
	// 被其他持有者占用
	globalCallback.OnLockAcquireFailed(rl.key, ErrLockAcquireConflict)
	return ErrLockAcquireConflict
}

// AcquireWithRetry 获取（或重入）锁并按策略重试
func (rl *RedisLock) AcquireWithRetry(ctx context.Context, strategy RetryStrategy) error {
	retries := 0
	for {
		if strategy.MaxRetries > 0 && retries >= strategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}
		if retries > 0 {
			// 加入抖动，避免热点碰撞
			delay := strategy.RetryDelay
			if delay > 0 {
				jitter := time.Duration(float64(delay) * (0.5 + randFloat64()))
				if jitter > 0 {
					delay = jitter
				}
			}
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err := rl.Acquire(ctx)
		if err == nil {
			return nil
		}
		if err != ErrLockAcquireConflict {
			return err
		}
		// 冲突则继续按策略重试
		if strategy.MaxRetries == 0 {
			return ErrLockAcquireConflict
		}
		retries++
	}
}

// EnableAutoRenew 将当前锁注册到全局续期管理器（如尚未启动会启动）
func (rl *RedisLock) EnableAutoRenew(options LockOptions) {
	globalLockManager.mutex.Lock()
	if _, exists := globalLockManager.locks[rl.key]; !exists {
		globalLockManager.locks[rl.key] = rl
		atomic.AddInt64(&globalLockManager.stats.TotalLocks, 1)
		atomic.AddInt64(&globalLockManager.stats.ActiveLocks, 1)
	}
	globalLockManager.mutex.Unlock()
	globalLockManager.startRenewalService(options)
}
