package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// CacheManager provides advanced cache management features
type CacheManager struct {
	client redis.UniversalClient
	logger zerolog.Logger
	stats  *CacheStats
}

// CacheStats cache statistics
type CacheStats struct {
	Hits        uint64
	Misses      uint64
	Sets        uint64
	Deletes     uint64
	Errors      uint64
	AvgLatency  time.Duration
	lastReset   time.Time
	mu          sync.RWMutex
}

// NewCacheManager creates a cache manager
func NewCacheManager(client redis.UniversalClient, logger zerolog.Logger) *CacheManager {
	return &CacheManager{
		client: client,
		logger: logger,
		stats: &CacheStats{
			lastReset: time.Now(),
		},
	}
}

// GetWithLoader gets from cache, or uses loader when missing
func (cm *CacheManager) GetWithLoader(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	start := time.Now()
	defer cm.recordLatency(start)
	
	// Try to get from cache
	val, err := cm.client.Get(ctx, key).Result()
	if err == nil {
		cm.recordHit()
		
		var result interface{}
		if unmarshalErr := json.Unmarshal([]byte(val), &result); unmarshalErr == nil {
			return result, nil
		}
	} else if err != redis.Nil {
		cm.recordError()
		cm.logger.Error().Err(err).Str("key", key).Msg("Failed to get from cache")
	} else {
		cm.recordMiss()
	}
	
	// Load data using loader
	data, err := loader()
	if err != nil {
		return nil, fmt.Errorf("loader failed: %w", err)
	}
	
	// Marshal and cache
	jsonData, err := json.Marshal(data)
	if err != nil {
		return data, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	if err := cm.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		cm.recordError()
		cm.logger.Error().Err(err).Str("key", key).Msg("Failed to set cache")
		// Return data even if caching fails
		return data, nil
	}
	
	cm.recordSet()
	return data, nil
}

// SetWithRetry sets cache value with retry
func (cm *CacheManager) SetWithRetry(ctx context.Context, key string, value interface{}, ttl time.Duration, maxRetries int) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	
	for i := 0; i < maxRetries; i++ {
		err := cm.client.Set(ctx, key, jsonData, ttl).Err()
		if err == nil {
			cm.recordSet()
			return nil
		}
		
		cm.logger.Warn().
			Err(err).
			Int("attempt", i+1).
			Str("key", key).
			Msg("Retry cache set")
		
		// Exponential backoff
		time.Sleep(time.Duration(1<<uint(i)) * time.Millisecond * 100)
	}
	
	cm.recordError()
	return fmt.Errorf("failed to set cache after %d retries", maxRetries)
}

// DeletePattern deletes all keys matching pattern
func (cm *CacheManager) DeletePattern(ctx context.Context, pattern string) error {
	iter := cm.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	
	if err := iter.Err(); err != nil {
		cm.recordError()
		return fmt.Errorf("failed to scan keys: %w", err)
	}
	
	if len(keys) > 0 {
		if err := cm.client.Del(ctx, keys...).Err(); err != nil {
			cm.recordError()
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		cm.recordDeletes(uint64(len(keys)))
	}
	
	return nil
}

// GetStats gets cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	cm.stats.mu.RLock()
	defer cm.stats.mu.RUnlock()
	// Create a copy without the mutex to avoid lock copy warnings
	return CacheStats{
		Hits:       cm.stats.Hits,
		Misses:     cm.stats.Misses,
		Sets:       cm.stats.Sets,
		Deletes:    cm.stats.Deletes,
		Errors:     cm.stats.Errors,
		AvgLatency: cm.stats.AvgLatency,
		lastReset:  cm.stats.lastReset,
		// Do not copy mu field
	}
}

// ResetStats resets statistics
func (cm *CacheManager) ResetStats() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Hits = 0
	cm.stats.Misses = 0
	cm.stats.Sets = 0
	cm.stats.Deletes = 0
	cm.stats.Errors = 0
	cm.stats.AvgLatency = 0
	cm.stats.lastReset = time.Now()
}

func (cm *CacheManager) recordHit() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Hits++
}

func (cm *CacheManager) recordMiss() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Misses++
}

func (cm *CacheManager) recordSet() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Sets++
}

func (cm *CacheManager) recordDeletes(count uint64) {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Deletes += count
}

func (cm *CacheManager) recordError() {
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	cm.stats.Errors++
}

