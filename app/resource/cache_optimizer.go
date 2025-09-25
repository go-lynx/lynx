// Package resource provides simple runtime resources used by other components.
package resource

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// CacheOptimizerConfig holds configuration for CacheOptimizer.
type CacheOptimizerConfig struct {
	// Reserved for future tuning knobs; currently unused
	DefaultTTL time.Duration
}

// DefaultCacheOptimizerConfig returns a reasonable default config.
func DefaultCacheOptimizerConfig() CacheOptimizerConfig {
	return CacheOptimizerConfig{DefaultTTL: 0}
}

// CacheOptimizerMetrics exposes basic metrics.
type CacheOptimizerMetrics struct {
	TotalCachesCreated int64
}

// CacheOptimizer manages optimized caches (lightweight stub implementation).
type CacheOptimizer struct {
	cfg     CacheOptimizerConfig
	logger  zerolog.Logger
	caches  sync.Map // name -> *OptimizedCache
	metrics CacheOptimizerMetrics
}

// NewCacheOptimizer creates a new optimizer instance.
func NewCacheOptimizer(cfg CacheOptimizerConfig, logger zerolog.Logger) *CacheOptimizer {
	return &CacheOptimizer{cfg: cfg, logger: logger}
}

// Start starts background tasks if any (no-op for stub).
func (o *CacheOptimizer) Start() error { return nil }

// Stop stops background tasks if any (no-op for stub).
func (o *CacheOptimizer) Stop() error { return nil }

// OptimizedCache is a tiny in-memory cache abstraction used by cache.Manager wrapper.
type OptimizedCache struct {
	items sync.Map // key string -> value any
}

// CreateCache creates or returns an OptimizedCache for the given name.
func (o *CacheOptimizer) CreateCache(name string, maxSize int64, ttl time.Duration) *OptimizedCache {
	// For stub we ignore maxSize/ttl and just create a map-backed cache.
	if v, ok := o.caches.Load(name); ok {
		if c, ok2 := v.(*OptimizedCache); ok2 {
			return c
		}
	}
	c := &OptimizedCache{}
	o.caches.Store(name, c)
	o.metrics.TotalCachesCreated++
	return c
}

// Get retrieves a value and whether it exists.
func (c *OptimizedCache) Get(key string) (interface{}, bool) {
	v, ok := c.items.Load(key)
	return v, ok
}

// Set stores a value with an associated cost (ignored in stub).
func (c *OptimizedCache) Set(key string, value interface{}, cost int64) {
	c.items.Store(key, value)
}

// Delete removes a key.
func (c *OptimizedCache) Delete(key string) { c.items.Delete(key) }

// Clear removes all items.
func (c *OptimizedCache) Clear() {
	c.items.Range(func(k, _ interface{}) bool {
		c.items.Delete(k)
		return true
	})
}

// GetMetrics returns a copy of metrics.
func (o *CacheOptimizer) GetMetrics() CacheOptimizerMetrics { return o.metrics }
