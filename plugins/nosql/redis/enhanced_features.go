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

// CacheManager 提供高级缓存管理功能
type CacheManager struct {
	client redis.UniversalClient
	logger zerolog.Logger
	mu     sync.RWMutex
	stats  *CacheStats
}

// CacheStats 缓存统计
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

// NewCacheManager 创建缓存管理器
func NewCacheManager(client redis.UniversalClient, logger zerolog.Logger) *CacheManager {
	return &CacheManager{
		client: client,
		logger: logger,
		stats: &CacheStats{
			lastReset: time.Now(),
		},
	}
}

// GetWithLoader 获取缓存，如果不存在则使用loader加载
func (cm *CacheManager) GetWithLoader(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	start := time.Now()
	defer cm.recordLatency(start)
	
	// 尝试从缓存获取
	val, err := cm.client.Get(ctx, key).Result()
	if err == nil {
		cm.recordHit()
		
		var result interface{}
		if err := json.Unmarshal([]byte(val), &result); err == nil {
			return result, nil
		}
	} else if err != redis.Nil {
		cm.recordError()
		cm.logger.Error().Err(err).Str("key", key).Msg("Failed to get from cache")
	} else {
		cm.recordMiss()
	}
	
	// 使用loader加载数据
	data, err := loader()
	if err != nil {
		return nil, fmt.Errorf("loader failed: %w", err)
	}
	
	// 序列化并缓存
	jsonData, err := json.Marshal(data)
	if err != nil {
		return data, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	if err := cm.client.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		cm.recordError()
		cm.logger.Error().Err(err).Str("key", key).Msg("Failed to set cache")
		// 即使缓存失败，也返回数据
		return data, nil
	}
	
	cm.recordSet()
	return data, nil
}

// SetWithRetry 带重试的缓存设置
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
		
		// 指数退避
		time.Sleep(time.Duration(1<<uint(i)) * time.Millisecond * 100)
	}
	
	cm.recordError()
	return fmt.Errorf("failed to set cache after %d retries", maxRetries)
}

// DeletePattern 删除匹配模式的所有键
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

// GetStats 获取缓存统计
func (cm *CacheManager) GetStats() CacheStats {
	cm.stats.mu.RLock()
	defer cm.stats.mu.RUnlock()
	return *cm.stats
}

// ResetStats 重置统计
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
	
	// 简单的移动平均
	if cm.stats.AvgLatency == 0 {
		cm.stats.AvgLatency = latency
	} else {
		cm.stats.AvgLatency = (cm.stats.AvgLatency + latency) / 2
	}
}

// DistributedLock 分布式锁实现
type DistributedLock struct {
	client   redis.UniversalClient
	key      string
	value    string
	ttl      time.Duration
	logger   zerolog.Logger
	unlocked bool
	mu       sync.Mutex
}

// NewDistributedLock 创建分布式锁
func NewDistributedLock(client redis.UniversalClient, key string, ttl time.Duration, logger zerolog.Logger) *DistributedLock {
	return &DistributedLock{
		client: client,
		key:    fmt.Sprintf("lock:%s", key),
		value:  fmt.Sprintf("%d:%d", time.Now().UnixNano(), randInt()),
		ttl:    ttl,
		logger: logger,
	}
}

// Lock 获取锁
func (dl *DistributedLock) Lock(ctx context.Context) error {
	return dl.LockWithRetry(ctx, 1)
}

// LockWithRetry 带重试的获取锁
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
		
		// 等待后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100 * time.Duration(i+1)):
			// 指数退避
		}
	}
	
	return fmt.Errorf("failed to acquire lock after %d retries", maxRetries)
}

// Unlock 释放锁
func (dl *DistributedLock) Unlock(ctx context.Context) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	if dl.unlocked {
		return fmt.Errorf("lock already released")
	}
	
	// 使用Lua脚本确保只释放自己的锁
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

// Extend 延长锁的时间
func (dl *DistributedLock) Extend(ctx context.Context, additionalTTL time.Duration) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	if dl.unlocked {
		return fmt.Errorf("lock already released")
	}
	
	// 使用Lua脚本确保只延长自己的锁
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

// RateLimiter 速率限制器
type RateLimiter struct {
	client redis.UniversalClient
	logger zerolog.Logger
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(client redis.UniversalClient, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		client: client,
		logger: logger,
	}
}

// Allow 检查是否允许操作
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())
	
	pipe := rl.client.Pipeline()
	
	// 移除窗口外的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// 计算当前窗口内的请求数
	count := pipe.ZCard(ctx, key)
	
	// 执行管道
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}
	
	// 获取计数
	currentCount, err := count.Result()
	if err != nil {
		return false, err
	}
	
	// 检查是否超限
	if currentCount >= int64(limit) {
		rl.logger.Warn().
			Str("key", key).
			Int64("count", currentCount).
			Int("limit", limit).
			Msg("Rate limit exceeded")
		return false, nil
	}
	
	// 添加新记录
	member := fmt.Sprintf("%d:%d", now, randInt())
	if err := rl.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: member,
	}).Err(); err != nil {
		return false, fmt.Errorf("failed to add rate limit record: %w", err)
	}
	
	// 设置过期时间
	rl.client.Expire(ctx, key, window)
	
	return true, nil
}

// GetUsage 获取当前使用量
func (rl *RateLimiter) GetUsage(ctx context.Context, key string, window time.Duration) (int64, error) {
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())
	
	// 移除窗口外的记录
	rl.client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	
	// 获取当前计数
	return rl.client.ZCard(ctx, key).Result()
}

// Reset 重置限制
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.client.Del(ctx, key).Err()
}

// randInt 生成随机数
func randInt() int {
	return int(time.Now().UnixNano() % 1000000)
}