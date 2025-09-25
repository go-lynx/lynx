package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-lynx/lynx/app/resource"
	"github.com/rs/zerolog"
)

// Manager manages multiple cache instances
type Manager struct {
	caches map[string]*Cache
	mu     sync.RWMutex
	
	// 新增：缓存优化器
	optimizer *resource.CacheOptimizer
	logger    zerolog.Logger
}

// Config 缓存配置
type Config struct {
	MaxSize int
	TTL     time.Duration
}

// NewManager creates a new cache manager
func NewManager(logger zerolog.Logger) *Manager {
	// 创建缓存优化器
	optimizer := resource.NewCacheOptimizer(
		resource.DefaultCacheOptimizerConfig(),
		logger.With().Str("component", "cache_optimizer").Logger(),
	)

	manager := &Manager{
		caches:    make(map[string]*Cache),
		optimizer: optimizer,
		logger:    logger,
	}

	// 启动缓存优化器
	if err := optimizer.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to start cache optimizer")
	}

	return manager
}

// Create creates a new cache instance with the given name and options
func (m *Manager) Create(name string, opts *Options) (*Cache, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.caches[name]; exists {
		return nil, fmt.Errorf("cache %s already exists", name)
	}

	cache, err := New(name, opts)
	if err != nil {
		return nil, err
	}

	m.caches[name] = cache
	return cache, nil
}

// Get retrieves a cache instance by name
func (m *Manager) Get(name string) (*Cache, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cache, exists := m.caches[name]
	return cache, exists
}

// GetOrCreate retrieves an existing cache or creates a new one
func (m *Manager) GetOrCreate(name string, opts *Options) (*Cache, error) {
	// Try to get existing cache first
	if cache, exists := m.Get(name); exists {
		return cache, nil
	}

	// Create new cache if not exists
	return m.Create(name, opts)
}

// Delete removes a cache instance
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cache, exists := m.caches[name]
	if !exists {
		return fmt.Errorf("cache %s not found", name)
	}

	cache.Close()
	delete(m.caches, name)
	return nil
}

// Clear clears all items in a specific cache
func (m *Manager) Clear(name string) error {
	cache, exists := m.Get(name)
	if !exists {
		return fmt.Errorf("cache %s not found", name)
	}

	cache.Clear()
	return nil
}

// ClearAll clears all items in all caches
func (m *Manager) ClearAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cache := range m.caches {
		cache.Clear()
	}
}

// Close closes all cache instances
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭所有缓存
	for name, cache := range m.caches {
		cache.Close()
		m.logger.Debug().
			Str("cache", name).
			Msg("Closed cache")
	}

	// 停止缓存优化器
	if err := m.optimizer.Stop(); err != nil {
		m.logger.Error().Err(err).Msg("Failed to stop cache optimizer")
		return err
	}

	m.caches = make(map[string]*Cache)
	return nil
}

// List returns a list of all cache names
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.caches))
	for name := range m.caches {
		names = append(names, name)
	}
	return names
}

// Stats returns statistics for all caches
func (m *Manager) Stats() map[string]*ristretto.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]*ristretto.Metrics)
	for name, cache := range m.caches {
		if cache.cache.Metrics != nil {
			stats[name] = cache.Metrics()
		}
	}
	return stats
}

// CreateOptimized 创建优化的缓存实例
func (m *Manager) CreateOptimized(name string, config Config) (*OptimizedCacheWrapper, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.caches[name]; exists {
		return nil, fmt.Errorf("cache %s already exists", name)
	}

	// 创建优化缓存
	optimizedCache := m.optimizer.CreateCache(
		name,
		int64(config.MaxSize),
		config.TTL,
	)

	// 包装为Cache接口
	wrapper := &OptimizedCacheWrapper{
		cache:  optimizedCache,
		config: config,
		name:   name,
	}

	m.logger.Info().
		Str("cache", name).
		Interface("config", config).
		Msg("Created optimized cache")

	return wrapper, nil
}

// GetOptimizerMetrics 获取优化器指标
func (m *Manager) GetOptimizerMetrics() resource.CacheOptimizerMetrics {
	return m.optimizer.GetMetrics()
}

// OptimizedCacheWrapper 优化缓存包装器
type OptimizedCacheWrapper struct {
	cache  *resource.OptimizedCache
	config Config
	name   string
}

func (w *OptimizedCacheWrapper) Get(key interface{}) (interface{}, error) {
	keyStr := fmt.Sprintf("%v", key)
	value, exists := w.cache.Get(keyStr)
	if !exists {
		return nil, ErrCacheMiss
	}
	return value, nil
}

func (w *OptimizedCacheWrapper) Set(key interface{}, value interface{}, ttl time.Duration) error {
	keyStr := fmt.Sprintf("%v", key)
	// 估算值的大小（简化实现）
	size := int64(len(fmt.Sprintf("%v", value)))
	w.cache.Set(keyStr, value, size)
	return nil
}

func (w *OptimizedCacheWrapper) SetWithCost(key interface{}, value interface{}, cost int64, ttl time.Duration) error {
	keyStr := fmt.Sprintf("%v", key)
	w.cache.Set(keyStr, value, cost)
	return nil
}

