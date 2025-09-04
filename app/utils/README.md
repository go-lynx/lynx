# Lynx Utility Packages (app/util)

The `app/util` directory contains a collection of general-purpose utility packages for the Lynx framework, providing various helper functions to help developers build microservice applications more efficiently.

## 📦 Package Overview

| Package | Description | Main Purpose |
|---------|-------------|--------------|
| **auth** | Authentication & Encryption | Password hashing, JWT token generation & validation |
| **cast** | Type Conversion | Safe type conversions with default values |
| **collection** | Collection Operations | Functional operations like Map/Filter/Unique |
| **ctxx** | Context Enhancement | Type-safe Context operations |
| **envx** | Environment Variables | Convenient env var reading & parsing |
| **errx** | Error Handling | Error aggregation, wrapping, recovery |
| **fsx** | File System | Atomic writes, limited reads, file operations |
| **idx** | ID Generation | NanoID and unique identifier generation |
| **netx** | Network Utilities | Port waiting, network error detection |
| **ptr** | Pointer Operations | Pointer creation & dereferencing helpers |
| **randx** | Random Generation | Cryptographically secure random generation |
| **strx** | String Utilities | String truncation, compression, splitting |
| **timex** | Time Processing | Time alignment, jitter, interval checking |

## 🔧 Detailed Function Descriptions

### auth - Authentication & Encryption

Provides password hashing and JWT token processing functionality.

#### bcrypt Password Hashing
```go
import "github.com/go-lynx/lynx/app/util/auth"

// Generate password hash
hash, err := auth.HashPassword("mypassword", bcrypt.DefaultCost)

// Verify password
err := auth.VerifyPassword(hash, "mypassword")

// Boolean verification
isValid := auth.CheckPassword(hash, "mypassword")
```

#### JWT Token Processing
```go
import "github.com/go-lynx/lynx/app/util/auth/jwt"

// Custom Claims
type MyClaims struct {
    jwt.RegisteredClaims
    UserID string `json:"user_id"`
}

// Sign and generate token
token, err := jwt.Sign(&myClaims, "ES256", privateKey)

// Verify token
ok, err := jwt.Verify(token, &myClaims, publicKey)
```

### cast - Type Conversion

Provides safe type conversions with default value fallback.

```go
import "github.com/go-lynx/lynx/app/util/cast"

// Convert to integer
val, err := cast.ToInt("123")
valWithDefault := cast.ToIntDefault("invalid", 0)

// Convert to boolean
boolVal, err := cast.ToBool("true")
boolWithDefault := cast.ToBoolDefault("invalid", false)

// Convert to float
floatVal, err := cast.ToFloat64("3.14")
floatWithDefault := cast.ToFloat64Default("invalid", 0.0)

// Convert to duration
duration, err := cast.ToDuration("5s")
durationWithDefault := cast.ToDurationDefault("invalid", time.Second)
```

### collection - Collection Operations

Provides functional programming style collection operations.

```go
import "github.com/go-lynx/lynx/app/util/collection"

// Map transformation
numbers := []int{1, 2, 3}
doubled := collection.Map(numbers, func(n int) int { return n * 2 })

// Filter
filtered := collection.Filter(numbers, func(n int) bool { return n > 1 })

// Unique - remove duplicates
unique := collection.Unique([]int{1, 2, 2, 3, 3})

// Chunk - split into chunks
chunks := collection.Chunk([]int{1, 2, 3, 4, 5}, 2)

// GroupBy
type User struct{ ID int; Name string }
users := []User{{1, "Alice"}, {2, "Bob"}, {1, "Charlie"}}
grouped := collection.GroupBy(users, func(u User) int { return u.ID })

// Set operations
set := collection.NewSet[string]()
set.Add("apple")
set.Add("banana")
exists := set.Contains("apple")

// Map operations
m := collection.NewMapX[string, int]()
m.Set("key", 100)
val, ok := m.Get("key")
```

### ctxx - Context Enhancement

Provides type-safe Context operations.

```go
import "github.com/go-lynx/lynx/app/util/ctxx"

// Create Context with timeout
ctx, cancel := ctxx.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Type-safe value retrieval
type UserKey struct{}
ctx = context.WithValue(ctx, UserKey{}, "user123")
userID, ok := ctxx.Value[string](ctx, UserKey{})

// Detach Context (preserve values but remove cancellation/deadline)
detachedCtx := ctxx.Detach(ctx)
```

### envx - Environment Variables

Convenient environment variable reading and parsing.

```go
import "github.com/go-lynx/lynx/app/util/envx"

// Read string
host := envx.Get("DB_HOST", "localhost")

// Read integer
port := envx.GetInt("DB_PORT", 5432)

// Read boolean
debug := envx.GetBool("DEBUG", false)

// Read duration
timeout := envx.GetDuration("TIMEOUT", 30*time.Second)
```

### errx - Error Handling

Provides error aggregation, wrapping, and recovery handling.

