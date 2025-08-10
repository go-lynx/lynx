package redislock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	lynx "github.com/go-lynx/lynx/plugins/nosql/redis"
)

// Lock 获取指定 key 的分布式锁并执行回调函数，执行完成后自动释放锁。
// - 使用 DefaultLockOptions 作为基础配置，仅覆盖 Expiration。
// - 使用 Lua 脚本原子获取/重入锁，避免竞态。
// - 若启用续期，将在全局管理器中注册并自动续期直到函数执行结束。
func Lock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	// 使用 DefaultLockOptions 作为基础配置，仅覆盖 Expiration
	options := DefaultLockOptions
	options.Expiration = expiration
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithToken 获取指定 key 的分布式锁并执行回调函数，回调可获得 fencing token。
// - 基于 DefaultLockOptions，仅覆盖 Expiration，重试策略使用 DefaultRetryStrategy。
// - token 仅在“首次获取”（非重入）时递增；重入不会产生新 token。
func LockWithToken(ctx context.Context, key string, expiration time.Duration, fn func(token int64) error) error {
	// 基于默认选项构建
	options := DefaultLockOptions
	options.Expiration = expiration

	// 创建锁实例（不主动加锁）
	lock, err := NewLock(ctx, key, options)
	if err != nil {
		return err
	}

	// 尝试按默认策略重试获取
	if err := lock.AcquireWithRetry(ctx, options.RetryStrategy); err != nil {
		return err
	}

	// 若启用续期，纳入全局管理
	if options.RenewalEnabled {
		lock.EnableAutoRenew(options)
	}

	// 确保最终释放
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

	// 执行业务回调，传递 token（若首次获取则为正整数，否则为 0）
	return fn(lock.GetToken())
}