func (w *OptimizedCacheWrapper) GetWithExpiration(key interface{}) (interface{}, bool) {
	keyStr := fmt.Sprintf("%v", key)
	return w.cache.Get(keyStr)
}

func (w *OptimizedCacheWrapper) Delete(key interface{}) {
	keyStr := fmt.Sprintf("%v", key)
	w.cache.Delete(keyStr)
}

func (w *OptimizedCacheWrapper) Clear() {
	w.cache.Clear()
}

func (w *OptimizedCacheWrapper) Has(key interface{}) bool {
	keyStr := fmt.Sprintf("%v", key)
	_, exists := w.cache.Get(keyStr)
	return exists
}

func (w *OptimizedCacheWrapper) Metrics() *ristretto.Metrics {
	// 返回nil，因为优化缓存使用不同的指标系统
	return nil
}

func (w *OptimizedCacheWrapper) Close() {
	// 优化缓存由优化器管理，这里不需要特殊处理
}

func (w *OptimizedCacheWrapper) Name() string {
	return w.name
}

func (w *OptimizedCacheWrapper) GetMulti(keys []interface{}) map[interface{}]interface{} {
	result := make(map[interface{}]interface{})
	for _, key := range keys {
		if value, err := w.Get(key); err == nil {
			result[key] = value
		}
	}
	return result
}

func (w *OptimizedCacheWrapper) SetMulti(items map[interface{}]interface{}, ttl time.Duration) error {
	for key, value := range items {
		if err := w.Set(key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (w *OptimizedCacheWrapper) DeleteMulti(keys []interface{}) {
	for _, key := range keys {
		w.Delete(key)
	}
}

func (w *OptimizedCacheWrapper) GetOrSet(key interface{}, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 先尝试获取
	if value, err := w.Get(key); err == nil {
		return value, nil
	}

	// 如果不存在，调用函数获取值
	value, err := fn()
	if err != nil {
		return nil, err
	}

	// 设置到缓存
	if err := w.Set(key, value, ttl); err != nil {
		return value, err // 返回值但报告设置错误
	}

	return value, nil
}

// DefaultManager is the global cache manager instance
var DefaultManager *Manager

// 初始化默认管理器
func init() {
	logger := zerolog.New(nil).With().Timestamp().Logger()
	DefaultManager = NewManager(logger)
}

// Create creates a new cache in the default manager
func Create(name string, opts *Options) (*Cache, error) {
	return DefaultManager.Create(name, opts)
}

// Get retrieves a cache from the default manager
func Get(name string) (*Cache, bool) {
	return DefaultManager.Get(name)
}

// GetOrCreate retrieves or creates a cache in the default manager
func GetOrCreate(name string, opts *Options) (*Cache, error) {
	return DefaultManager.GetOrCreate(name, opts)
}

// Delete removes a cache from the default manager
func Delete(name string) error {
	return DefaultManager.Delete(name)
}

// Clear clears a cache in the default manager
func Clear(name string) error {
	return DefaultManager.Clear(name)
}

// ClearAll clears all caches in the default manager
func ClearAll() {
	DefaultManager.ClearAll()
}

// Close closes all caches in the default manager
func Close() {
	DefaultManager.Close()
}

// List returns all cache names in the default manager
func List() []string {
	return DefaultManager.List()
}

// Stats returns statistics for all caches in the default manager
func Stats() map[string]*ristretto.Metrics {
	return DefaultManager.Stats()
}

// QuickCache creates a simple cache with default settings
func QuickCache(name string) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e6,     // 1 million
		MaxCost:     1 << 28, // 256MB
		BufferItems: 64,
		Metrics:     false,
	})
}

// SmallCache creates a small cache suitable for limited data
func SmallCache(name string) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e4,     // 10 thousand
		MaxCost:     1 << 24, // 16MB
		BufferItems: 64,
		Metrics:     false,
	})
}

// LargeCache creates a large cache suitable for big data
func LargeCache(name string) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e8,     // 100 million
		MaxCost:     1 << 32, // 4GB
		BufferItems: 64,
		Metrics:     true,
	})
}

// TTLCache creates a cache optimized for TTL-based eviction
func TTLCache(name string, defaultTTL time.Duration) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e6,     // 1 million
		MaxCost:     1 << 28, // 256MB
		BufferItems: 64,
		Metrics:     false,
		OnEvict: func(item *ristretto.Item) {
			// Custom eviction logic for TTL items
		},
	})
}

// CreateOptimizedCache 创建优化缓存的便捷函数
func CreateOptimizedCache(name string, maxSize int, ttl time.Duration) (*OptimizedCacheWrapper, error) {
	return DefaultManager.CreateOptimized(name, Config{
		MaxSize: maxSize,
		TTL:     ttl,
	})
}

// GetOptimizerMetrics 获取默认管理器的优化器指标
func GetOptimizerMetrics() resource.CacheOptimizerMetrics {
	return DefaultManager.GetOptimizerMetrics()
}
