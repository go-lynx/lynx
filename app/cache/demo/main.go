package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-lynx/lynx/app/cache"
)

func main() {
	fmt.Println("=== Lynx Cache Demo ===\n")

	// 1. Create a simple cache
	fmt.Println("1. Creating a simple cache...")
	myCache, err := cache.QuickCache("demo-cache")
	if err != nil {
		log.Fatal(err)
	}
	defer myCache.Close()

	// 2. Basic Set and Get operations
	fmt.Println("\n2. Basic operations:")

	// Set some values with different TTLs
	myCache.Set("user:1", "Alice", 10*time.Second)
	myCache.Set("user:2", "Bob", 5*time.Second)
	myCache.Set("user:3", "Charlie", 0) // No expiration

	// Get values
	if val, err := myCache.Get("user:1"); err == nil {
		fmt.Printf("   Found user:1 = %v\n", val)
	}

	if val, err := myCache.Get("user:2"); err == nil {
		fmt.Printf("   Found user:2 = %v\n", val)
	}

	// 3. Check if key exists
	fmt.Println("\n3. Checking key existence:")
	fmt.Printf("   user:1 exists? %v\n", myCache.Has("user:1"))
	fmt.Printf("   user:999 exists? %v\n", myCache.Has("user:999"))

	// 4. Batch operations
	fmt.Println("\n4. Batch operations:")
	items := map[interface{}]interface{}{
		"product:1": "Laptop",
		"product:2": "Mouse",
		"product:3": "Keyboard",
	}
	myCache.SetMulti(items, 1*time.Minute)

	keys := []interface{}{"product:1", "product:2", "product:3", "product:4"}
	results := myCache.GetMulti(keys)
	for _, key := range keys {
		if val, ok := results[key]; ok {
			fmt.Printf("   %v = %v\n", key, val)
		} else {
			fmt.Printf("   %v = not found\n", key)
		}
	}

	// 5. GetOrSet pattern (lazy loading)
	fmt.Println("\n5. Lazy loading with GetOrSet:")

	start := time.Now()
	value, _ := myCache.GetOrSet("expensive-data", func() (interface{}, error) {
		// Simulate expensive operation
		time.Sleep(100 * time.Millisecond)
		return "computed result", nil
	}, 30*time.Second)
	fmt.Printf("   First call (computed): %v (took %v)\n", value, time.Since(start))

	start = time.Now()
	value, _ = myCache.GetOrSet("expensive-data", func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "this won't be called", nil
	}, 30*time.Second)
	fmt.Printf("   Second call (cached): %v (took %v)\n", value, time.Since(start))

	// 6. TTL demonstration
	fmt.Println("\n6. TTL demonstration:")
	myCache.Set("temp-key", "temporary value", 2*time.Second)
	fmt.Printf("   Before expiry: exists? %v\n", myCache.Has("temp-key"))

	fmt.Println("   Waiting 3 seconds...")
	time.Sleep(3 * time.Second)
	fmt.Printf("   After expiry: exists? %v\n", myCache.Has("temp-key"))

	// 7. Cache Manager - Multiple caches
	fmt.Println("\n7. Using Cache Manager for multiple caches:")

	userCache, _ := cache.Create("users", cache.DefaultOptions())
	sessionCache, _ := cache.SmallCache("sessions")
	apiCache, _ := cache.Create("api", &cache.Options{
		NumCounters: 1e5,
		MaxCost:     1 << 25, // 32MB
		BufferItems: 64,
		Metrics:     true,
	})

	userCache.Set("user:alice", map[string]string{"name": "Alice", "role": "admin"}, 0)
	sessionCache.Set("session:123", "active", 30*time.Minute)
	apiCache.Set("/api/users", []string{"Alice", "Bob", "Charlie"}, 5*time.Minute)

	fmt.Printf("   Active caches: %v\n", cache.List())

	// Get from specific cache
	if c, ok := cache.Get("users"); ok {
		if val, err := c.Get("user:alice"); err == nil {
			fmt.Printf("   Found in users cache: %v\n", val)
		}
	}

	// 8. Cache metrics
	fmt.Println("\n8. Cache Metrics:")

	// Generate some hits and misses
	for i := 0; i < 10; i++ {
		apiCache.Get("/api/users")   // hits
		apiCache.Get("/api/unknown") // misses
	}

	if metrics := apiCache.Metrics(); metrics != nil {
		fmt.Printf("   API Cache - Hits: %d, Misses: %d, Keys Added: %d\n",
			metrics.Hits(), metrics.Misses(), metrics.KeysAdded())

		if metrics.Hits() > 0 || metrics.Misses() > 0 {
			hitRatio := float64(metrics.Hits()) / float64(metrics.Hits()+metrics.Misses()) * 100
			fmt.Printf("   Hit Ratio: %.1f%%\n", hitRatio)
		}
	}

	// 9. Clear and cleanup
	fmt.Println("\n9. Cleanup:")
	fmt.Printf("   Clearing sessions cache...\n")
	cache.Clear("sessions")

	fmt.Printf("   Deleting api cache...\n")
	cache.Delete("api")

	fmt.Printf("   Remaining caches: %v\n", cache.List())

	fmt.Println("\n=== Demo Complete ===")
}
