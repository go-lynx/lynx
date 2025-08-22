package cache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

// Builder provides a fluent interface for building cache instances
type Builder struct {
	name    string
	options *Options
}

// NewBuilder creates a new cache builder
func NewBuilder(name string) *Builder {
	return &Builder{
		name:    name,
		options: DefaultOptions(),
	}
}

// WithNumCounters sets the number of counters for the cache
func (b *Builder) WithNumCounters(num int64) *Builder {
	b.options.NumCounters = num
	return b
}

// WithMaxCost sets the maximum cost for the cache
func (b *Builder) WithMaxCost(cost int64) *Builder {
	b.options.MaxCost = cost
	return b
}

// WithBufferItems sets the buffer items for the cache
func (b *Builder) WithBufferItems(items int64) *Builder {
	b.options.BufferItems = items
	return b
}

// WithMetrics enables or disables metrics collection
func (b *Builder) WithMetrics(enabled bool) *Builder {
	b.options.Metrics = enabled
	return b
}

// WithMaxItems sets the maximum number of items (approximation)
func (b *Builder) WithMaxItems(items int64) *Builder {
	b.options.NumCounters = items * 10
	b.options.MaxCost = items
	return b
}

// WithMaxMemory sets the maximum memory usage in bytes
func (b *Builder) WithMaxMemory(bytes int64) *Builder {
	b.options.MaxCost = bytes
	return b
}

// WithEvictionCallback sets the eviction callback
func (b *Builder) WithEvictionCallback(fn func(item *ristretto.Item)) *Builder {
	b.options.OnEvict = fn
	return b
}

// WithRejectionCallback sets the rejection callback
func (b *Builder) WithRejectionCallback(fn func(item *ristretto.Item)) *Builder {
	b.options.OnReject = fn
	return b
}

// WithExitCallback sets the exit callback
func (b *Builder) WithExitCallback(fn func(interface{})) *Builder {
	b.options.OnExit = fn
	return b
}

// WithHashFunction sets a custom hash function
func (b *Builder) WithHashFunction(fn func(key interface{}) (uint64, uint64)) *Builder {
	b.options.KeyToHash = fn
	return b
}

// WithCostFunction sets a custom cost calculation function
func (b *Builder) WithCostFunction(fn func(value interface{}) int64) *Builder {
	b.options.Cost = fn
	return b
}

// Build creates the cache instance
func (b *Builder) Build() (*Cache, error) {
	return New(b.name, b.options)
}

// BuildAndRegister creates the cache and registers it with the default manager
func (b *Builder) BuildAndRegister() (*Cache, error) {
	return DefaultManager.Create(b.name, b.options)
}

// Presets for common cache configurations

// SmallCacheBuilder creates a builder for small caches
func SmallCacheBuilder(name string) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e4).    // 10K
		WithMaxCost(1 << 24).     // 16MB
		WithBufferItems(64).
		WithMetrics(false)
}

// MediumCacheBuilder creates a builder for medium caches
func MediumCacheBuilder(name string) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e6).    // 1M
		WithMaxCost(1 << 28).     // 256MB
		WithBufferItems(64).
		WithMetrics(false)
}

// LargeCacheBuilder creates a builder for large caches
func LargeCacheBuilder(name string) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e8).    // 100M
		WithMaxCost(1 << 32).     // 4GB
		WithBufferItems(64).
		WithMetrics(true)
}

// SessionCacheBuilder creates a builder for session caches
func SessionCacheBuilder(name string, sessionTTL time.Duration) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e5).    // 100K sessions
		WithMaxCost(1 << 26).     // 64MB
		WithBufferItems(64).
		WithMetrics(false).
		WithEvictionCallback(func(item *ristretto.Item) {
			// Log session expiration if needed
		})
}

// APICacheBuilder creates a builder for API response caches
func APICacheBuilder(name string) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e6).    // 1M API calls
		WithMaxCost(1 << 29).     // 512MB
		WithBufferItems(128).    // Higher buffer for concurrent API calls
		WithMetrics(true)
}

// ObjectCacheBuilder creates a builder for object caches with custom cost calculation
func ObjectCacheBuilder(name string) *Builder {
	return NewBuilder(name).
		WithNumCounters(1e6).    // 1M objects
		WithMaxCost(1 << 30).     // 1GB
		WithBufferItems(64).
		WithMetrics(true).
		WithCostFunction(func(value interface{}) int64 {
			// Calculate cost based on object size
			// This is a simple example, adjust based on your needs
			switch v := value.(type) {
			case string:
				return int64(len(v))
			case []byte:
				return int64(len(v))
			default:
				return 1
			}
		})
}
