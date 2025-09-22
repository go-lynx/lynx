package cache_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-lynx/lynx/app/cache"
)

func ExampleCache_basic() {
	// Create a simple cache
	c, err := cache.QuickCache("example")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Set a value with 5 minute TTL
	err = c.Set("user:123", "John Doe", 5*time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	// Get the value
	value, err := c.Get("user:123")
	if err != nil {
		fmt.Println("Not found")
	} else {
		fmt.Println(value)
	}

	// Output: John Doe
}

func ExampleCache_GetOrSet() {
	c, err := cache.QuickCache("lazy-load")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// This function is only called if the key doesn't exist
	value, err := c.GetOrSet("expensive-data", func() (interface{}, error) {
		// Simulate expensive operation
		time.Sleep(100 * time.Millisecond)
		return "computed value", nil
	}, 10*time.Minute)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)

	// Second call will get from cache (no sleep)
	value, _ = c.GetOrSet("expensive-data", func() (interface{}, error) {
		return "this won't be called", nil
	}, 10*time.Minute)
	fmt.Println(value)

	// Output:
	// computed value
	// computed value
}

func ExampleCache_batch() {
	c, err := cache.QuickCache("batch-example")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Set multiple values at once
	items := map[interface{}]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	err = c.SetMulti(items, 5*time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	// Get multiple values at once
	keys := []interface{}{"key1", "key2", "key3", "key4"}
	values := c.GetMulti(keys)

	for _, key := range keys {
		if val, ok := values[key]; ok {
			fmt.Printf("%v: %v\n", key, val)
		} else {
			fmt.Printf("%v: not found\n", key)
		}
	}

	// Output:
	// key1: value1
	// key2: value2
	// key3: value3
	// key4: not found
}

func ExampleBuilder() {
	// Build a custom cache with specific settings
	c, err := cache.NewBuilder("custom").
		WithMaxItems(1000).     // Maximum 1000 items
		WithMaxMemory(1 << 26). // 64MB memory limit
		WithMetrics(true).      // Enable metrics
		WithEvictionCallback(func(item *ristretto.Item) {
			fmt.Printf("Evicted key: %v\n", item.Key)
		}).
		Build()

	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Use the cache
	c.Set("test", "value", 1*time.Minute)
}

func ExampleManager() {
	// Create different caches for different purposes
	userCache, err := cache.Create("users", cache.DefaultOptions())
	if err != nil {
		log.Fatal(err)
	}

	sessionCache, err := cache.SessionCacheBuilder("sessions", 30*time.Minute).
		BuildAndRegister()
	if err != nil {
		log.Fatal(err)
	}

	apiCache, err := cache.APICacheBuilder("api-responses").
		BuildAndRegister()
	if err != nil {
		log.Fatal(err)
	}

	// Use the caches
	userCache.Set("user:1", "Alice", 10*time.Minute)
	sessionCache.Set("session:xyz", "session-data", 30*time.Minute)
	apiCache.Set("/api/users", "cached-response", 1*time.Minute)

	// List all caches
	fmt.Println("Active caches:", cache.List())

	// Get statistics
	stats := cache.Stats()
	for name, metrics := range stats {
		if metrics != nil {
			fmt.Printf("Cache %s - Hits: %d, Misses: %d\n",
				name, metrics.Hits(), metrics.Misses())
		}
	}

	// Clean up
	cache.Close()
}

func ExampleCache_concurrent() {
	c, err := cache.QuickCache("concurrent")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key:%d", id)
			value := fmt.Sprintf("value:%d", id)
			c.Set(key, value, 5*time.Minute)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key:%d", id)
			c.Get(key)
		}(i)
	}

	wg.Wait()
	fmt.Println("Concurrent operations completed")

	// Output: Concurrent operations completed
}

func ExampleCache_GetOrSetContext() {
	c, err := cache.QuickCache("context-example")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Fetch with context support
	value, err := c.GetOrSetContext(ctx, "api-data",
		func(ctx context.Context) (interface{}, error) {
			// Simulate API call
			select {
			case <-time.After(1 * time.Second):
				return "api response", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}, 5*time.Minute)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)

	// Output: api response
}
