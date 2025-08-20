package redislock

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// GetKey gets the lock key name
func (rl *RedisLock) GetKey() string {
	return rl.key
}

// GetExpiration gets the lock expiration time
func (rl *RedisLock) GetExpiration() time.Duration {
	return rl.expiration
}

// GetExpiresAt gets the lock expiration time point
func (rl *RedisLock) GetExpiresAt() time.Time {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.expiresAt
}

// GetAcquiredAt gets the lock acquisition time
func (rl *RedisLock) GetAcquiredAt() time.Time {
	return rl.acquiredAt
}

// GetToken returns the most recently acquired fencing token (generated on non-reentrant acquisition).
// If 0, it means the lock has not been successfully acquired for the first time in this process
// (or only reentry occurred).
func (rl *RedisLock) GetToken() int64 {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return rl.token
}

// GetRemainingTime gets the remaining time of the lock
func (rl *RedisLock) GetRemainingTime() time.Duration {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Until(rl.expiresAt)
}

// GetStatus gets the current status information of the lock (avoids repeated time calculations)
func (rl *RedisLock) GetStatus() (remainingTime time.Duration, isExpired bool) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	now := time.Now()
	remainingTime = rl.expiresAt.Sub(now)
	isExpired = now.After(rl.expiresAt)
	return
}

// IsExpired checks if the lock has expired
func (rl *RedisLock) IsExpired() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	return time.Now().After(rl.expiresAt)
}

// Renew manually renews the lock
func (rl *RedisLock) Renew(ctx context.Context, newExpiration time.Duration) error {
	// Set optional timeout for single call
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	start := time.Now()
	// Execute renewal script (avoid network calls while holding lock)
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
	case 1: // Renewal successful
		now := time.Now()
		rl.mutex.Lock()
		rl.expiration = newExpiration
		rl.expiresAt = now.Add(newExpiration)
		rl.mutex.Unlock()
		globalCallback.OnLockRenewed(rl.key, newExpiration)
		return nil
	case 0, -1, -2: // Lock does not exist or not current holder
		globalCallback.OnLockRenewalFailed(rl.key, ErrLockRenewalFailed)
		return ErrLockRenewalFailed
	default:
		err := fmt.Errorf("unknown renewal result: %d", n)
		globalCallback.OnLockRenewalFailed(rl.key, err)
		return err
	}
}

// Release releases the lock
func (rl *RedisLock) Release(ctx context.Context) error {
	// Set optional timeout for single call
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	start := time.Now()
	// Execute unlock script (unified semantics: partial release does not refresh TTL, pass 0)
	result, err := unlockScript.Run(runCtx, rl.client, []string{rl.ownerKey, rl.countKey}, rl.value, int64(0)).Result()
	if cancel != nil {
		cancel()
	}
	observeScriptLatency("unlock", time.Since(start))
	if err != nil {
		return fmt.Errorf("unlock script execution failed: %w", err)
	}

	// Check if lock was successfully released
	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown unlock result type: %T", result)
	}
	switch n {
	case 2: // Partial release (still held)
		return nil
	case 1: // Fully released lock
		duration := time.Since(rl.acquiredAt)
		globalCallback.OnLockReleased(rl.key, duration)
		return nil
	case 0: // Lock does not exist
		return ErrLockNotHeld
	case -1: // Lock exists but not current holder
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %d", n)
	}
}

// IsLocked checks if the lock is held by the current instance
func (rl *RedisLock) IsLocked(ctx context.Context) (bool, error) {
	// Set optional timeout for single call
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

// Acquire attempts to acquire (or reenter) the lock based on the current RedisLock instance
// If called again on the same instance, the Lua script will treat it as reentrant and renew
// because the value remains unchanged
func (rl *RedisLock) Acquire(ctx context.Context) error {
	// Set optional timeout for single call
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
		// If this is the first acquisition (non-reentrant), generate and record fencing token
		if n == 1 {
			// A single command is sufficient since no other holder can exist while holding the lock
			// Optional timeout
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
	// Occupied by another holder
	globalCallback.OnLockAcquireFailed(rl.key, ErrLockAcquireConflict)
	return ErrLockAcquireConflict
}

// AcquireWithRetry acquires (or reenters) the lock and retries according to strategy
func (rl *RedisLock) AcquireWithRetry(ctx context.Context, strategy RetryStrategy) error {
	retries := 0
	for {
		if strategy.MaxRetries > 0 && retries >= strategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}
		if retries > 0 {
			// Add jitter to avoid hot spot collisions
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
		// Continue retrying according to strategy on conflict
		if strategy.MaxRetries == 0 {
			return ErrLockAcquireConflict
		}
		retries++
	}
}

// EnableAutoRenew registers the current lock to the global renewal manager (starts if not already started)
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
