# Redis Plugin Configuration Validation

This document describes the configuration validation functionality of the Redis plugin, including validation rules, usage methods, and best practices.

## Overview

The Redis plugin now includes complete configuration validation logic to ensure configuration correctness and reasonableness before plugin startup. Validation functionality includes:

- **Basic Connection Validation**: address format, network type, etc.
- **Connection Pool Configuration Validation**: connection count relationships, timeout times, etc.
- **Timeout Configuration Validation**: reasonableness and relationships of various timeout times
- **Retry Configuration Validation**: reasonableness of retry counts and backoff times
- **TLS Configuration Validation**: matching of TLS enablement and address format
- **Sentinel Configuration Validation**: necessary parameters for sentinel mode
- **Database Configuration Validation**: database number range
- **Client Name Validation**: name format and length

## Validation Rules Details

### 1. Basic Connection Validation

#### Address Format Validation
- Supports `redis://`, `rediss://` prefixes
- Validates host:port format
- Port range: 1-65535
- Empty addresses not allowed

```yaml
# ✅ Valid addresses
addrs: ["localhost:6379", "redis://127.0.0.1:6380", "rediss://secure.redis:6379"]

# ❌ Invalid addresses
addrs: ["invalid-address", ":6379", "localhost:", "localhost:99999"]
```

#### Network Type Validation
- Supported network types: `tcp`, `tcp4`, `tcp6`, `unix`, `unixpacket`
- Default is `tcp`

### 2. Connection Pool Configuration Validation

#### Connection Count Relationships
- `MinIdleConns` ≥ 0
- `MaxActiveConns` > 0
- `MinIdleConns` ≤ `MaxActiveConns`

```yaml
# ✅ Valid configuration
min_idle_conns: 10
max_active_conns: 20

# ❌ Invalid configuration
min_idle_conns: 30
max_active_conns: 20  # Minimum cannot be greater than maximum
```

#### Connection Lifecycle
- `ConnMaxIdleTime`: 0-24 hours
- `MaxConnAge`: 0-7 days
- `PoolTimeout`: 0-30 seconds

### 3. Timeout Configuration Validation

#### Timeout Time Ranges
- `DialTimeout`: 0-60 seconds
- `ReadTimeout`: 0-5 minutes
- `WriteTimeout`: 0-5 minutes

#### Timeout Time Relationships
- `DialTimeout` ≤ `ReadTimeout` (recommended)

```yaml
# ✅ Valid configuration
dial_timeout: { seconds: 5 }
read_timeout: { seconds: 10 }

# ❌ Invalid configuration
dial_timeout: { seconds: 10 }
read_timeout: { seconds: 5 }  # Connection timeout should not be greater than read timeout
```

### 4. Retry Configuration Validation

#### Retry Count
- `MaxRetries`: 0-10

#### Backoff Time
- `MinRetryBackoff`: 0-1 second
- `MaxRetryBackoff`: 0-30 seconds
- `MinRetryBackoff` ≤ `MaxRetryBackoff`

### 5. TLS Configuration Validation

- If TLS is enabled, it's recommended to use `rediss://` prefix
- Supports `tls.enabled` and `tls.insecure_skip_verify` configuration

### 6. Sentinel Configuration Validation

- `master_name` must be provided when Sentinel mode is enabled
- Sentinel address format validation

### 7. Database Configuration Validation

- Database number range: 0-15 (Redis default limit)

### 8. Client Name Validation

- Length limit: ≤ 64 characters
- Character restrictions: only letters, numbers, underscores, and hyphens allowed

## Usage Methods

### 1. Automatic Validation (Recommended)

Configuration validation is automatically executed during plugin initialization:

```go
// Automatically called in InitializeResources
if err := ValidateAndSetDefaults(r.conf); err != nil {
    return fmt.Errorf("redis configuration validation failed: %w", err)
}
```

### 2. Manual Validation

If manual configuration validation is needed:

```go
import "github.com/go-lynx/lynx/plugins/nosql/redis"

// Validate configuration
result := redis.ValidateRedisConfig(config)
if !result.IsValid {
    log.Errorf("Configuration validation failed: %s", result.Error())
    return
}

// Validate and set default values
if err := redis.ValidateAndSetDefaults(config); err != nil {
    log.Errorf("Configuration validation failed: %v", err)
    return
}
```

### 3. Get Validation Error Details

```go
result := redis.ValidateRedisConfig(config)
if !result.IsValid {
    for _, err := range result.Errors {
        log.Errorf("Field: %s, Error: %s", err.Field, err.Message)
    }
}
```

## Default Value Settings

If configuration validation passes, the system will automatically set reasonable default values:

```go
// Network type
Network: "tcp"

// Connection pool
MinIdleConns: 10
MaxIdleConns: 20
MaxActiveConns: 20

// Timeout times
DialTimeout: 10s
ReadTimeout: 10s
WriteTimeout: 10s
PoolTimeout: 3s

// Connection lifecycle
ConnMaxIdleTime: 10s
MaxConnAge: 30m

// Retry configuration
MaxRetries: 3
MinRetryBackoff: 8ms
MaxRetryBackoff: 512ms
```

## Error Handling

### Validation Error Types

```go
type ValidationError struct {
    Field   string  // Field name with error
    Message string  // Error description
}
```

### Validation Results

```go
type ValidationResult struct {
    IsValid bool              // Whether validation passed
    Errors  []ValidationError // Error list
}
```

## Best Practices

### 1. Configuration Templates

```yaml
# Production environment configuration template
redis:
  network: tcp
  addrs: ["redis-master:6379", "redis-slave:6379"]
  min_idle_conns: 20
  max_active_conns: 100
  dial_timeout: { seconds: 5 }
  read_timeout: { seconds: 10 }
  write_timeout: { seconds: 10 }
  pool_timeout: { seconds: 2 }
  max_retries: 3
  client_name: "myapp-prod"
```

### 2. Development Environment Configuration

```yaml
# Development environment configuration
redis:
  addrs: ["localhost:6379"]
  min_idle_conns: 5
  max_active_conns: 20
  dial_timeout: { seconds: 2 }
  read_timeout: { seconds: 5 }
  write_timeout: { seconds: 5 }
  client_name: "myapp-dev"
```

### 3. Sentinel Mode Configuration

```yaml
redis:
  addrs: ["sentinel1:26379", "sentinel2:26379"]
  sentinel:
    master_name: "mymaster"
  min_idle_conns: 10
  max_active_conns: 50
  pool_timeout: { seconds: 1 }
```

### 4. TLS Configuration

```yaml
redis:
  addrs: ["rediss://secure.redis:6379"]
  tls:
    enabled: true
    insecure_skip_verify: false  # Should be false in production
  min_idle_conns: 10
  max_active_conns: 20
```

## Troubleshooting

### Common Validation Errors

1. **Address Format Error**
   ```
   validation error in field 'addrs[0]': invalid address format: address localhost: invalid port
   ```

2. **Connection Pool Configuration Error**
   ```
   validation error in field 'min_idle_conns': cannot be greater than max_active_conns
   ```

3. **Timeout Configuration Error**
   ```
   validation error in field 'dial_timeout': should not be greater than read_timeout
   ```

4. **Database Number Error**
   ```
   validation error in field 'db': database number cannot exceed 15 (Redis default limit)
   ```

### Debugging Suggestions

1. Use `ValidateRedisConfig()` for pre-validation
2. Check YAML syntax in configuration files
3. Validate address format and port numbers
4. Confirm connection pool parameter relationships
5. Check timeout time settings

## Testing

Run configuration validation tests:

```bash
cd lynx/plugins/nosql/redis
go test -v -run TestValidateRedisConfig
```

Tests cover various configuration scenarios, including:
- Valid configuration validation
- Invalid configuration detection
- Boundary condition testing
- Error message format validation