// UnlockByValue 使用 key + value 的方式释放锁（无需持有 RedisLock 实例）。
// 语义说明：
// - 当计数 > 0 时，该操作属于“部分释放”。本实现统一传 TTL=0 给脚本，表示不刷新 TTL（保持剩余过期时间不变）。
// - 当 key 不存在或 value 不匹配，返回 ErrLockNotHeld。
// 超时说明：
// - 单次脚本调用使用 DefaultLockOptions.ScriptCallTimeout 作为可选的 per-call 超时。
func UnlockByValue(ctx context.Context, key, value string) error {
	// 验证锁键名
	if err := ValidateKey(key); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// 获取 Redis 客户端
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}
	// 构建锁键
	ownerKey, countKey := buildLockKeys(key)
	// 执行解锁脚本
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
	// 处理解锁结果
	n, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unknown unlock result type: %T", result)
	}
	switch n {
	case 2:
		// 部分释放（仍持有）
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

// NewLock 创建一个可复用的锁实例（支持同实例可重入）。
// 行为：
// - 不主动触发加锁，只构建 RedisLock 对象；调用者需显式调用 Acquire() 获取或重入锁。
// - 同一实例多次 Acquire 因 value 不变，会被脚本视为可重入并刷新 TTL。
// - Redis Cluster：内部 ownerKey 与 countKey 使用相同 hashtag，确保同槽位以支持 Lua 原子操作。
func NewLock(ctx context.Context, key string, options LockOptions) (*RedisLock, error) {
	// 验证锁键名
	if err := ValidateKey(key); err != nil {
		return nil, newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// 验证配置选项
	if err := options.Validate(); err != nil {
		return nil, newLockError(ErrCodeInvalidOptions, "invalid lock options", err)
	}
	// 获取 Redis 客户端
	client := lynx.GetRedis()
	if client == nil {
		return nil, ErrRedisClientNotFound
	}
	// 构建锁键与 fencing token 键
	ownerKey, countKey := buildLockKeys(key)
	tokenKey := buildTokenKey(key)
	// 生成锁值
	value := generateLockValue()
	// 创建锁实例
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

// LockWithRetry 获取锁并执行函数，支持按策略重试。
// - 基于 DefaultLockOptions，覆盖 Expiration 与 RetryStrategy，其余沿用默认。
// - 重试期间使用随机抖动（0.5~1.5x）降低热点碰撞。
func LockWithRetry(ctx context.Context, key string, expiration time.Duration, fn func() error, strategy RetryStrategy) error {
	// 使用 DefaultLockOptions 作为基础配置，覆盖 Expiration 与 RetryStrategy
	options := DefaultLockOptions
	options.Expiration = expiration
	options.RetryStrategy = strategy
	return LockWithOptions(ctx, key, options, fn)
}

// LockWithOptions 使用完整的配置选项获取锁并执行回调函数。
// 关键行为：
// - 脚本调用均可配置 per-call 超时（options.ScriptCallTimeout）。设置后会在每次调用后立即 cancel。
// - 获取成功后，若启用续期，将把锁注册到全局 manager 并启动续期服务。
// - 函数返回前通过 defer 释放锁。释放和状态检查(IsLocked)均使用短超时上下文，避免阻塞调用方。
// - 统一的部分释放语义：释放脚本传 TTL=0 时不刷新 TTL，仅减少计数。
// 错误：
// - 竞争导致的获取失败将触发回调 OnLockAcquireFailed，并根据重试策略决定是否继续。
func LockWithOptions(ctx context.Context, key string, options LockOptions, fn func() error) error {
	// 验证回调函数
	if fn == nil {
		return ErrLockFnRequired
	}
	// 验证锁键名
	if err := ValidateKey(key); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock key", err)
	}
	// 验证配置选项
	if err := options.Validate(); err != nil {
		return newLockError(ErrCodeInvalidOptions, "invalid lock options", err)
	}
	// 获取 Redis 客户端
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}
	// 生成锁值
	value := generateLockValue()
	// 构建锁键
	ownerKey, countKey := buildLockKeys(key)
	// 创建锁实例
	lock := &RedisLock{
		client:           client,
		key:              key,
		value:            value,
		expiration:       options.Expiration,
		renewalThreshold: options.RenewalThreshold,
		ownerKey:         ownerKey,
		countKey:         countKey,
	}
	// 尝试获取锁
	for retries := 0; ; retries++ {
		// 检查是否超过最大重试次数
		if options.RetryStrategy.MaxRetries > 0 && retries >= options.RetryStrategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}
		// 若不是第一次尝试，按策略等待后再重试（加入抖动以减少同时碰撞）
		if retries > 0 {
			// 加入抖动，避免热点碰撞
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
		// 执行加锁脚本（可选 per-call 超时，Run 后立即 cancel 释放资源）
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
		// 处理加锁结果
		n, ok := result.(int64)
		if !ok {
			return fmt.Errorf("unknown lock result type: %T", result)
		}
		if n > 0 {
			// 使用同一个时间戳，避免重复调用 time.Now()
			now := time.Now()
			lock.expiresAt = now.Add(lock.expiration)
			lock.acquiredAt = now
			incAcquire("success")
			// 触发获取锁回调
			globalCallback.OnLockAcquired(key, lock.expiration)
			// 如果启用续期，添加到全局锁管理器
			if options.RenewalEnabled {
				globalLockManager.mutex.Lock()
				globalLockManager.locks[key] = lock
				atomic.AddInt64(&globalLockManager.stats.TotalLocks, 1)
				atomic.AddInt64(&globalLockManager.stats.ActiveLocks, 1)
				globalLockManager.mutex.Unlock()
				activeLocksInc()
				// 启动续期服务
				globalLockManager.startRenewalService(options)
			}
			// 使用 defer 确保锁会被释放
			var err error
			inManager := options.RenewalEnabled
			defer func() {
				// 使用短超时上下文，避免释放在外层长期 ctx 下阻塞
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
				// 仅在完全释放时，从 manager 中移除（部分释放需继续续期）
				if inManager {
					// 使用短超时上下文进行 IsLocked 查询，避免阻塞
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
						// 保守处理：查询失败则保留在 manager，避免锁意外失去续期
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
			// 执行用户函数
			err = fn()
			return err
		}
		// 触发获取锁失败回调（冲突场景）
		incAcquire("conflict")
		globalCallback.OnLockAcquireFailed(key, ErrLockAcquireConflict)
		// 如果不需要重试，直接返回错误
		if options.RetryStrategy.MaxRetries == 0 {
			return ErrLockAcquireConflict
		}
		// 否则继续重试
		continue
	}
}
