package redislock

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
)

// 用于复用的切片对象池
var batchPool = sync.Pool{
	New: func() interface{} {
		return make([]*RedisLock, 0, DefaultConfig.BatchSize)
	},
}

// renewalTask 续期任务
type renewalTask struct {
	lock     *RedisLock
	expireAt time.Time
}

// renewalQueue 续期任务队列
type renewalQueue []*renewalTask

func (q renewalQueue) Len() int            { return len(q) }
func (q renewalQueue) Less(i, j int) bool  { return q[i].expireAt.Before(q[j].expireAt) }
func (q renewalQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *renewalQueue) Push(x interface{}) { *q = append(*q, x.(*renewalTask)) }
func (q *renewalQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[0 : n-1]
	return item
}

// lockManager 管理所有的分布式锁实例
type lockManager struct {
	locks       *sync.Map
	renewCtx    context.Context
	renewCancel context.CancelFunc
	workerPool  *ants.Pool
	initOnce    sync.Once     // 使用 sync.Once 保证只初始化一次
	closeOnce   sync.Once     // 使用 sync.Once 保证只关闭一次
	queue       renewalQueue  // 续期任务队列
	queueLock   sync.RWMutex  // 保护队列的锁

	// 监控指标
	renewSuccess    atomic.Int64 // 续期成功次数
	renewFailure    atomic.Int64 // 续期失败次数
	renewLatency    atomic.Int64 // 续期延迟（统计平均值）
	renewInProgress atomic.Int32 // 正在进行的续期数

	// 配置
	config Config
}

// 全局锁管理器
var globalLockManager = &lockManager{
	locks: new(sync.Map),
	workerPool: func() *ants.Pool {
		pool, err := ants.NewPool(DefaultConfig.WorkerPoolSize,
			ants.WithExpiryDuration(time.Minute), // 协程池中的协程过期时间
			ants.WithPreAlloc(true),              // 预分配协程池内存
		)
		if err != nil {
			panic(fmt.Sprintf("failed to create worker pool: %v", err))
		}
		return pool
	}(),
	config: DefaultConfig,
}

// addRenewalTask 添加续期任务
func (lm *lockManager) addRenewalTask(lock *RedisLock) {
	expiresAt := time.Unix(0, lock.expiresAt.Load())
	// 计算下次续期时间，提前配置的比例
	renewAt := expiresAt.Add(-time.Duration(float64(lock.expiration) * lm.config.RenewalAheadRatio))
	if time.Until(renewAt) < lm.config.MinRenewalThreshold*2 {
		renewAt = time.Now().Add(lm.config.MinRenewalThreshold)
	}

	lm.queueLock.Lock()
	heap.Push(&lm.queue, &renewalTask{lock: lock, expireAt: renewAt})
	lm.queueLock.Unlock()
}

// startRenewalService 启动续期服务
func (lm *lockManager) startRenewalService() {
	lm.initOnce.Do(func() {
		lm.renewCtx, lm.renewCancel = context.WithCancel(context.Background())

		go func() {
			const batchSize = 100 // 批量处理大小

			for {
				select {
				case <-lm.renewCtx.Done():
					return
				default:
					lm.queueLock.RLock()
					if lm.queue.Len() == 0 {
						lm.queueLock.RUnlock()
						time.Sleep(lm.config.MinRenewalThreshold / 2) // 队列为空时等待
						continue
					}

					// 查看队列顶部任务
					task := lm.queue[0]
					waitDuration := time.Until(task.expireAt)

					if waitDuration > 0 {
						lm.queueLock.RUnlock()
						// 如果还不需要续期，等待一段时间
						time.Sleep(min(waitDuration, lm.config.MinRenewalThreshold/2))
						continue
					}
					lm.queueLock.RUnlock()
					lm.queueLock.Lock()

					// 收集需要续期的锁
					batch := make([]*RedisLock, 0, batchSize)
					for lm.queue.Len() > 0 && len(batch) < batchSize {
						task := heap.Pop(&lm.queue).(*renewalTask)
						if time.Until(task.expireAt) <= 0 {
							batch = append(batch, task.lock)
						} else {
							// 如果还不需要续期，放回队列
							heap.Push(&lm.queue, task)
							break
						}
					}
					lm.queueLock.Unlock()

					// 批量提交续期任务
					_ = lm.workerPool.Submit(func() {
						lm.renewBatch(batch)
					})
				}
			}
		}()
	})
}

// min 返回两个 duration 中的较小值
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// Close 关闭锁管理器
func (lm *lockManager) Close() error {
	var err error
	lm.closeOnce.Do(func() {
		// 取消续期服务
		if lm.renewCancel != nil {
			lm.renewCancel()
		}

		// 释放所有锁
		lm.locks.Range(func(key, value interface{}) bool {
			if lock, ok := value.(*RedisLock); ok {
				if unlockErr := Unlock(context.Background(), lock.key); unlockErr != nil {
					log.Error(context.Background(), "failed to unlock during close", "error", unlockErr)
					if err == nil {
						err = unlockErr
					}
				}
			}
			return true
		})

		// 关闭协程池
		lm.workerPool.Release()
	})
	return err
}

