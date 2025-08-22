package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCache_BasicOperations(t *testing.T) {
	c, err := New("test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Test Set and Get
	key := "test-key"
	value := "test-value"
	
	err = c.Set(key, value, 0)
	if err != nil {
		t.Errorf("Failed to set value: %v", err)
	}

	got, err := c.Get(key)
	if err != nil {
		t.Errorf("Failed to get value: %v", err)
	}
	
	if got != value {
		t.Errorf("Expected %v, got %v", value, got)
	}

	// Test Has
	if !c.Has(key) {
		t.Error("Expected key to exist")
	}

	// Test Delete
	c.Delete(key)
	if c.Has(key) {
		t.Error("Expected key to be deleted")
	}

	// Test Get on deleted key
	_, err = c.Get(key)
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}
}

func TestCache_TTL(t *testing.T) {
	c, err := New("ttl-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	key := "ttl-key"
	value := "ttl-value"
	
	// Set with 1 second TTL
	err = c.Set(key, value, 1*time.Second)
	if err != nil {
		t.Errorf("Failed to set value with TTL: %v", err)
	}

	// Should exist immediately
	if !c.Has(key) {
		t.Error("Key should exist immediately after setting")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Should not exist after TTL
	if c.Has(key) {
		t.Error("Key should not exist after TTL")
	}
}

func TestCache_InvalidTTL(t *testing.T) {
	c, err := New("invalid-ttl", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	err = c.Set("key", "value", -1*time.Second)
	if err != ErrInvalidTTL {
		t.Errorf("Expected ErrInvalidTTL, got %v", err)
	}
}

func TestCache_BatchOperations(t *testing.T) {
	c, err := New("batch-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Test SetMulti
	items := map[interface{}]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	err = c.SetMulti(items, 0)
	if err != nil {
		t.Errorf("Failed to set multiple values: %v", err)
	}

	// Test GetMulti
	keys := []interface{}{"key1", "key2", "key3", "key4"}
	results := c.GetMulti(keys)
	
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
	
	for k, v := range items {
		if results[k] != v {
			t.Errorf("Expected %v for key %v, got %v", v, k, results[k])
		}
	}
	
	// key4 should not be in results
	if _, ok := results["key4"]; ok {
		t.Error("key4 should not be in results")
	}

	// Test DeleteMulti
	c.DeleteMulti([]interface{}{"key1", "key2"})
	
	if c.Has("key1") || c.Has("key2") {
		t.Error("key1 and key2 should be deleted")
	}
	
	if !c.Has("key3") {
		t.Error("key3 should still exist")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c, err := New("getorset-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	key := "lazy-key"
	expectedValue := "computed-value"
	callCount := 0
	
	// First call should compute the value
	value, err := c.GetOrSet(key, func() (interface{}, error) {
		callCount++
		return expectedValue, nil
	}, 0)
	
	if err != nil {
		t.Errorf("GetOrSet failed: %v", err)
	}
	
	if value != expectedValue {
		t.Errorf("Expected %v, got %v", expectedValue, value)
	}
	
	if callCount != 1 {
		t.Errorf("Expected function to be called once, called %d times", callCount)
	}

	// Second call should get from cache
	value, err = c.GetOrSet(key, func() (interface{}, error) {
		callCount++
		return "should-not-be-called", nil
	}, 0)
	
	if err != nil {
		t.Errorf("GetOrSet failed: %v", err)
	}
	
	if value != expectedValue {
		t.Errorf("Expected %v, got %v", expectedValue, value)
	}
	
	if callCount != 1 {
		t.Errorf("Expected function to be called once total, called %d times", callCount)
	}
}

func TestCache_GetOrSetContext(t *testing.T) {
	c, err := New("context-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	_, err = c.GetOrSetContext(ctx, "key", func(ctx context.Context) (interface{}, error) {
		return "value", nil
	}, 0)
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Test with valid context
	ctx = context.Background()
	value, err := c.GetOrSetContext(ctx, "key2", func(ctx context.Context) (interface{}, error) {
		return "value2", nil
	}, 0)
	
	if err != nil {
		t.Errorf("GetOrSetContext failed: %v", err)
	}
	
	if value != "value2" {
		t.Errorf("Expected value2, got %v", value)
	}
}

func TestCache_Clear(t *testing.T) {
	c, err := New("clear-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Add some items
	c.Set("key1", "value1", 0)
	c.Set("key2", "value2", 0)
	c.Set("key3", "value3", 0)

	// Clear cache
	c.Clear()

	// All keys should be gone
	if c.Has("key1") || c.Has("key2") || c.Has("key3") {
		t.Error("Cache should be empty after Clear")
	}
}

func TestCache_Concurrent(t *testing.T) {
	c, err := New("concurrent-test", DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	var wg sync.WaitGroup
	numGoroutines := 100
	
	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := id
			value := id * 2
			c.Set(key, value, 0)
		}(i)
	}
	
	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.Get(id)
		}(i)
	}
	
	wg.Wait()
	
	// Verify some values
	for i := 0; i < 10; i++ {
		if val, err := c.Get(i); err == nil {
			expected := i * 2
			if val != expected {
				t.Errorf("Expected %d, got %v", expected, val)
			}
		}
	}
}

func TestCache_SetWithCost(t *testing.T) {
	c, err := New("cost-test", &Options{
		NumCounters: 1e4,
		MaxCost:     1000, // Increased max cost to ensure items fit
		BufferItems: 64,
		Metrics:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Set items with different costs
	err = c.SetWithCost("small", "value", 1, 0)
	if err != nil {
		t.Errorf("Failed to set small item: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)
	
	if !c.Has("small") {
		t.Error("Small item should exist after setting")
	}

	err = c.SetWithCost("large", "value", 50, 0)
	if err != nil {
		t.Errorf("Failed to set large item: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Both should exist
	if !c.Has("large") {
		t.Error("Large item should exist after setting")
	}
}

func TestManager_Operations(t *testing.T) {
	manager := NewManager()
	defer manager.Close()

	// Create cache
	cache1, err := manager.Create("cache1", DefaultOptions())
	if err != nil {
		t.Errorf("Failed to create cache1: %v", err)
	}

	// Try to create duplicate
	_, err = manager.Create("cache1", DefaultOptions())
	if err == nil {
		t.Error("Should not allow duplicate cache names")
	}

	// Get cache
	c, exists := manager.Get("cache1")
	if !exists || c != cache1 {
		t.Error("Failed to get cache1")
	}

	// GetOrCreate existing
	c2, err := manager.GetOrCreate("cache1", DefaultOptions())
	if err != nil || c2 != cache1 {
		t.Error("GetOrCreate should return existing cache")
	}

	// GetOrCreate new
	cache2, err := manager.GetOrCreate("cache2", DefaultOptions())
	if err != nil || cache2 == nil {
		t.Error("Failed to create cache2")
	}

	// List caches
	names := manager.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 caches, got %d", len(names))
	}

	// Clear cache
	cache1.Set("key", "value", 0)
	err = manager.Clear("cache1")
	if err != nil {
		t.Errorf("Failed to clear cache1: %v", err)
	}
	if cache1.Has("key") {
		t.Error("Cache1 should be empty after clear")
	}

	// Delete cache
	err = manager.Delete("cache1")
	if err != nil {
		t.Errorf("Failed to delete cache1: %v", err)
	}
	
	_, exists = manager.Get("cache1")
	if exists {
		t.Error("cache1 should not exist after deletion")
	}

	// Delete non-existent cache
	err = manager.Delete("non-existent")
	if err == nil {
		t.Error("Should error when deleting non-existent cache")
	}
}

func TestBuilder(t *testing.T) {
	// Test basic builder
	c, err := NewBuilder("builder-test").
		WithMaxItems(1000).
		WithMaxMemory(1 << 20). // 1MB
		WithMetrics(true).
		Build()
	
	if err != nil {
		t.Fatalf("Failed to build cache: %v", err)
	}
	defer c.Close()

	if c.Name() != "builder-test" {
		t.Errorf("Expected name builder-test, got %s", c.Name())
	}

	// Test preset builders
	small, err := SmallCacheBuilder("small").Build()
	if err != nil {
		t.Errorf("Failed to build small cache: %v", err)
	}
	defer small.Close()

	medium, err := MediumCacheBuilder("medium").Build()
	if err != nil {
		t.Errorf("Failed to build medium cache: %v", err)
	}
	defer medium.Close()

	large, err := LargeCacheBuilder("large").Build()
	if err != nil {
		t.Errorf("Failed to build large cache: %v", err)
	}
	defer large.Close()
}

func BenchmarkCache_Set(b *testing.B) {
	c, _ := New("bench", DefaultOptions())
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(i, i, 0)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	c, _ := New("bench", DefaultOptions())
	defer c.Close()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		c.Set(i, i, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1000)
	}
}

func BenchmarkCache_SetWithTTL(b *testing.B) {
	c, _ := New("bench", DefaultOptions())
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(i, i, 5*time.Minute)
	}
}

func BenchmarkCache_Concurrent(b *testing.B) {
	c, _ := New("bench", DefaultOptions())
	defer c.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				c.Set(i, i, 0)
			} else {
				c.Get(i)
			}
			i++
		}
	})
}
