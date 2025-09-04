package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	lynx "github.com/go-lynx/lynx/plugins/nosql/redis"
)

// Lock acquires a distributed lock for the specified key and executes the callback function, automatically releasing the lock after execution.
// - Uses DefaultLockOptions as base configuration, only overriding Expiration.
// - Uses Lua script for atomic lock acquisition/reentrancy, avoiding race conditions.
// - If renewal is enabled, registers in global manager and automatically renews until function execution ends.
func Lock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	// Use DefaultLockOptions as base configuration, only overriding Expiration
	options := DefaultLockOptions
	options.Expiration = expiration
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithToken acquires a distributed lock for the specified key and executes the callback function, callback can obtain fencing token.
// - Based on DefaultLockOptions, only overriding Expiration, retry strategy uses DefaultRetryStrategy.
// - token is only incremented on "first acquisition" (non-reentrant); reentry does not generate a new token.
func LockWithToken(ctx context.Context, key string, expiration time.Duration, fn func(token int64) error) error {
	// Build based on default options
	options := DefaultLockOptions
	options.Expiration = expiration

	// Create lock instance (not actively locking)
	lock, err := NewLock(ctx, key, options)
	if err != nil {
		return err
	}

	// Try to acquire with default retry strategy
	if err := lock.AcquireWithRetry(ctx, options.RetryStrategy); err != nil {
		return err
	}

	// If renewal is enabled, include in global management
	if options.RenewalEnabled {
		lock.EnableAutoRenew(options)
	}

	// Ensure final release
	defer func() {
		rctx := ctx
		var cancel context.CancelFunc
		to := options.ScriptCallTimeout
		if to <= 0 {
			to = DefaultLockOptions.ScriptCallTimeout
		}
		if to > 0 {
			rctx, cancel = context.WithTimeout(ctx, to)
		}
		start := time.Now()
		if releaseErr := lock.Release(rctx); releaseErr != nil {
			log.ErrorCtx(ctx, "failed to release redis lock", "error", releaseErr)
		}
		if cancel != nil {
			cancel()
		}
		observeScriptLatency("unlock", time.Since(start))
	}()

	// Execute business callback, passing token (positive integer if first acquisition, otherwise 0)
	return fn(lock.GetToken())
}

// UnlockByValue releases lock using key + value method (no need to hold RedisLock instance).
// Semantic explanation:
//   - When count > 0, this operation is a "partial release". This implementation uniformly passes TTL=0 to the script,
//     indicating not to refresh TTL (keeping the remaining expiration time unchanged).
//   - When key does not exist or value does not match, returns ErrLockNotHeld.
//
// Timeout explanation:
// - Single script call uses DefaultLockOptions.ScriptCallTimeout as optional per-call timeout.
func UnlockByValue(ctx context.Context, key, value string) error {
	// Validate lock key name
	if err := ValidateKey(key); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// Get Redis client
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}
	// Build lock keys
	ownerKey, countKey := buildLockKeys(key)
	// Execute unlock script
	runCtx := ctx
	var cancel context.CancelFunc
	if to := DefaultLockOptions.ScriptCallTimeout; to > 0 {
		runCtx, cancel = context.WithTimeout(ctx, to)
	}
	result, err := unlockScript.Run(runCtx, client, []string{ownerKey, countKey}, value, int64(0)).Result()
	if cancel != nil {
		cancel()
	}
	if err != nil {
		return fmt.Errorf("unlock script execution failed: %w", err)
	}
	// Handle unlock result
	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown unlock result type: %T", result)
	}
	switch n {
	case 2:
		// Partial release (still held)
		return nil
	case 1:
		return nil
	case 0:
		return ErrLockNotHeld
	case -1:
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %d", n)
	}
}

