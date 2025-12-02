package cache

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-lynx/lynx/internal/resource"
	"github.com/rs/zerolog"
)

// Manager manages multiple cache instances
type Manager struct {
	caches map[string]*Cache
	mu     sync.RWMutex

	// New: cache optimizer
	optimizer *resource.CacheOptimizer
	logger    zerolog.Logger
}

// Config cache configuration
type Config struct {
	MaxSize int
	TTL     time.Duration
}

// NewManager creates a new cache manager with a default logger.
func NewManager() *Manager {
	// Create a default logger to avoid nil writer issues.
	defLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	return NewManagerWithLogger(defLogger)
}

// NewManagerWithLogger creates a new cache manager using the provided logger.
func NewManagerWithLogger(logger zerolog.Logger) *Manager {
	// Create cache optimizer
	optimizer := resource.NewCacheOptimizer(
		resource.DefaultCacheOptimizerConfig(),
		logger.With().Str("component", "cache_optimizer").Logger(),
	)

	manager := &Manager{
		caches:    make(map[string]*Cache),
		optimizer: optimizer,
		logger:    logger,
	}

	// Start cache optimizer
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

	// Close all caches
	for name, cache := range m.caches {
		cache.Close()
		m.logger.Debug().
			Str("cache", name).
			Msg("Closed cache")
	}

	// Stop cache optimizer
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

// CreateOptimized creates an optimized cache instance
func (m *Manager) CreateOptimized(name string, config Config) (*OptimizedCacheWrapper, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.caches[name]; exists {
		return nil, fmt.Errorf("cache %s already exists", name)
	}

	// Create optimized cache
	optimizedCache := m.optimizer.CreateCache(
		name,
		int64(config.MaxSize),
		config.TTL,
	)

	// Wrap as Cache interface
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

// GetOptimizerMetrics gets optimizer metrics
func (m *Manager) GetOptimizerMetrics() resource.CacheOptimizerMetrics {
	return m.optimizer.GetMetrics()
}

// OptimizedCacheWrapper optimized cache wrapper
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
	// Estimate value size (simplified implementation)
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
	// Return nil because optimized cache uses a different metrics system
	return nil
}

func (w *OptimizedCacheWrapper) Close() {
	// Optimized cache is managed by the optimizer, no special handling needed here
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
	// Try to get first
	if value, err := w.Get(key); err == nil {
		return value, nil
	}

	// If not exists, call function to get value
	value, err := fn()
	if err != nil {
		return nil, err
	}

	// Set to cache
	if err := w.Set(key, value, ttl); err != nil {
		return value, err // Return value but report set error
	}

	return value, nil
}

// DefaultManager is the global cache manager instance
var DefaultManager *Manager

// Initialize default manager
func init() {
	DefaultManager = NewManager()
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

// CreateOptimizedCache creates an optimized cache (convenience function)
func CreateOptimizedCache(name string, maxSize int, ttl time.Duration) (*OptimizedCacheWrapper, error) {
	return DefaultManager.CreateOptimized(name, Config{
		MaxSize: maxSize,
		TTL:     ttl,
	})
}

// GetOptimizerMetrics gets optimizer metrics from the default manager
func GetOptimizerMetrics() resource.CacheOptimizerMetrics {
	return DefaultManager.GetOptimizerMetrics()
}
