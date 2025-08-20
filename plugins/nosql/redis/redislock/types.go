package redislock

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// LockOptions lock configuration options
type LockOptions struct {
	Expiration       time.Duration // Lock expiration time
	RetryStrategy    RetryStrategy // Retry strategy
	RenewalEnabled   bool          // Whether to enable auto renewal
	RenewalThreshold float64       // Renewal threshold (proportion relative to expiration time, default 1/3)
	WorkerPoolSize   int           // Renewal worker pool size, default 50
	RenewalConfig    RenewalConfig // Renewal configuration
	// ScriptCallTimeout timeout control for single script call (acquire/release). 0 means no separate timeout.
	ScriptCallTimeout time.Duration
}

// Validate validates configuration options
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

// ValidateKey validates the validity of lock key name
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("lock key cannot be empty")
	}

	if len(key) > 255 {
		return fmt.Errorf("lock key too long, max length is 255, got %d", len(key))
	}

	// Check for invalid characters
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

// Validate validates retry strategy
func (rs *RetryStrategy) Validate() error {
	if rs.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", rs.MaxRetries)
	}

	if rs.RetryDelay < 0 {
		return fmt.Errorf("retry delay must be non-negative, got %v", rs.RetryDelay)
	}

	return nil
}

// RetryStrategy defines lock retry strategy
type RetryStrategy struct {
	MaxRetries int           // Maximum retry attempts
	RetryDelay time.Duration // Retry interval
}

// RenewalConfig renewal configuration
type RenewalConfig struct {
	MaxRetries    int           // Maximum renewal retry attempts
	BaseDelay     time.Duration // Base retry delay
	MaxDelay      time.Duration // Maximum retry delay
	CheckInterval time.Duration // Renewal check interval
	// CallTimeout single renewal script call timeout. 0 means no separate timeout.
	CallTimeout time.Duration
}

// LockCallback lock operation callback interface
type LockCallback interface {
	OnLockAcquired(key string, duration time.Duration)
	OnLockReleased(key string, duration time.Duration)
	OnLockRenewed(key string, duration time.Duration)
	OnLockRenewalFailed(key string, error error)
	OnLockAcquireFailed(key string, error error)
}

// NoOpCallback empty implementation callback
type NoOpCallback struct{}

func (NoOpCallback) OnLockAcquired(key string, duration time.Duration) {}
func (NoOpCallback) OnLockReleased(key string, duration time.Duration) {}
func (NoOpCallback) OnLockRenewed(key string, duration time.Duration)  {}
func (NoOpCallback) OnLockRenewalFailed(key string, error error)       {}
func (NoOpCallback) OnLockAcquireFailed(key string, error error)       {}

// RedisLock implements Redis-based distributed lock
type RedisLock struct {
	client           *redis.Client // Redis client
	key              string        // Lock key name
	value            string        // Lock value (used to identify holder)
	expiration       time.Duration // Lock expiration time
	expiresAt        time.Time     // Lock expiration time point
	mutex            sync.Mutex    // Protect internal state
	renewalThreshold float64       // Renewal threshold
	acquiredAt       time.Time     // Time when lock was acquired

	// Two keys actually used in Redis (using same hash tag to ensure same slot in cluster)
	ownerKey string // Key storing holder identifier
	countKey string // Key storing reentry count
	// fencing token key and most recently acquired token value
	// Note: token is only incremented and recorded on first acquisition (non-reentrant).
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

// lockManager manages all distributed lock instances
type lockManager struct {
	mutex sync.RWMutex
	locks map[string]*RedisLock
	// Renewal service
	renewCtx    context.Context
	renewCancel context.CancelFunc
	running     bool
	// Worker pool
	workerPool chan struct{}
	// Statistics
	stats struct {
		TotalLocks    int64
		ActiveLocks   int64
		RenewalCount  int64
		RenewalErrors int64
		// Number of renewals skipped when worker pool is full
		SkippedRenewals int64
		// Accumulated renewal latency (nanoseconds) and count for calculating average latency
		RenewLatencyNs    int64
		RenewLatencyCount int64
		WorkerPoolCap     int
	}
}

// Default configurations
var (
	DefaultRetryStrategy = RetryStrategy{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	DefaultRenewalConfig = RenewalConfig{
		// More conservative renewal strategy to cover short Redis latency spikes
		MaxRetries:    4,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      800 * time.Millisecond,
		CheckInterval: 300 * time.Millisecond,
		CallTimeout:   600 * time.Millisecond,
	}

	DefaultLockOptions LockOptions
)

// init initializes default configurations to avoid circular dependencies
func init() {
	DefaultLockOptions = LockOptions{
		Expiration:     30 * time.Second,
		RetryStrategy:  DefaultRetryStrategy,
		RenewalEnabled: true,
		// Enter renewal window at 30% TTL in advance, leaving enough margin for retries and jitter
		RenewalThreshold: 0.3,
		// Increase default worker pool capacity to reduce skip probability (can be adjusted based on business load testing)
		WorkerPoolSize: 50,
		RenewalConfig:  DefaultRenewalConfig,
		// Single timeout for acquire/release scripts, slightly higher than Redis P99
		ScriptCallTimeout: 600 * time.Millisecond,
	}
}

// buildLockKeys generates actual Redis ownerKey and countKey based on business key.
// Uses the same hash tag to ensure both keys fall in the same slot under Redis Cluster.
func buildLockKeys(base string) (ownerKey, countKey string) {
	// Use per-key hashtag to distribute slots while ensuring owner/count are in the same slot: {lynx:lock:<base>}
	// For example: {lynx:lock:order123}:owner and {lynx:lock:order123}:count
	hashtag := "{lynx:lock:" + base + "}"
	ownerKey = hashtag + ":owner"
	countKey = hashtag + ":count"
	return
}

// buildTokenKey generates fencing token counter key based on business key.
// Uses the same hashtag as owner/count to ensure it falls in the same slot under Redis Cluster.
func buildTokenKey(base string) (tokenKey string) {
	hashtag := "{lynx:lock:" + base + "}"
	tokenKey = hashtag + ":token"
	return
}
