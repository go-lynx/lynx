package redislock

import (
	"context"
	"fmt"
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
	WorkerPoolSize   int           // 续期工作池大小，默认20
	RenewalConfig    RenewalConfig // 续期配置
}

// Validate 验证配置选项
func (lo *LockOptions) Validate() error {
	if lo.Expiration <= 0 {
		return fmt.Errorf("expiration must be positive, got %v", lo.Expiration)
	}

	if lo.RenewalThreshold < 0 || lo.RenewalThreshold > 1 {
		return fmt.Errorf("renewal threshold must be between 0 and 1, got %f", lo.RenewalThreshold)
	}

	if lo.WorkerPoolSize < 0 {
		return fmt.Errorf("worker pool size must be non-negative, got %d", lo.WorkerPoolSize)
	}

	return lo.RetryStrategy.Validate()
}

// ValidateKey 验证锁键名的有效性
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("lock key cannot be empty")
	}

	if len(key) > 255 {
		return fmt.Errorf("lock key too long, max length is 255, got %d", len(key))
	}

	// 检查是否包含非法字符
	for _, char := range key {
		if char < 32 || char > 126 {
			return fmt.Errorf("lock key contains invalid character: %c", char)
		}
	}

	return nil
}

// Validate 验证重试策略
func (rs *RetryStrategy) Validate() error {
	if rs.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", rs.MaxRetries)
	}

	if rs.RetryDelay < 0 {
		return fmt.Errorf("retry delay must be non-negative, got %v", rs.RetryDelay)
	}

	return nil
}

// RetryStrategy 定义锁重试策略
type RetryStrategy struct {
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试间隔
}

// RenewalConfig 续期配置
type RenewalConfig struct {
	MaxRetries    int           // 续期最大重试次数
	BaseDelay     time.Duration // 基础重试延迟
	MaxDelay      time.Duration // 最大重试延迟
	CheckInterval time.Duration // 续期检查间隔
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
	DefaultRetryStrategy = RetryStrategy{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	DefaultRenewalConfig = RenewalConfig{
		MaxRetries:    3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		CheckInterval: 500 * time.Millisecond,
	}

	DefaultLockOptions LockOptions
)

// init 初始化默认配置，避免循环依赖
func init() {
	DefaultLockOptions = LockOptions{
		Expiration:       30 * time.Second,
		RetryStrategy:    DefaultRetryStrategy,
		RenewalEnabled:   true,
		RenewalThreshold: 1.0 / 3.0,
		WorkerPoolSize:   20,
		RenewalConfig:    DefaultRenewalConfig,
	}
}
