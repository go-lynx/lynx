package cache

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/ristretto"
)

var (
	// ErrCacheMiss indicates that a key was not found in the cache
	ErrCacheMiss = errors.New("cache: key not found")
	// ErrCacheSet indicates that a value could not be set in the cache
	ErrCacheSet = errors.New("cache: failed to set value")
	// ErrInvalidTTL indicates that an invalid TTL was provided
	ErrInvalidTTL = errors.New("cache: invalid TTL")
)

// Cache represents a thread-safe in-memory cache with TTL support
type Cache struct {
	cache *ristretto.Cache
	name  string
}

// Options represents cache configuration options
type Options struct {
	// NumCounters is the number of 4-bit counters for admission policy (10x max items)
	NumCounters int64
	// MaxCost is the maximum cost of cache (sum of all items' costs)
	MaxCost int64
	// BufferItems is the number of keys per Get buffer
	BufferItems int64
	// Metrics enables cache metrics collection
	Metrics bool
	// OnEvict is called when an item is evicted from the cache
	OnEvict func(item *ristretto.Item)
	// OnReject is called when an item is rejected from the cache
	OnReject func(item *ristretto.Item)
	// OnExit is called when cache.Close() is called
	OnExit func(interface{})
	// KeyToHash is a custom hash function for keys
	KeyToHash func(key interface{}) (uint64, uint64)
	// Cost is a function to calculate the cost of a value
	Cost func(value interface{}) int64
}

// DefaultOptions returns default cache options
func DefaultOptions() *Options {
	return &Options{
		NumCounters: 1e7,     // 10 million
		MaxCost:     1 << 30, // 1GB
		BufferItems: 64,
		Metrics:     false,
	}
}

// New creates a new cache instance with the given options
func New(name string, opts *Options) (*Cache, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	config := &ristretto.Config{
		NumCounters: opts.NumCounters,
		MaxCost:     opts.MaxCost,
		BufferItems: opts.BufferItems,
		Metrics:     opts.Metrics,
	}

	if opts.OnEvict != nil {
		config.OnEvict = opts.OnEvict
	}
	if opts.OnReject != nil {
		config.OnReject = opts.OnReject
	}
	if opts.OnExit != nil {
		config.OnExit = opts.OnExit
	}
	if opts.KeyToHash != nil {
		config.KeyToHash = opts.KeyToHash
	}
	if opts.Cost != nil {
		config.Cost = opts.Cost
	}

	cache, err := ristretto.NewCache(config)
	if err != nil {
		return nil, err
	}

	return &Cache{
		cache: cache,
		name:  name,
	}, nil
}

// Set stores a key-value pair in the cache with the given TTL
func (c *Cache) Set(key interface{}, value interface{}, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	cost := int64(1)
	if ttl == 0 {
		// No expiration
		if !c.cache.Set(key, value, cost) {
			return ErrCacheSet
		}
	} else {
		// With TTL
		if !c.cache.SetWithTTL(key, value, cost, ttl) {
			return ErrCacheSet
		}
	}

	// Wait for the value to be processed
	c.cache.Wait()
	return nil
}

// SetWithCost stores a key-value pair with a custom cost
func (c *Cache) SetWithCost(key interface{}, value interface{}, cost int64, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	if ttl == 0 {
		if !c.cache.Set(key, value, cost) {
			return ErrCacheSet
		}
	} else {
		if !c.cache.SetWithTTL(key, value, cost, ttl) {
			return ErrCacheSet
		}
	}

	c.cache.Wait()
	return nil
}

// Get retrieves a value from the cache by key
func (c *Cache) Get(key interface{}) (interface{}, error) {
	value, found := c.cache.Get(key)
	if !found {
		return nil, ErrCacheMiss
	}
	return value, nil
}

// GetWithExpiration retrieves a value and checks if it exists
func (c *Cache) GetWithExpiration(key interface{}) (interface{}, bool) {
	value, found := c.cache.Get(key)
	return value, found
}

// Delete removes a key from the cache
func (c *Cache) Delete(key interface{}) {
	c.cache.Del(key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.cache.Clear()
}

// Has checks if a key exists in the cache
func (c *Cache) Has(key interface{}) bool {
	_, found := c.cache.Get(key)
	return found
}

// Metrics returns cache statistics
func (c *Cache) Metrics() *ristretto.Metrics {
	return c.cache.Metrics
}

// Close gracefully shuts down the cache
func (c *Cache) Close() {
	c.cache.Close()
}

// Name returns the cache name
func (c *Cache) Name() string {
	return c.name
}

// GetMulti retrieves multiple values from the cache
func (c *Cache) GetMulti(keys []interface{}) map[interface{}]interface{} {
	result := make(map[interface{}]interface{})
	for _, key := range keys {
		if value, found := c.cache.Get(key); found {
			result[key] = value
		}
	}
	return result
}

// SetMulti stores multiple key-value pairs in the cache
func (c *Cache) SetMulti(items map[interface{}]interface{}, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}

	for key, value := range items {
		if ttl == 0 {
			if !c.cache.Set(key, value, 1) {
				return ErrCacheSet
			}
		} else {
			if !c.cache.SetWithTTL(key, value, 1, ttl) {
				return ErrCacheSet
			}
		}
	}

	c.cache.Wait()
	return nil
}

// DeleteMulti removes multiple keys from the cache
func (c *Cache) DeleteMulti(keys []interface{}) {
	for _, key := range keys {
		c.cache.Del(key)
	}
}

// GetOrSet retrieves a value from the cache or sets it if not found
func (c *Cache) GetOrSet(key interface{}, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get from cache first
	if value, found := c.cache.Get(key); found {
		return value, nil
	}

	// Generate value if not found
	value, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.Set(key, value, ttl); err != nil {
		// Even if cache set fails, return the value
		return value, nil
	}

	return value, nil
}

// GetOrSetContext is like GetOrSet but with context support
func (c *Cache) GetOrSetContext(ctx context.Context, key interface{}, fn func(context.Context) (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get from cache first
	if value, found := c.cache.Get(key); found {
		return value, nil
	}

	// Check context before generating value
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Generate value if not found
	value, err := fn(ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.Set(key, value, ttl); err != nil {
		// Even if cache set fails, return the value
		return value, nil
	}

	return value, nil
}