func (cm *CacheManager) recordLatency(start time.Time) {
	latency := time.Since(start)
	cm.stats.mu.Lock()
	defer cm.stats.mu.Unlock()
	
	// Simple moving average
	if cm.stats.AvgLatency == 0 {
		cm.stats.AvgLatency = latency
	} else {
		cm.stats.AvgLatency = (cm.stats.AvgLatency + latency) / 2
	}
}

// DistributedLock distributed lock implementation
type DistributedLock struct {
	client   redis.UniversalClient
	key      string
	value    string
	ttl      time.Duration
	logger   zerolog.Logger
	unlocked bool
	mu       sync.Mutex
}

// NewDistributedLock creates a distributed lock
func NewDistributedLock(client redis.UniversalClient, key string, ttl time.Duration, logger zerolog.Logger) *DistributedLock {
	return &DistributedLock{
		client: client,
		key:    fmt.Sprintf("lock:%s", key),
		value:  fmt.Sprintf("%d:%d", time.Now().UnixNano(), randInt()),
		ttl:    ttl,
		logger: logger,
	}
}

// Lock acquires the lock
func (dl *DistributedLock) Lock(ctx context.Context) error {
	return dl.LockWithRetry(ctx, 1)
}

// LockWithRetry acquires the lock with retry
func (dl *DistributedLock) LockWithRetry(ctx context.Context, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		success, err := dl.client.SetNX(ctx, dl.key, dl.value, dl.ttl).Result()
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		
		if success {
			dl.logger.Debug().
				Str("key", dl.key).
				Str("value", dl.value).
				Msg("Lock acquired")
			return nil
		}
		
		// Wait then retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100 * time.Duration(i+1)):
			// Exponential backoff
		}
	}
	
	return fmt.Errorf("failed to acquire lock after %d retries", maxRetries)
}

// Unlock releases the lock
func (dl *DistributedLock) Unlock(ctx context.Context) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	if dl.unlocked {
		return fmt.Errorf("lock already released")
	}
	
	// Use Lua script to ensure only the owner releases the lock
	script := redis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('DEL', KEYS[1])
		else
			return 0
		end
	`)
	
	result, err := script.Run(ctx, dl.client, []string{dl.key}, dl.value).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	
	if result.(int64) == 0 {
		return fmt.Errorf("lock not found or already expired")
	}
	
	dl.unlocked = true
	dl.logger.Debug().
		Str("key", dl.key).
		Str("value", dl.value).
		Msg("Lock released")
	
	return nil
}

// Extend extends the lock TTL
func (dl *DistributedLock) Extend(ctx context.Context, additionalTTL time.Duration) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	if dl.unlocked {
		return fmt.Errorf("lock already released")
	}
	
	// Use Lua script to ensure only the owner extends the lock
	script := redis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('EXPIRE', KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	
	newTTL := int(additionalTTL.Seconds())
	result, err := script.Run(ctx, dl.client, []string{dl.key}, dl.value, newTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}
	
	if result.(int64) == 0 {
		return fmt.Errorf("lock not found or already expired")
	}
	
	return nil
}

// RateLimiter rate limiter
type RateLimiter struct {
	client redis.UniversalClient
	logger zerolog.Logger
}

// NewRateLimiter creates a rate limiter
func NewRateLimiter(client redis.UniversalClient, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		client: client,
		logger: logger,
	}
}

// Allow checks whether the operation is allowed
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())
	
	pipe := rl.client.Pipeline()
	
	// Remove records outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// Count requests within the current window
	count := pipe.ZCard(ctx, key)
	
	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}
	
	// Get count
	currentCount, err := count.Result()
	if err != nil {
		return false, err
	}
	
	// Check if limit exceeded
	if currentCount >= int64(limit) {
		rl.logger.Warn().
			Str("key", key).
			Int64("count", currentCount).
			Int("limit", limit).
			Msg("Rate limit exceeded")
		return false, nil
	}
	
	// Add new record
	member := fmt.Sprintf("%d:%d", now, randInt())
	if err := rl.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: member,
	}).Err(); err != nil {
		return false, fmt.Errorf("failed to add rate limit record: %w", err)
	}
	
	// Set expiration
	rl.client.Expire(ctx, key, window)
	
	return true, nil
}

// GetUsage gets current usage
func (rl *RateLimiter) GetUsage(ctx context.Context, key string, window time.Duration) (int64, error) {
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())
	
	// Remove records outside the window
	rl.client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// Get current count
	return rl.client.ZCard(ctx, key).Result()
}

// Reset resets the limiter
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.client.Del(ctx, key).Err()
}

// randInt generates a pseudo-random int
func randInt() int {
	return int(time.Now().UnixNano() % 1000000)
}