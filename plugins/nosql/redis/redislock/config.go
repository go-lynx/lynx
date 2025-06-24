package redislock

import "time"

// Config 分布式锁配置
type Config struct {
	// 工作池大小
	WorkerPoolSize int
	// 最小续期阈值
	MinRenewalThreshold time.Duration
	// 批量处理大小
	BatchSize int
	// 最大重试次数
	MaxRetries int
	// 重试基础间隔
	RetryBackoff time.Duration
	// 续期提前量（过期时间的比例）
	RenewalAheadRatio float64
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	WorkerPoolSize:      100,
	MinRenewalThreshold: 500 * time.Millisecond,
	BatchSize:           100,
	MaxRetries:          3,
	RetryBackoff:        50 * time.Millisecond,
	RenewalAheadRatio:   0.33, // 提前 1/3 过期时间续期
}
