package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// 全局锁管理器
// 说明：
// - 续期服务采用工作池限制并发，避免续期风暴；
// - 续期调用支持 per-call 超时（options.RenewalConfig.CallTimeout）；
// - 统计数据通过原子变量维护，避免高频加锁；
// - 所有 Script.Run 调用后均立即 cancel 对应 context，防止资源泄漏。
var globalLockManager = &lockManager{
	locks: make(map[string]*RedisLock),
}

// 全局回调实例
var globalCallback LockCallback = NoOpCallback{}

// SetCallback 设置全局回调
func SetCallback(callback LockCallback) {
	if callback == nil {
		callback = NoOpCallback{}
	}
	globalCallback = callback
}

// startRenewalService 启动续期服务（改进版）
func (lm *lockManager) startRenewalService(options LockOptions) {
	lm.mutex.Lock()
	if lm.running {
		lm.mutex.Unlock()
		return
	}
	lm.renewCtx, lm.renewCancel = context.WithCancel(context.Background())
	lm.running = true
	// 初始化工作池，限制并发 goroutine 数量，防止因集中续期导致的资源争抢
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

// stopRenewalService 停止续期服务
func (lm *lockManager) stopRenewalService() {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if !lm.running {
		return
	}

	lm.renewCancel()
	lm.running = false
	// 不关闭 workerPool，避免其他 goroutine 发送时发生 panic。
}

// processRenewals 处理锁续期（使用工作池模式）
func (lm *lockManager) processRenewals(options LockOptions) {
	lm.mutex.RLock()

	// 预分配切片容量，减少内存分配
	locksToRenew := make([]*RedisLock, 0, len(lm.locks))

	for _, lock := range lm.locks {
		// 读取锁的快照（expiresAt/expiration/threshold），避免数据竞争
		lock.mutex.Lock()
		expiresAtSnap := lock.expiresAt
		expirationSnap := lock.expiration
		thresholdSnap := lock.renewalThreshold
		lock.mutex.Unlock()

		// 只处理需要续期的锁（基于快照判断），阈值=expiration*threshold
		thresholdDur := time.Duration(float64(expirationSnap) * thresholdSnap)
		if time.Until(expiresAtSnap) <= thresholdDur {
			locksToRenew = append(locksToRenew, lock)
		}
	}
	lm.mutex.RUnlock()

	// 使用工作池限制并发数；当池满时跳过当前轮次（累计 SkippedRenewals）
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
			// 工作池已满，本轮跳过，避免阻塞主循环
			atomic.AddInt64(&lm.stats.SkippedRenewals, 1)
			incSkippedRenewal()
		}
	}
}

// renewLockWithRetry 带重试的锁续期
func (lm *lockManager) renewLockWithRetry(lock *RedisLock, options LockOptions) {
	config := options.RenewalConfig
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultRenewalConfig.MaxRetries
	}

	for i := 0; i < maxRetries; i++ {
		// 为单次续期调用设置超时（如配置），Run 后立即 cancel 释放资源
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

		// 指数退避重试 + 抖动（50%~150%），降低集中竞争
		if i < maxRetries-1 {
			delay := config.BaseDelay * time.Duration(1<<i)
			// 加入抖动 50%~150%
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

	// 重试失败：从管理器中移除锁并减少活动计数，避免无效续期
	lm.mutex.Lock()
	delete(lm.locks, lock.key)
	atomic.AddInt64(&lm.stats.ActiveLocks, -1)
	lm.mutex.Unlock()

	log.ErrorCtx(context.Background(), "lock renewal failed after retries",
		"key", lock.key, "retries", maxRetries)
}

// renewLock 续期单个锁（改进版）
func (lm *lockManager) renewLock(ctx context.Context, lock *RedisLock) error {
	// 读取快照，避免并发读写冲突
	lock.mutex.Lock()
	expiresAtSnap := lock.expiresAt
	expirationSnap := lock.expiration
	thresholdSnap := lock.renewalThreshold
	lock.mutex.Unlock()

	// 检查是否需要续期（基于快照）
	if time.Until(expiresAtSnap) > time.Duration(float64(expirationSnap)*thresholdSnap) {
		return nil
	}

	// 执行续期脚本（使用可取消上下文）
	start := time.Now()
	result, err := renewScript.Run(ctx, lock.client, []string{lock.ownerKey, lock.countKey},
		lock.value, expirationSnap.Milliseconds()).Result()
	// 记录续期时延
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
	case 1: // 续期成功
		lock.mutex.Lock()
		lock.expiresAt = time.Now().Add(lock.expiration)
		lock.mutex.Unlock()
		incRenew("success")
		return nil
	case 0, -1, -2: // 锁不存在或不是当前持有者
		lm.mutex.Lock()
		delete(lm.locks, lock.key)
		atomic.AddInt64(&lm.stats.ActiveLocks, -1)
		lm.mutex.Unlock()
		activeLocksDec()
		// 更细致的区分
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

// GetStats 获取锁管理器统计信息
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
	// 追加当前工作池队列使用量（读取 len/cap 为原子快照，不需加锁）
	// worker_queue_len 表示当前占用的令牌数；worker_queue_cap 表示最大并发能力。
	if globalLockManager.workerPool != nil {
		m["worker_queue_len"] = int64(len(globalLockManager.workerPool))
		m["worker_queue_cap"] = int64(cap(globalLockManager.workerPool))
	}
	return m
}

// Shutdown 优雅关闭锁管理器
func Shutdown(ctx context.Context) error {
	globalLockManager.stopRenewalService()

	// 等待所有锁释放或超时
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