// NewLock creates a reusable lock instance (supports reentrancy within the same instance).
// Behavior:
// - Does not actively trigger locking, only builds RedisLock object; caller must explicitly call Acquire() to obtain or reenter lock.
// - Multiple Acquire calls on the same instance are treated as reentrant by the script due to unchanged value, and TTL is refreshed.
// - Redis Cluster: internal ownerKey and countKey use the same hashtag to ensure same slot for Lua atomic operations.
func NewLock(ctx context.Context, key string, options LockOptions) (*RedisLock, error) {
	// Validate lock key name
	if err := ValidateKey(key); err != nil {
		return nil, newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// Validate configuration options
	if err := options.Validate(); err != nil {
		return nil, newLockError(ErrCodeInvalidOptions, "invalid lock options", err)
	}
	// Get Redis client
	client := lynx.GetRedis()
	if client == nil {
		return nil, ErrRedisClientNotFound
	}
	// Build lock keys and fencing token key
	ownerKey, countKey := buildLockKeys(key)
	tokenKey := buildTokenKey(key)
	// Generate lock value
	value := generateLockValue()
	// Create lock instance
	lock := &RedisLock{
		client:           client,
		key:              key,
		value:            value,
		expiration:       options.Expiration,
		renewalThreshold: options.RenewalThreshold,
		ownerKey:         ownerKey,
		countKey:         countKey,
		tokenKey:         tokenKey,
	}
	return lock, nil
}

// LockWithRetry acquires lock and executes function, supports retry by strategy.
// - Based on DefaultLockOptions, overrides Expiration and RetryStrategy, others use defaults.
// - Uses random jitter (0.5~1.5x) during retries to reduce hot spot collisions.
func LockWithRetry(ctx context.Context, key string, expiration time.Duration, fn func() error, strategy RetryStrategy) error {
	// Use DefaultLockOptions as base configuration, override Expiration and RetryStrategy
	options := DefaultLockOptions
	options.Expiration = expiration
	options.RetryStrategy = strategy
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithOptions uses complete configuration options to acquire lock and execute callback function.
// Key behaviors:
// - Script calls can configure per-call timeout (options.ScriptCallTimeout). When set, cancel immediately after each call.
// - After successful acquisition, if renewal is enabled, register in global manager and start renewal service.
// - Release lock via defer before function returns. Release and status check (IsLocked) both use short timeout context to avoid blocking caller.
// - Unified partial release semantics: release script passes TTL=0 to not refresh TTL, only reduce count.
// Errors:
// - Acquisition failures due to contention will trigger OnLockAcquireFailed callback and decide whether to continue based on retry strategy.
func LockWithOptions(ctx context.Context, key string, options LockOptions, fn func() error) error {
	// Validate callback function
	if fn == nil {
		return ErrLockFnRequired
	}
	// Validate lock key name
	if err := ValidateKey(key); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// Validate configuration options
	if err := options.Validate(); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock options", err)
	}
	// Get Redis client
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}
	// Generate lock value
	value := generateLockValue()
	// Build lock keys
	ownerKey, countKey := buildLockKeys(key)
	// Create lock instance
	lock := &RedisLock{
		client:           client,
		key:              key,
		value:            value,
		expiration:       options.Expiration,
		renewalThreshold: options.RenewalThreshold,
		ownerKey:         ownerKey,
		countKey:         countKey,
	}
	// Try to acquire lock
	for retries := 0; ; retries++ {
		// Check if maximum retry count is exceeded
		if options.RetryStrategy.MaxRetries > 0 && retries >= options.RetryStrategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}
		// If not the first attempt, wait according to strategy before retrying (add jitter to reduce simultaneous collisions)
		if retries > 0 {
			// Add jitter to avoid hot spot collisions
			delay := options.RetryStrategy.RetryDelay
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
		// Execute lock script (optional per-call timeout, cancel immediately after Run to release resources)
		runCtx := ctx
		var cancel context.CancelFunc
		if to := options.ScriptCallTimeout; to > 0 {
			runCtx, cancel = context.WithTimeout(ctx, to)
		}
		start := time.Now()
		result, err := lockScript.Run(runCtx, lock.client, []string{lock.ownerKey, lock.countKey},
			lock.value, lock.expiration.Milliseconds()).Result()
		if cancel != nil {
			cancel()
		}
		observeScriptLatency("acquire", time.Since(start))
		if err != nil {
			incAcquire("error")
			return fmt.Errorf("lock script execution failed: %w", err)
		}
		// Handle lock result
		n, ok := result.(int64)
		if !ok {
			return fmt.Errorf("unknown lock result type: %T", result)
		}
		if n > 0 {
			// Use the same timestamp to avoid repeated time.Now() calls
			now := time.Now()
			lock.expiresAt = now.Add(lock.expiration)
			lock.acquiredAt = now
			incAcquire("success")
			// Trigger lock acquired callback
			globalCallback.OnLockAcquired(key, lock.expiration)
			// If renewal is enabled, add to global lock manager
			if options.RenewalEnabled {
				globalLockManager.mutex.Lock()
				globalLockManager.locks[key] = lock
				atomic.AddInt64(&globalLockManager.stats.TotalLocks, 1)
				atomic.AddInt64(&globalLockManager.stats.ActiveLocks, 1)
				globalLockManager.mutex.Unlock()
				activeLocksInc()
				// Start renewal service
				globalLockManager.startRenewalService(options)
			}
			// Use defer to ensure lock is released
			var err error
			inManager := options.RenewalEnabled
			defer func() {
				// Use short timeout context to avoid blocking release under long-term outer ctx
				rctx := ctx
				var cancel context.CancelFunc
				to := options.ScriptCallTimeout
				if to <= 0 {
					to = DefaultLockOptions.ScriptCallTimeout
				}
				if to > 0 {
					rctx, cancel = context.WithTimeout(ctx, to)
				}
				start := time.Now()
				if releaseErr := lock.Release(rctx); releaseErr != nil {
					log.ErrorCtx(ctx, "failed to release redis lock", "error", releaseErr)
					if err == nil {
						err = releaseErr
					}
					if cancel != nil {
						cancel()
					}
					return
				}
				if cancel != nil {
					cancel()
				}
				observeScriptLatency("unlock", time.Since(start))
				// Only remove from manager when fully released (partial release needs to continue renewal)
				if inManager {
					// Use short timeout context for IsLocked query to avoid blocking
					cctx := ctx
					var ccancel context.CancelFunc
					if to > 0 {
						cctx, ccancel = context.WithTimeout(ctx, to)
					}
					stillHeld, checkErr := lock.IsLocked(cctx)
					if ccancel != nil {
						ccancel()
					}
					if checkErr != nil {
						// Conservative handling: if query fails, keep in manager to avoid lock losing renewal unexpectedly
						log.ErrorCtx(ctx, "failed to check lock held after release", "error", checkErr)
						return
					}
					if !stillHeld {
						globalLockManager.mutex.Lock()
						if _, exists := globalLockManager.locks[key]; exists {
							delete(globalLockManager.locks, key)
							atomic.AddInt64(&globalLockManager.stats.ActiveLocks, -1)
						}
						globalLockManager.mutex.Unlock()
						activeLocksDec()
					}
				}
			}()
			// Execute user function
			err = fn()
			return err
		}
		// Trigger lock acquire failed callback (conflict scenario)
		incAcquire("conflict")
		globalCallback.OnLockAcquireFailed(key, ErrLockAcquireConflict)
		// If no retry needed, return error directly
		if options.RetryStrategy.MaxRetries == 0 {
			return ErrLockAcquireConflict
		}
		// Otherwise continue retrying
		continue
	}
}
