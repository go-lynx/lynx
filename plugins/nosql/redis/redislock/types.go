package redislock

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// LockOptions 锁的配置选项
type LockOptions struct {
	Expiration       time.Duration // 锁过期时间
	RetryStrategy    RetryStrategy // 重试策略
	RenewalEnabled   bool          // 是否启用自动续期
	RenewalThreshold float64       // 续期阈值（相对于过期时间的比例，默认1/3）
}

// RetryStrategy 定义锁重试策略
type RetryStrategy struct {
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试间隔
}

// LockCallback 锁操作回调接口
type LockCallback interface {
	OnLockAcquired(key string, duration time.Duration)
	OnLockReleased(key string, duration time.Duration)
	OnLockRenewed(key string, duration time.Duration)
	OnLockRenewalFailed(key string, error error)
	OnLockAcquireFailed(key string, error error)
}

// NoOpCallback 空实现回调
type NoOpCallback struct{}

func (NoOpCallback) OnLockAcquired(key string, duration time.Duration) {}
func (NoOpCallback) OnLockReleased(key string, duration time.Duration) {}
func (NoOpCallback) OnLockRenewed(key string, duration time.Duration)  {}
func (NoOpCallback) OnLockRenewalFailed(key string, error error)       {}
func (NoOpCallback) OnLockAcquireFailed(key string, error error)       {}

// RedisLock 实现了基于 Redis 的分布式锁
type RedisLock struct {
	client           *redis.Client // Redis 客户端
	key              string        // 锁的键名
	value            string        // 锁的值（用于识别持有者）
	expiration       time.Duration // 锁的过期时间
	expiresAt        time.Time     // 锁的过期时间点
	mutex            sync.Mutex    // 保护内部状态
	renewalThreshold float64       // 续期阈值
	acquiredAt       time.Time     // 获取锁的时间
}

// lockManager 管理所有的分布式锁实例
type lockManager struct {
	mutex sync.RWMutex
	locks map[string]*RedisLock
	// 续期服务
	renewCtx    context.Context
	renewCancel context.CancelFunc
	running     bool
	// 工作池
	workerPool chan struct{}
	// 统计信息
	stats struct {
		TotalLocks    int64
		ActiveLocks   int64
		RenewalCount  int64
		RenewalErrors int64
	}
}

// 默认配置
var (
	DefaultLockOptions = LockOptions{
		Expiration:       30 * time.Second,
		RetryStrategy:    DefaultRetryStrategy,
		RenewalEnabled:   true,
		RenewalThreshold: 1.0 / 3.0,
	}

	DefaultRetryStrategy = RetryStrategy{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}
)
