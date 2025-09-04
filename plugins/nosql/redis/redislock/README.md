# Redis Distributed Lock (Refactored Version)

Redis-based distributed lock implementation with domain-separated modular design.

## File Structure

```
redislock_v2/
├── errors.go      # Error definitions
├── types.go       # Type definitions and interfaces
├── scripts.go     # Lua scripts
├── utils.go       # Utility functions
├── manager.go     # Lock manager
├── lock.go        # Lock instance methods
├── api.go         # Public API
└── README.md      # Documentation
```


## Module Descriptions

### 1. errors.go - Error Definitions
- Centralized definition of all lock-related error types
- Facilitates error handling and internationalization

### 2. types.go - Type Definitions
- [LockOptions](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go#L13-L22): Lock configuration options
- [RetryStrategy](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go#L78-L81): Retry strategy
- [LockCallback](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go#L94-L100): Monitoring callback interface
- [RedisLock](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go#L112-L129): Lock instance structure
- [lockManager](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go#L146-L168): Lock manager structure
- Default configuration constants

### 3. scripts.go - Lua Scripts
- [lockScript](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/scripts.go#L105-L105): Script for acquiring locks
- [unlockScript](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/scripts.go#L106-L106): Script for releasing locks
- [renewScript](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/scripts.go#L107-L107): Script for renewing locks
- Support for reentrant lock extensions

### 4. utils.go - Utility Functions
- Lock value generation logic
- Process identifier initialization
- Hostname and IP retrieval

### 5. manager.go - Lock Manager
- Global lock manager instance
- Renewal service management
- Worker pool pattern
- Statistics collection
- Graceful shutdown

### 6. lock.go - Lock Instance Methods
- Lock status query methods
- Manual renewal and release
- Lock status checking

### 7. api.go - Public API
- Main lock operation interfaces
- Backward compatible API
- Error handling and callbacks

## Design Advantages

### 1. **Modular Design**
- Each file has a single responsibility, making maintenance easier
- Better code reusability
- Easier testing

### 2. **Clear Dependencies**
```
api.go → manager.go → lock.go
    ↓         ↓         ↓
types.go → scripts.go → utils.go
    ↓
errors.go
```


### 3. **Easy to Extend**
- New features only require modification of corresponding modules
- Does not affect other modules
- Facilitates adding new features

### 4. **Better Readability**
- Clearer code organization
- Moderate file sizes
- Facilitates team collaboration

## Usage

Usage is identical to the original version, just with a more modular internal structure:

```go
import "github.com/go-lynx/lynx/plugins/nosql/redis/redislock"

// Basic usage
err := redislock.Lock(context.Background(), "my-lock", 30*time.Second, func() error {
    // Business logic
    return nil
})

// Usage with configuration
options := redislock.LockOptions{
    Expiration:       60 * time.Second,
    RetryStrategy:    redislock.DefaultRetryStrategy,
    RenewalEnabled:   true,
    RenewalThreshold: 0.5,
}

err := redislock.LockWithOptions(context.Background(), "my-lock", options, func() error {
    // Business logic
    return nil
})
```


## Migration Guide

Migrating from the original version to the refactored version:

1. **Import path remains unchanged**
2. **API interfaces are fully compatible**
3. **Configuration options remain consistent**
4. **Error handling approach is the same**

## Performance Optimizations

The refactored version maintains all performance optimizations:

- ✅ Worker pool pattern to limit concurrency
- ✅ Intelligent renewal checking
- ✅ Exponential backoff retry
- ✅ High-frequency check response
- ✅ Atomic operation statistics

## Maintenance Recommendations

1. **Error Handling**: Manage errors centrally in [errors.go](file:///Users/claire/GolandProjects/lynx/lynx/plugins/errors.go)
2. **Type Extensions**: Add new type definitions in [types.go](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/types.go)
3. **Script Optimization**: Optimize Lua scripts in [scripts.go](file:///Users/claire/GolandProjects/lynx/lynx/plugins/nosql/redis/redislock/scripts.go)
4. **Feature Enhancement**: Add new features in corresponding modules
5. **Test Coverage**: Write unit tests for each module