// renewBatch 批量续期锁
func (lm *lockManager) renewBatch(locks []*RedisLock) {
	if len(locks) == 0 {
		return
	}

	// 单个锁直接使用 renewLock
	if len(locks) == 1 {
		if lm.renewLock(locks[0]) {
			lm.addRenewalTask(locks[0])
		}
		return
	}

	ctx := context.Background()
	start := time.Now()

	// 使用 pipeline 批量续期
	pipe := locks[0].client.Pipeline()
	for _, lock := range locks {
		// 检查是否需要续期
		if !lm.shouldRenew(lock) {
			continue
		}
		pipe.Eval(ctx, renewScript, []string{lock.key},
			lock.value, lock.expiration.Milliseconds())
	}

	// 执行 pipeline
	results, err := pipe.Exec(ctx)
	if err != nil {
		log.Error(ctx, "failed to execute pipeline", "error", err)
		lm.renewFailure.Add(int64(len(locks)))
		return
	}

	// 获取服务器时间
	var serverTime time.Time
	if st, err := locks[0].client.Time(ctx).Result(); err == nil {
		serverTime = st
	} else {
		serverTime = time.Now()
	}

	// 处理结果
	for i, result := range results {
		if result.Err() != nil {
			log.Error(ctx, "failed to renew lock", "key", locks[i].key, "error", result.Err())
			lm.renewFailure.Add(1)
			lm.locks.Delete(locks[i].key)
			continue
		}

		// 获取结果值
		cmdResult := result.(*redis.Cmd)
		val, err := cmdResult.Int64()
		if err != nil {
			log.Error(ctx, "failed to get result value", "error", err)
			lm.renewFailure.Add(1)
			continue
		}

		switch val {
		case 1: // 续期成功
			locks[i].expiresAt.Store(serverTime.Add(locks[i].expiration).UnixNano())
			lm.renewSuccess.Add(1)
			// 重新加入队列
			lm.addRenewalTask(locks[i])

		case 0, -1, -2: // 锁不存在或不是当前持有者
			lm.locks.Delete(locks[i].key)
			lm.renewFailure.Add(1)

		default:
			log.Error(ctx, "unknown renewal result", "result", val)
			lm.renewFailure.Add(1)
		}
	}

	// 更新延迟指标
	elapsed := time.Since(start)
	currentAvg := time.Duration(lm.renewLatency.Load())
	newAvg := currentAvg + (elapsed-currentAvg)/10 // 移动平均系数为 0.1
	lm.renewLatency.Store(int64(newAvg))
}

// shouldRenew 检查是否需要续期
func (lm *lockManager) shouldRenew(lock *RedisLock) bool {
	expiresAt := time.Unix(0, lock.expiresAt.Load())
	timeUntilExpiry := time.Until(expiresAt)

	// 考虑时钟偏移，使用更保守的续期策略
	return timeUntilExpiry <= time.Duration(float64(lock.expiration)*lm.config.RenewalAheadRatio) || 
		timeUntilExpiry <= lm.config.MinRenewalThreshold*2
}

// renewLock 续期单个锁
func (lm *lockManager) renewLock(lock *RedisLock) bool {
	ctx := context.Background()

	// 检查是否需要续期
	if !lm.shouldRenew(lock) {
		return true // 不需要续期也算成功
	}

	// 增加正在进行的续期计数
	lm.renewInProgress.Add(1)
	defer lm.renewInProgress.Add(-1)

	// 记录开始时间
	start := time.Now()

	// 使用指数退避重试
	backoff := lm.config.RetryBackoff
	for i := 0; i < lm.config.MaxRetries; i++ {
		// 执行续期脚本
		result, err := lock.client.Eval(ctx, renewScript, []string{lock.key},
			lock.value, lock.expiration.Milliseconds()).Result()
		if err != nil {
			if i == lm.config.MaxRetries-1 { // 最后一次重试也失败
				lm.renewFailure.Add(1)
				log.Error(ctx, "failed to renew lock after retries", "error", err)
				return false
			}
			time.Sleep(backoff)
			backoff *= 2 // 指数退避
			continue
		}

		// 检查结果类型
		val, ok := result.(int64)
		if !ok {
			lm.renewFailure.Add(1)
			log.Error(ctx, "unexpected result type", "type", fmt.Sprintf("%T", result))
			return false
		}

		// 处理结果
		switch val {
		case 1: // 续期成功
			// 考虑时钟偏移，使用服务器时间
			serverTime, err := lock.client.Time(ctx).Result()
			if err == nil {
				lock.expiresAt.Store(serverTime.Add(lock.expiration).UnixNano())
			} else {
				lock.expiresAt.Store(time.Now().Add(lock.expiration).UnixNano())
			}

			// 更新指标
			lm.renewSuccess.Add(1)
			elapsed := time.Since(start)
			// 使用移动平均更新延迟
			currentAvg := time.Duration(lm.renewLatency.Load())
			newAvg := currentAvg + (elapsed-currentAvg)/10 // 移动平均系数为 0.1
			lm.renewLatency.Store(int64(newAvg))
			return true

		case 0, -1, -2: // 锁不存在或不是当前持有者
			lm.locks.Delete(lock.key)
			lm.renewFailure.Add(1)
			return false

		default:
			lm.renewFailure.Add(1)
			log.Error(ctx, "unknown renewal result", "result", val)
			return false
		}
	}
	return false
}
