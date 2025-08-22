# Lynx Cache Package

High-performance in-memory caching solution based on [Ristretto](https://github.com/dgraph-io/ristretto), providing thread-safe caching with automatic memory management and TTL support.

## Features

- **High Performance**: Built on Ristretto, one of the fastest caching libraries in Go
- **Thread-Safe**: All operations are concurrent-safe
- **TTL Support**: Set expiration time for cached items
- **Memory Management**: Automatic eviction based on cost and frequency
- **Multiple Cache Instances**: Manage multiple named caches through Manager
- **Fluent Builder API**: Easy cache configuration with Builder pattern
- **Metrics Support**: Built-in metrics collection for monitoring
- **Batch Operations**: Support for bulk get/set/delete operations

## Installation

```bash
go get github.com/dgraph-io/ristretto
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    "github.com/go-lynx/lynx/app/cache"
)

func main() {
    // Create a simple cache
    c, err := cache.QuickCache("my-cache")
    if err != nil {
        panic(err)
    }
    defer c.Close()

    // Set a value with TTL
    err = c.Set("key1", "value1", 5*time.Minute)
    if err != nil {
        panic(err)
    }

    // Get a value
    value, err := c.Get("key1")
    if err != nil {
        fmt.Println("Key not found")
    } else {
        fmt.Println("Value:", value)
    }

    // Check if key exists
    if c.Has("key1") {
        fmt.Println("Key exists")
    }

    // Delete a key
    c.Delete("key1")
}
```

### Using Builder Pattern

```go
// Create a custom cache with builder
cache, err := cache.NewBuilder("api-cache").
    WithMaxItems(10000).           // Max 10,000 items
    WithMaxMemory(1 << 28).        // Max 256MB memory
    WithMetrics(true).             // Enable metrics
    WithEvictionCallback(func(item *ristretto.Item) {
        fmt.Printf("Evicted: %v\n", item.Key)
    }).
    Build()
```

### Using Cache Manager

```go
// Create caches through manager
userCache, err := cache.Create("users", cache.DefaultOptions())
if err != nil {
    panic(err)
}

// Get existing cache
if c, exists := cache.Get("users"); exists {
    c.Set("user:1", userData, 10*time.Minute)
}

// Get or create cache
sessionCache, err := cache.GetOrCreate("sessions", &cache.Options{
    NumCounters: 1e5,     // 100K sessions
    MaxCost:     1 << 26, // 64MB
    BufferItems: 64,
})

// List all caches
cacheNames := cache.List()
fmt.Println("Active caches:", cacheNames)

// Get statistics
stats := cache.Stats()
for name, metrics := range stats {
    fmt.Printf("Cache %s - Hits: %d, Misses: %d\n", 
        name, metrics.Hits, metrics.Misses)
}
```

## Advanced Features

### GetOrSet Pattern

```go
// Lazy loading with GetOrSet
value, err := cache.GetOrSet("expensive-key", func() (interface{}, error) {
    // This function is only called if key doesn't exist
    return expensiveOperation(), nil
}, 1*time.Hour)

// With context support
value, err := cache.GetOrSetContext(ctx, "api-key", 
    func(ctx context.Context) (interface{}, error) {
        return fetchFromAPI(ctx)
    }, 5*time.Minute)
```

### Batch Operations

```go
// Set multiple values
items := map[interface{}]interface{}{
    "key1": "value1",
    "key2": "value2",
    "key3": "value3",
}
err := cache.SetMulti(items, 10*time.Minute)

// Get multiple values
keys := []interface{}{"key1", "key2", "key3"}
values := cache.GetMulti(keys)
for k, v := range values {
    fmt.Printf("%v: %v\n", k, v)
}

// Delete multiple keys
cache.DeleteMulti(keys)
```

### Custom Cost Function

```go
cache, err := cache.NewBuilder("object-cache").
    WithMaxCost(1 << 30). // 1GB total cost
    WithCostFunction(func(value interface{}) int64 {
        // Calculate actual memory usage
        switch v := value.(type) {
        case string:
            return int64(len(v))
        case []byte:
            return int64(len(v))
        case User:
            return int64(unsafe.Sizeof(v)) + int64(len(v.Name))
        default:
            return 1
        }
    }).
    Build()
```

## Preset Configurations

The package provides several preset cache configurations:

### Small Cache
```go
cache, err := cache.SmallCache("small")
// 10K items, 16MB memory
```

### Medium Cache (Default)
```go
cache, err := cache.QuickCache("medium")
// 1M items, 256MB memory
```

### Large Cache
```go
cache, err := cache.LargeCache("large")
// 100M items, 4GB memory, metrics enabled
```

### Session Cache
```go
cache, err := cache.SessionCacheBuilder("sessions", 30*time.Minute).
    BuildAndRegister()
// Optimized for session storage with TTL
```

### API Cache
```go
cache, err := cache.APICacheBuilder("api").
    BuildAndRegister()
// Optimized for API response caching
```

## Best Practices

### 1. Choose Appropriate Size
- Set `NumCounters` to 10x your expected unique items
- Set `MaxCost` based on available memory

### 2. Use Cost Functions
- Implement custom cost functions for complex objects
- Accurately calculate memory usage for better eviction

### 3. Handle Cache Misses
```go
value, err := cache.Get("key")
if err == cache.ErrCacheMiss {
    // Handle cache miss
    value = loadFromDatabase()
    cache.Set("key", value, 10*time.Minute)
}
```

### 4. Monitor Metrics
```go
metrics := cache.Metrics()
hitRatio := float64(metrics.Hits) / float64(metrics.Hits + metrics.Misses)
fmt.Printf("Hit ratio: %.2f%%\n", hitRatio*100)
```

### 5. Graceful Shutdown
```go
defer cache.Close() // Close individual cache
defer cache.DefaultManager.Close() // Close all caches
```

## Performance Tips

1. **Buffer Size**: Increase `BufferItems` for high-concurrency scenarios
2. **Metrics**: Disable metrics in production if not needed
3. **TTL**: Use appropriate TTL to balance memory usage and cache effectiveness
4. **Batch Operations**: Use batch operations for multiple keys to reduce overhead

## Error Handling

```go
// Common errors
var (
    ErrCacheMiss  = errors.New("cache: key not found")
    ErrCacheSet   = errors.New("cache: failed to set value")
    ErrInvalidTTL = errors.New("cache: invalid TTL")
)

// Handle errors appropriately
if err := cache.Set("key", "value", -1*time.Second); err != nil {
    if err == cache.ErrInvalidTTL {
        // Handle invalid TTL
    }
}
```

## Thread Safety

All cache operations are thread-safe and can be called concurrently:

```go
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        cache.Set(fmt.Sprintf("key%d", id), id, 5*time.Minute)
    }(i)
}
wg.Wait()
```

## License

This package is part of the Lynx framework and is licensed under the Apache License 2.0.