```go
import "github.com/go-lynx/lynx/app/util/errx"

// Aggregate multiple errors
err := errx.All(err1, err2, err3)

// Get first non-nil error
firstErr := errx.First(err1, err2, err3)

// Wrap error with context
wrappedErr := errx.Wrap(err, "failed to process")

// Deferred recovery handling
defer errx.DeferRecover(func(e any) {
    log.Printf("recovered from panic: %v", e)
})
```

### fsx - File System

Provides enhanced file operation functionality.

```go
import "github.com/go-lynx/lynx/app/util/fsx"

// Check if file exists
exists, err := fsx.Exists("/path/to/file")

// Create directories and write file
err := fsx.WriteFileMkdirAll("/path/to/file.txt", data, 0644)

// Size-limited file reading (prevent OOM)
data, err := fsx.ReadFileLimit("/path/to/file", 10*1024*1024) // 10MB

// Atomic write (write to temp file then rename)
err := fsx.AtomicWrite("/path/to/file", data, 0644)
```

### idx - ID Generation

Generate URL-safe unique identifiers.

```go
import "github.com/go-lynx/lynx/app/util/idx"

// Generate NanoID with specified length
id, err := idx.NanoID(21)

// Generate NanoID with default length (21)
defaultID, err := idx.DefaultNanoID()
```

### netx - Network Utilities

Network-related helper functions.

```go
import "github.com/go-lynx/lynx/app/util/netx"

// Check if temporary network error
isTemp := netx.IsTemporary(err)

// Check if timeout error
isTimeout := netx.IsTimeout(err)

// Wait for port availability
err := netx.WaitPort("localhost:8080", 30*time.Second)
```

### ptr - Pointer Operations

Simplify pointer creation and dereferencing.

```go
import "github.com/go-lynx/lynx/app/util/ptr"

// Create pointer
strPtr := ptr.Ptr("hello")
intPtr := ptr.Ptr(42)

// Safe dereference (return default if nil)
value := ptr.Deref(strPtr, "default")

// Return default for zero value
result := ptr.OrDefault("", "default")
```

### randx - Random Generation

Cryptographically secure random generation.

```go
import "github.com/go-lynx/lynx/app/util/randx"

// Generate random bytes
bytes, err := randx.CryptoBytes(32)

// Generate random string (using default alphabet)
str, err := randx.RandString(16, "")

// Use custom alphabet
hexStr, err := randx.RandString(32, "0123456789abcdef")
```

### strx - String Utilities

Provides enhanced string processing functionality.

```go
import "github.com/go-lynx/lynx/app/util/strx"

// Check multiple prefixes
hasPrefix := strx.HasPrefixAny("hello world", "hi", "hello", "hey")

// Check multiple suffixes
hasSuffix := strx.HasSuffixAny("file.txt", ".jpg", ".txt", ".png")

// Safe truncation (Unicode-aware)
truncated := strx.Truncate("This is a very long string", 10, "...")

// Compress whitespace
compressed := strx.TrimSpaceAndCompress("  hello   world  ")

// Split and trim
parts := strx.SplitAndTrim("a, b, , c", ",")
```

### timex - Time Processing

Time-related helper functions.

```go
import "github.com/go-lynx/lynx/app/util/timex"

// Get UTC time
now := timex.NowUTC()

// Parse time with multiple format attempts
layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"}
t, err := timex.ParseAny(layouts, "2024-01-01")

// Time alignment (floor to nearest interval)
aligned := timex.Align(time.Now(), 5*time.Minute)

// Add random jitter (to avoid thundering herd)
jittered := timex.Jitter(10*time.Second, 0.3) // 10s ± 30%

// Check if time is within interval
isWithin := timex.Within(time.Now(), startTime, endTime)
```

## 🎯 Best Practices

### 1. Error Handling
- Use `errx` package to aggregate and wrap errors for better error context
- Use `DeferRecover` at critical points to prevent program crashes from panics

### 2. Type Conversion
- Prefer using `cast` package for type conversions to avoid panics
- Use functions with default values for uncertain inputs

### 3. File Operations
- Use `AtomicWrite` to ensure atomic file writes
- Use `ReadFileLimit` to prevent OOM from reading large files

### 4. Random Generation
- Use `randx` package for cryptographically secure random numbers
- Use `idx` package for ID generation to ensure uniqueness and URL safety

### 5. Collection Operations
- Use functional operations from `collection` package for cleaner, more readable code
- Generic support ensures type safety

## 📝 Important Notes

1. **Performance Considerations**: Some utility functions may have performance overhead; evaluate in high-frequency scenarios
2. **Concurrency Safety**: Most utility functions are stateless and naturally concurrent-safe
3. **Error Handling**: Always check returned errors; ensure reasonable defaults when using default value functions
4. **Memory Management**: Set reasonable limits when using file operation utilities

## 🔗 Related Links

- [Lynx Framework Documentation](https://go-lynx.cn/docs)
- [Example Code](https://github.com/go-lynx/lynx/tree/main/examples)
- [API Reference](https://pkg.go.dev/github.com/go-lynx/lynx/app/util)

## 📄 License

This project is licensed under the Apache License 2.0.
