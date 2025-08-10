package redislock

import (
	"context"
	"fmt"
	"math/rand"
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
	WorkerPoolSize   int           // 续期工作池大小，默认50
	RenewalConfig    RenewalConfig // 续期配置
	// ScriptCallTimeout 单次脚本调用的超时控制（获取/释放）。为0则不单独设置超时。
	ScriptCallTimeout time.Duration
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
		if char == '{' || char == '}' {
			return fmt.Errorf("lock key cannot contain '{' or '}' to protect Redis Cluster hashtag semantics: %q", key)
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
	// CallTimeout 单次续期脚本调用超时。为0则不单独设置超时。
	CallTimeout time.Duration
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

	// 实际在 Redis 中使用的两个键（使用相同 hash tag，保证在集群中同槽位）
	ownerKey string // 保存持有者标识的键
	countKey string // 保存重入计数的键
	// fencing token 键与最近一次获取到的 token 值
	// 注意：token 仅在第一次获取（非重入）时递增并记录。
	tokenKey string
	token    int64
}

// rng provides package-local randomness; guard with mutex for concurrency safety.
var (
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMu sync.Mutex
)

// randFloat64 returns a random float64 in [0.0, 1.0) using the local RNG.
func randFloat64() float64 {
	rngMu.Lock()
	v := rng.Float64()
	rngMu.Unlock()
	return v
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
		// 当工作池满载时被跳过的续期次数
		SkippedRenewals int64
		// 续期时延累计（纳秒）与次数，用于计算平均时延
		RenewLatencyNs    int64
		RenewLatencyCount int64
		WorkerPoolCap     int
	}
}

// 默认配置
var (
	DefaultRetryStrategy = RetryStrategy{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	DefaultRenewalConfig = RenewalConfig{
		// 更保守的续期策略，覆盖短时 Redis 延迟尖刺
		MaxRetries:    4,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      800 * time.Millisecond,
		CheckInterval: 300 * time.Millisecond,
		CallTimeout:   600 * time.Millisecond,
	}

	DefaultLockOptions LockOptions
)

// init 初始化默认配置，避免循环依赖
func init() {
	DefaultLockOptions = LockOptions{
		Expiration:     30 * time.Second,
		RetryStrategy:  DefaultRetryStrategy,
		RenewalEnabled: true,
		// 提前在 30% TTL 进入续期窗口，给重试与抖动留足余量
		RenewalThreshold: 0.3,
		// 提升默认工作池容量，降低跳过概率（视业务压测可再调）
		WorkerPoolSize: 50,
		RenewalConfig:  DefaultRenewalConfig,
		// 获取/释放脚本单次超时，略高于 Redis P99
		ScriptCallTimeout: 600 * time.Millisecond,
	}
}

// buildLockKeys 基于业务 key 生成实际用于 Redis 的 ownerKey 与 countKey。
// 使用相同的 hash tag，确保在 Redis Cluster 下两个键落在同一槽位。
func buildLockKeys(base string) (ownerKey, countKey string) {
	// 使用 per-key hashtag 分散槽位，同时保证 owner/count 同槽位：{lynx:lock:<base>}
	// 例如：{lynx:lock:order123}:owner 与 {lynx:lock:order123}:count
	hashtag := "{lynx:lock:" + base + "}"
	ownerKey = hashtag + ":owner"
	countKey = hashtag + ":count"
	return
}

// buildTokenKey 基于业务 key 生成 fencing token 计数器键。
// 与 owner/count 使用相同 hashtag，确保在 Redis Cluster 下落在同一槽位。
func buildTokenKey(base string) (tokenKey string) {
	hashtag := "{lynx:lock:" + base + "}"
	tokenKey = hashtag + ":token"
	return
}
