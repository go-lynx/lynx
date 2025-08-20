package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// Global lock manager
// Description:
// - Renewal service uses worker pool to limit concurrency and avoid renewal storms;
// - Renewal calls support per-call timeout (options.RenewalConfig.CallTimeout);
// - Statistics are maintained through atomic variables to avoid high-frequency locking;
// - All Script.Run calls immediately cancel the corresponding context to prevent resource leaks.
var globalLockManager = &lockManager{
	locks: make(map[string]*RedisLock),
}

// Global callback instance
var globalCallback LockCallback = NoOpCallback{}

// SetCallback sets the global callback
func SetCallback(callback LockCallback) {
	if callback == nil {
		callback = NoOpCallback{}
	}
	globalCallback = callback
}

// startRenewalService starts the renewal service (improved version)
func (lm *lockManager) startRenewalService(options LockOptions) {
	lm.mutex.Lock()
	if lm.running {
		lm.mutex.Unlock()
		return
	}
	lm.renewCtx, lm.renewCancel = context.WithCancel(context.Background())
	lm.running = true
	// Initialize worker pool to limit concurrent goroutine count, preventing resource contention due to concentrated renewal
	workerPoolSize := options.WorkerPoolSize
	if workerPoolSize <= 0 {
		workerPoolSize = DefaultLockOptions.WorkerPoolSize
	}
	lm.workerPool = make(chan struct{}, workerPoolSize)
	lm.mutex.Unlock()

	go func() {
		checkInterval := options.RenewalConfig.CheckInterval
		if checkInterval <= 0 {
			checkInterval = DefaultRenewalConfig.CheckInterval
		}
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.processRenewals(options)
			case <-lm.renewCtx.Done():
				return
			}
		}
	}()
}

// stopRenewalService stops the renewal service
func (lm *lockManager) stopRenewalService() {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if !lm.running {
		return
	}

	lm.renewCancel()
	lm.running = false
	// Do not close workerPool to avoid panic when other goroutines send to it.
}

// processRenewals processes lock renewals (using worker pool pattern)
func (lm *lockManager) processRenewals(options LockOptions) {
	lm.mutex.RLock()

	// Pre-allocate slice capacity to reduce memory allocation
	locksToRenew := make([]*RedisLock, 0, len(lm.locks))

	for _, lock := range lm.locks {
		// Read lock snapshots (expiresAt/expiration/threshold) to avoid data race
		lock.mutex.Lock()
		expiresAtSnap := lock.expiresAt
		expirationSnap := lock.expiration
		thresholdSnap := lock.renewalThreshold
		lock.mutex.Unlock()

		// Only process locks that need renewal (based on snapshot), threshold = expiration * threshold
		thresholdDur := time.Duration(float64(expirationSnap) * thresholdSnap)
		if time.Until(expiresAtSnap) <= thresholdDur {
			locksToRenew = append(locksToRenew, lock)
		}
	}
	lm.mutex.RUnlock()

	// Use worker pool to limit concurrency; skip current round when pool is full (accumulate SkippedRenewals)
	for _, lock := range locksToRenew {
		select {
		case <-lm.renewCtx.Done():
			return
		case lm.workerPool <- struct{}{}:
			go func(l *RedisLock) {
				defer func() { <-lm.workerPool }()
				lm.renewLockWithRetry(l, options)
			}(lock)
		default:
			// Worker pool is full, skip this round to avoid blocking main loop
			atomic.AddInt64(&lm.stats.SkippedRenewals, 1)
			incSkippedRenewal()
		}
	}
}

// renewLockWithRetry lock renewal with retry
func (lm *lockManager) renewLockWithRetry(lock *RedisLock, options LockOptions) {
	config := options.RenewalConfig
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultRenewalConfig.MaxRetries
	}

	for i := 0; i < maxRetries; i++ {
		// Set timeout for single renewal call (if configured), cancel immediately after Run to release resources
		ctx := lm.renewCtx
		var cancel context.CancelFunc
		if to := config.CallTimeout; to > 0 {
			ctx, cancel = context.WithTimeout(ctx, to)
		}

		err := lm.renewLock(ctx, lock)
		if cancel != nil {
			cancel()
		}
		if err == nil {
			atomic.AddInt64(&lm.stats.RenewalCount, 1)
			return
		}

		atomic.AddInt64(&lm.stats.RenewalErrors, 1)

		// Exponential backoff retry + jitter (50%~150%) to reduce concentrated competition
		if i < maxRetries-1 {
			delay := config.BaseDelay * time.Duration(1<<i)
			// Add jitter 50%~150%
			if delay > 0 {
				jitter := time.Duration(float64(delay) * (0.5 + randFloat64()))
				if jitter > 0 {
					delay = jitter
				}
			}
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			time.Sleep(delay)
		}
	}

	// Retry failed: remove lock from manager and decrease active count to avoid invalid renewal
	lm.mutex.Lock()
	delete(lm.locks, lock.key)
	atomic.AddInt64(&lm.stats.ActiveLocks, -1)
	lm.mutex.Unlock()

	log.ErrorCtx(context.Background(), "lock renewal failed after retries",
		"key", lock.key, "retries", maxRetries)
}

