package redislock

import (
	"context"
	"errors"
	"fmt"
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

// GetRemainingTime 获取锁的剩余时间
func (rl *RedisLock) GetRemainingTime() time.Duration {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Until(rl.expiresAt)
}

// IsExpired 检查锁是否已过期
func (rl *RedisLock) IsExpired() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Now().After(rl.expiresAt)
}

// Renew 手动续期锁
func (rl *RedisLock) Renew(ctx context.Context, newExpiration time.Duration) error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// 执行续期脚本
	result, err := rl.client.Eval(ctx, renewScript, []string{rl.key},
		rl.value, newExpiration.Milliseconds()).Result()
	if err != nil {
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return fmt.Errorf("renewal script execution failed: %w", err)
	}

	switch result.(int64) {
	case 1: // 续期成功
		rl.expiration = newExpiration
		rl.expiresAt = time.Now().Add(newExpiration)
		globalCallback.OnLockRenewed(rl.key, newExpiration)
		return nil
	case 0, -1, -2: // 锁不存在或不是当前持有者
		globalCallback.OnLockRenewalFailed(rl.key, ErrLockRenewalFailed)
		return ErrLockRenewalFailed
	default:
		err := fmt.Errorf("unknown renewal result: %v", result)
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return err
	}
}

// Release 释放锁
func (rl *RedisLock) Release(ctx context.Context) error {
	// 执行解锁脚本
	result, err := rl.client.Eval(ctx, unlockScript, []string{rl.key}, rl.value).Result()
	if err != nil {
		return fmt.Errorf("unlock script execution failed: %w", err)
	}

	// 检查是否成功释放锁
	switch result.(int64) {
	case 1: // 成功释放锁
		duration := time.Since(rl.acquiredAt)
		globalCallback.OnLockReleased(rl.key, duration)
		return nil
	case 0: // 锁不存在
		return ErrLockNotHeld
	case -1: // 锁存在但不是当前持有者
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %v", result)
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
