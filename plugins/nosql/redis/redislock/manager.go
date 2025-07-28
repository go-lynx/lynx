package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// 全局锁管理器
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
func (lm *lockManager) startRenewalService() {
	lm.mutex.Lock()
	if lm.running {
		lm.mutex.Unlock()
		return
	}
	lm.renewCtx, lm.renewCancel = context.WithCancel(context.Background())
	lm.running = true
	// 初始化工作池，限制并发goroutine数量
	lm.workerPool = make(chan struct{}, 20) // 最多20个并发续期
	lm.mutex.Unlock()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond) // 提高检查频率
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.processRenewals()
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
	close(lm.workerPool)
}

// processRenewals 处理锁续期（使用工作池模式）
func (lm *lockManager) processRenewals() {
	lm.mutex.RLock()
	locksToRenew := make([]*RedisLock, 0)

	for _, lock := range lm.locks {
		// 只处理需要续期的锁
		if time.Until(lock.expiresAt) <= lock.expiration*time.Duration(lock.renewalThreshold) {
			locksToRenew = append(locksToRenew, lock)
		}
	}
	lm.mutex.RUnlock()

	// 使用工作池限制并发数
	for _, lock := range locksToRenew {
		select {
		case lm.workerPool <- struct{}{}:
			go func(l *RedisLock) {
				defer func() { <-lm.workerPool }()
				lm.renewLockWithRetry(l)
			}(lock)
		default:
			// 工作池已满，直接在当前goroutine中处理
			lm.renewLockWithRetry(lock)
		}
	}
}

// renewLockWithRetry 带重试的锁续期
func (lm *lockManager) renewLockWithRetry(lock *RedisLock) {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if err := lm.renewLock(lock); err == nil {
			atomic.AddInt64(&lm.stats.RenewalCount, 1)
			return
		}

		atomic.AddInt64(&lm.stats.RenewalErrors, 1)

		// 指数退避重试
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
		}
	}

	// 重试失败，从管理器中移除锁
	lm.mutex.Lock()
	delete(lm.locks, lock.key)
	atomic.AddInt64(&lm.stats.ActiveLocks, -1)
	lm.mutex.Unlock()

	log.Error(context.Background(), "lock renewal failed after retries",
		"key", lock.key, "retries", maxRetries)
}

// renewLock 续期单个锁（改进版）
func (lm *lockManager) renewLock(lock *RedisLock) error {
	// 检查是否需要续期
	if time.Until(lock.expiresAt) > lock.expiration*time.Duration(lock.renewalThreshold) {
		return nil
	}

	// 执行续期脚本
	result, err := lock.client.Eval(context.Background(), renewScript, []string{lock.key},
		lock.value, lock.expiration.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("renewal script execution failed: %w", err)
	}

	switch result.(int64) {
	case 1: // 续期成功
		lock.mutex.Lock()
		lock.expiresAt = time.Now().Add(lock.expiration)
		lock.mutex.Unlock()
		return nil
	case 0, -1, -2: // 锁不存在或不是当前持有者
		lm.mutex.Lock()
		delete(lm.locks, lock.key)
		atomic.AddInt64(&lm.stats.ActiveLocks, -1)
		lm.mutex.Unlock()
		return ErrLockRenewalFailed
	default:
		return fmt.Errorf("unknown renewal result: %v", result)
	}
}

// GetStats 获取锁管理器统计信息
func GetStats() map[string]int64 {
	globalLockManager.mutex.RLock()
	defer globalLockManager.mutex.RUnlock()

	return map[string]int64{
		"total_locks":    atomic.LoadInt64(&globalLockManager.stats.TotalLocks),
		"active_locks":   atomic.LoadInt64(&globalLockManager.stats.ActiveLocks),
		"renewal_count":  atomic.LoadInt64(&globalLockManager.stats.RenewalCount),
		"renewal_errors": atomic.LoadInt64(&globalLockManager.stats.RenewalErrors),
	}
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
