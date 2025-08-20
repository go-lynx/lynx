package polaris

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConcurrentStateAccess tests concurrent state access
func TestConcurrentStateAccess(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// Start multiple goroutines to access state concurrently
	var wg sync.WaitGroup
	concurrentCount := 100

	// Test concurrent state reading
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Concurrently call state check methods
			_ = plugin.IsInitialized()
			_ = plugin.IsDestroyed()
			_ = plugin.checkInitialized()
		}()
	}

	wg.Wait()

	// Verify state consistency
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())
}

// TestConcurrentWatcherManagement tests concurrent watcher management
func TestConcurrentWatcherManagement(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// Simulate initialization
	plugin.setInitialized()

	var wg sync.WaitGroup
	concurrentCount := 10
	serviceName := "test-service"

	// Concurrently create watchers
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This is just testing concurrency safety, not actually creating watchers
			plugin.watcherMutex.RLock()
			_ = plugin.activeWatchers[serviceName]
			plugin.watcherMutex.RUnlock()
		}()
	}

	wg.Wait()
}

// TestConcurrentCacheAccess tests concurrent cache access
func TestConcurrentCacheAccess(t *testing.T) {
	plugin := NewPolarisControlPlane()

	var wg sync.WaitGroup
	concurrentCount := 50

	// Concurrently write to cache
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			serviceName := fmt.Sprintf("service-%d", index)
			// Simulate cache update
			plugin.cacheMutex.Lock()
			if plugin.serviceCache == nil {
				plugin.serviceCache = make(map[string]interface{})
			}
			plugin.serviceCache[serviceName] = map[string]interface{}{
				"service": serviceName,
				"index":   index,
			}
			plugin.cacheMutex.Unlock()
		}(i)
	}

	// Concurrently read from cache
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			serviceName := fmt.Sprintf("service-%d", index)
			// Simulate cache reading
			plugin.cacheMutex.RLock()
			_ = plugin.serviceCache[serviceName]
			plugin.cacheMutex.RUnlock()
		}(i)
	}

	wg.Wait()

	// Verify cache consistency
	plugin.cacheMutex.RLock()
	cacheSize := len(plugin.serviceCache)
	plugin.cacheMutex.RUnlock()

	assert.Equal(t, concurrentCount, cacheSize)
}

// TestAtomicOperations tests atomic operations
func TestAtomicOperations(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// Test atomic state setting
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	plugin.setInitialized()
	assert.True(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	plugin.setDestroyed()
	assert.True(t, plugin.IsInitialized()) // Once initialized, state won't change
	assert.True(t, plugin.IsDestroyed())
}

// TestStateConsistency tests state consistency
func TestStateConsistency(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// Initial state
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	// Set initialization state
	plugin.setInitialized()
	assert.True(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	// Check state check method
	err := plugin.checkInitialized()
	assert.Nil(t, err)

	// Set destruction state
	plugin.setDestroyed()
	assert.True(t, plugin.IsInitialized()) // Initialization state won't change
	assert.True(t, plugin.IsDestroyed())

	// Check state check method should return error
	err = plugin.checkInitialized()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "has been destroyed")
}

// BenchmarkConcurrentStateAccess benchmarks concurrent state access performance
func BenchmarkConcurrentStateAccess(b *testing.B) {
	plugin := NewPolarisControlPlane()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = plugin.IsInitialized()
			_ = plugin.IsDestroyed()
			_ = plugin.checkInitialized()
		}
	})
}

// BenchmarkConcurrentCacheAccess benchmarks concurrent cache access performance
func BenchmarkConcurrentCacheAccess(b *testing.B) {
	plugin := NewPolarisControlPlane()

	// Initialize cache
	plugin.cacheMutex.Lock()
	plugin.serviceCache = make(map[string]interface{})
	plugin.cacheMutex.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", counter)

			// Write
			plugin.cacheMutex.Lock()
			plugin.serviceCache[key] = counter
			plugin.cacheMutex.Unlock()

			// Read
			plugin.cacheMutex.RLock()
			_ = plugin.serviceCache[key]
			plugin.cacheMutex.RUnlock()

			counter++
		}
	})
}
