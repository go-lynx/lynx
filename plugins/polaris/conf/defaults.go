package conf

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// Default configuration constants
const (
	// Namespace related
	DefaultNamespace = "default"

	// Service weight related
	DefaultWeight = 100
	MinWeight     = 1
	MaxWeight     = 1000

	// TTL related
	DefaultTTL = 30
	MinTTL     = 5
	MaxTTL     = 300

	// Timeout related
	DefaultTimeoutSeconds = 10
	MinTimeoutSeconds     = 1
	MaxTimeoutSeconds     = 60

	// Retry related
	DefaultMaxRetryTimes = 3
	MinRetryTimes        = 0
	MaxRetryTimes        = 10
	DefaultRetryInterval = 1 * time.Second
	MinRetryInterval     = 100 * time.Millisecond
	MaxRetryInterval     = 30 * time.Second

	// Circuit breaker related
	DefaultCircuitBreakerThreshold = 0.5
	MinCircuitBreakerThreshold     = 0.1
	MaxCircuitBreakerThreshold     = 0.9

	// Health check related
	DefaultHealthCheckInterval = 30 * time.Second
	MinHealthCheckInterval     = 5 * time.Second
	MaxHealthCheckInterval     = 300 * time.Second

	// Graceful shutdown related
	DefaultShutdownTimeout = 30 * time.Second
	MinShutdownTimeout     = 5 * time.Second
	MaxShutdownTimeout     = 300 * time.Second

	// Load balancer types
	LoadBalancerTypeWeightedRandom = "weighted_random"
	LoadBalancerTypeRingHash       = "ring_hash"
	LoadBalancerTypeMaglev         = "maglev"
	LoadBalancerTypeL5CST          = "l5cst"

	// Rate limit types
	RateLimitTypeLocal  = "local"
	RateLimitTypeGlobal = "global"

	// Log levels
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Supported load balancer types
var SupportedLoadBalancerTypes = []string{
	LoadBalancerTypeWeightedRandom,
	LoadBalancerTypeRingHash,
	LoadBalancerTypeMaglev,
	LoadBalancerTypeL5CST,
}

// Supported rate limit types
var SupportedRateLimitTypes = []string{
	RateLimitTypeLocal,
	RateLimitTypeGlobal,
}

// Supported log levels
var SupportedLogLevels = []string{
	LogLevelDebug,
	LogLevelInfo,
	LogLevelWarn,
	LogLevelError,
}

// GetDefaultTimeout get default timeout duration
func GetDefaultTimeout() *durationpb.Duration {
	return &durationpb.Duration{Seconds: DefaultTimeoutSeconds}
}

// GetDefaultHealthCheckInterval get default health check interval
func GetDefaultHealthCheckInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultHealthCheckInterval.Seconds())}
}

// GetDefaultRetryInterval get default retry interval
func GetDefaultRetryInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultRetryInterval.Seconds())}
}

// GetDefaultShutdownTimeout get default shutdown timeout
func GetDefaultShutdownTimeout() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultShutdownTimeout.Seconds())}
}
