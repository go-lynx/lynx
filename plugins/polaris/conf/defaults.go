package conf

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// 默认配置常量
const (
	// 命名空间相关
	DefaultNamespace = "default"

	// 服务权重相关
	DefaultWeight = 100
	MinWeight     = 1
	MaxWeight     = 1000

	// TTL 相关
	DefaultTTL = 30
	MinTTL     = 5
	MaxTTL     = 300

	// 超时相关
	DefaultTimeoutSeconds = 10
	MinTimeoutSeconds     = 1
	MaxTimeoutSeconds     = 60

	// 重试相关
	DefaultMaxRetryTimes = 3
	MinRetryTimes        = 0
	MaxRetryTimes        = 10
	DefaultRetryInterval = 1 * time.Second
	MinRetryInterval     = 100 * time.Millisecond
	MaxRetryInterval     = 30 * time.Second

	// 熔断器相关
	DefaultCircuitBreakerThreshold = 0.5
	MinCircuitBreakerThreshold     = 0.1
	MaxCircuitBreakerThreshold     = 0.9

	// 健康检查相关
	DefaultHealthCheckInterval = 30 * time.Second
	MinHealthCheckInterval     = 5 * time.Second
	MaxHealthCheckInterval     = 300 * time.Second

	// 优雅关闭相关
	DefaultShutdownTimeout = 30 * time.Second
	MinShutdownTimeout     = 5 * time.Second
	MaxShutdownTimeout     = 300 * time.Second

	// 负载均衡类型
	LoadBalancerTypeWeightedRandom = "weighted_random"
	LoadBalancerTypeRingHash       = "ring_hash"
	LoadBalancerTypeMaglev         = "maglev"
	LoadBalancerTypeL5CST          = "l5cst"

	// 限流类型
	RateLimitTypeLocal  = "local"
	RateLimitTypeGlobal = "global"

	// 日志级别
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// 支持的负载均衡类型
var SupportedLoadBalancerTypes = []string{
	LoadBalancerTypeWeightedRandom,
	LoadBalancerTypeRingHash,
	LoadBalancerTypeMaglev,
	LoadBalancerTypeL5CST,
}

// 支持的限流类型
var SupportedRateLimitTypes = []string{
	RateLimitTypeLocal,
	RateLimitTypeGlobal,
}

// 支持的日志级别
var SupportedLogLevels = []string{
	LogLevelDebug,
	LogLevelInfo,
	LogLevelWarn,
	LogLevelError,
}

// GetDefaultTimeout 获取默认超时时间
func GetDefaultTimeout() *durationpb.Duration {
	return &durationpb.Duration{Seconds: DefaultTimeoutSeconds}
}

// GetDefaultHealthCheckInterval 获取默认健康检查间隔
func GetDefaultHealthCheckInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultHealthCheckInterval.Seconds())}
}

// GetDefaultRetryInterval 获取默认重试间隔
func GetDefaultRetryInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultRetryInterval.Seconds())}
}

// GetDefaultShutdownTimeout 获取默认关闭超时
func GetDefaultShutdownTimeout() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultShutdownTimeout.Seconds())}
}
