package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

// Manager manages multiple cache instances
type Manager struct {
	caches map[string]*Cache
	mu     sync.RWMutex
}

// NewManager creates a new cache manager
func NewManager() *Manager {
	return &Manager{
		caches: make(map[string]*Cache),
	}
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
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cache := range m.caches {
		cache.Close()
	}
	m.caches = make(map[string]*Cache)
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

// DefaultManager is the global cache manager instance
var DefaultManager = NewManager()

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
		NumCounters: 1e6,    // 1 million
		MaxCost:     1 << 28, // 256MB
		BufferItems: 64,
		Metrics:     false,
	})
}

// SmallCache creates a small cache suitable for limited data
func SmallCache(name string) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e4,    // 10 thousand
		MaxCost:     1 << 24, // 16MB
		BufferItems: 64,
		Metrics:     false,
	})
}

// LargeCache creates a large cache suitable for big data
func LargeCache(name string) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e8,    // 100 million
		MaxCost:     1 << 32, // 4GB
		BufferItems: 64,
		Metrics:     true,
	})
}

// TTLCache creates a cache optimized for TTL-based eviction
func TTLCache(name string, defaultTTL time.Duration) (*Cache, error) {
	return GetOrCreate(name, &Options{
		NumCounters: 1e6,    // 1 million
		MaxCost:     1 << 28, // 256MB
		BufferItems: 64,
		Metrics:     false,
		OnEvict: func(item *ristretto.Item) {
			// Custom eviction logic for TTL items
		},
	})
}