// renewLock renew a single lock (improved version)
func (lm *lockManager) renewLock(ctx context.Context, lock *RedisLock) error {
	// Read snapshot to avoid concurrent read-write conflicts
	lock.mutex.Lock()
	expiresAtSnap := lock.expiresAt
	expirationSnap := lock.expiration
	thresholdSnap := lock.renewalThreshold
	lock.mutex.Unlock()

	// Check if renewal is needed (based on snapshot)
	if time.Until(expiresAtSnap) > time.Duration(float64(expirationSnap)*thresholdSnap) {
		return nil
	}

	// Execute renewal script (using cancellable context)
	start := time.Now()
	result, err := renewScript.Run(ctx, lock.client, []string{lock.ownerKey, lock.countKey},
		lock.value, expirationSnap.Milliseconds()).Result()
	// Record renewal latency
	latency := time.Since(start)
	atomic.AddInt64(&lm.stats.RenewLatencyNs, latency.Nanoseconds())
	atomic.AddInt64(&lm.stats.RenewLatencyCount, 1)
	observeScriptLatency("renew", latency)
	if err != nil {
		incRenew("error")
		return fmt.Errorf("renewal script execution failed: %w", err)
	}

	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown renewal result type: %T", result)
	}
	switch n {
	case 1: // Renewal successful
		lock.mutex.Lock()
		lock.expiresAt = time.Now().Add(lock.expiration)
		lock.mutex.Unlock()
		incRenew("success")
		return nil
	case 0, -1, -2: // Lock does not exist or not current holder
		lm.mutex.Lock()
		delete(lm.locks, lock.key)
		atomic.AddInt64(&lm.stats.ActiveLocks, -1)
		lm.mutex.Unlock()
		activeLocksDec()
		// More detailed distinction
		switch n {
		case 0:
			incRenew("not_exist")
		case -1:
			incRenew("not_owner")
		case -2:
			incRenew("fail")
		}
		return ErrLockRenewalFailed
	default:
		return fmt.Errorf("unknown renewal result: %v", result)
	}
}

// GetStats gets lock manager statistics
func GetStats() map[string]int64 {
	m := map[string]int64{
		"total_locks":         atomic.LoadInt64(&globalLockManager.stats.TotalLocks),
		"active_locks":        atomic.LoadInt64(&globalLockManager.stats.ActiveLocks),
		"renewal_count":       atomic.LoadInt64(&globalLockManager.stats.RenewalCount),
		"renewal_errors":      atomic.LoadInt64(&globalLockManager.stats.RenewalErrors),
		"skipped_renewals":    atomic.LoadInt64(&globalLockManager.stats.SkippedRenewals),
		"renew_latency_ns":    atomic.LoadInt64(&globalLockManager.stats.RenewLatencyNs),
		"renew_latency_count": atomic.LoadInt64(&globalLockManager.stats.RenewLatencyCount),
	}
	// Append current worker pool queue usage (reading len/cap as atomic snapshot, no locking required)
	// worker_queue_len represents currently occupied tokens; worker_queue_cap represents maximum concurrency capability.
	if globalLockManager.workerPool != nil {
		m["worker_queue_len"] = int64(len(globalLockManager.workerPool))
		m["worker_queue_cap"] = int64(cap(globalLockManager.workerPool))
	}
	return m
}

// Shutdown gracefully shuts down the lock manager
func Shutdown(ctx context.Context) error {
	globalLockManager.stopRenewalService()

	// Wait for all locks to be released or timeout
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("shutdown timeout, %d locks still active",
				atomic.LoadInt64(&globalLockManager.stats.ActiveLocks))
		case <-ticker.C:
			if atomic.LoadInt64(&globalLockManager.stats.ActiveLocks) == 0 {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
