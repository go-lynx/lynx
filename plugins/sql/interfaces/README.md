# SQL Interfaces Plugin for Lynx Framework

The SQL Interfaces plugin defines common interfaces and types for all SQL database plugins in the Lynx framework. It provides standardized contracts that ensure consistency across MySQL, PostgreSQL, and MSSQL plugins.

## Features

### Core Interfaces
- **Database Interface**: Common database operations interface
- **Connection Interface**: Standardized connection management
- **Transaction Interface**: Transaction handling interface
- **Health Check Interface**: Health monitoring interface
- **Metrics Interface**: Metrics collection interface

### Type Definitions
- **Error Types**: Standardized error type definitions
- **Configuration Types**: Common configuration structures
- **Health Status Types**: Health check result types
- **Metrics Types**: Metrics collection types

### Common Contracts
- **Plugin Interface**: Standard plugin implementation contract
- **Lifecycle Interface**: Plugin lifecycle management
- **Configuration Interface**: Configuration validation contract

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  SQL Interfaces Plugin                     │
├─────────────────────────────────────────────────────────────┤
│  Database Interface  │  Connection Interface  │  Transaction │
├─────────────────────────────────────────────────────────────┤
│  Health Interface    │  Metrics Interface     │  Error Types │
├─────────────────────────────────────────────────────────────┤
│              Common Type Definitions                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              SQL Plugin Implementations                     │
│  MySQL Plugin  │  PostgreSQL Plugin  │  MSSQL Plugin      │
└─────────────────────────────────────────────────────────────┘
```

## Installation

The SQL Interfaces plugin is automatically included when you import any SQL plugin:

```go
import _ "github.com/go-lynx/lynx/plugins/sql/mysql"     // Includes interfaces
import _ "github.com/go-lynx/lynx/plugins/sql/pgsql"     // Includes interfaces
import _ "github.com/go-lynx/lynx/plugins/sql/mssql"     // Includes interfaces
```

## Core Interfaces

### Database Interface

```go
type Database interface {
    // Connection management
    Connect() error
    Disconnect() error
    Ping() error
    
    // Query operations
    Query(query string, args ...interface{}) (Rows, error)
    QueryRow(query string, args ...interface{}) Row
    Exec(query string, args ...interface{}) (Result, error)
    
    // Transaction operations
    Begin() (Transaction, error)
    BeginTx(ctx context.Context, opts *TxOptions) (Transaction, error)
    
    // Health and metrics
    Health() HealthStatus
    Metrics() Metrics
}
```

### Connection Interface

```go
type Connection interface {
    // Connection lifecycle
    Open() error
    Close() error
    IsOpen() bool
    
    // Connection properties
    GetID() string
    GetDatabase() string
    GetDriver() string
    
    // Connection health
    Ping() error
    IsHealthy() bool
}
```

### Transaction Interface

```go
type Transaction interface {
    // Transaction operations
    Commit() error
    Rollback() error
    
    // Query operations within transaction
    Query(query string, args ...interface{}) (Rows, error)
    QueryRow(query string, args ...interface{}) Row
    Exec(query string, args ...interface{}) (Result, error)
    
    // Transaction state
    IsActive() bool
    GetID() string
}
```

### Health Check Interface

```go
type HealthChecker interface {
    // Health checking
    CheckHealth() HealthStatus
    CheckConnection() error
    CheckQuery() error
    
    // Health monitoring
    StartMonitoring(interval time.Duration)
    StopMonitoring()
    IsMonitoring() bool
}
```

### Metrics Interface

```go
type MetricsCollector interface {
    // Metrics collection
    RecordQuery(duration time.Duration, success bool)
    RecordConnection(active bool)
    RecordError(errorType ErrorType)
    
    // Metrics retrieval
    GetQueryMetrics() QueryMetrics
    GetConnectionMetrics() ConnectionMetrics
    GetErrorMetrics() ErrorMetrics
}
```

## Type Definitions

### Error Types

```go
type ErrorType string

const (
    ErrConnectionFailed     ErrorType = "connection_failed"
    ErrQueryTimeout        ErrorType = "query_timeout"
    ErrConstraintViolation ErrorType = "constraint_violation"
    ErrTransactionFailed   ErrorType = "transaction_failed"
    ErrConfigurationError  ErrorType = "configuration_error"
    ErrUnknown             ErrorType = "unknown"
)

type DatabaseError struct {
    Type    ErrorType
    Message string
    Cause   error
    Query   string
    Args    []interface{}
}
```

### Health Status Types

```go
type HealthStatus struct {
    IsHealthy bool
    Message   string
    Timestamp time.Time
    Duration  time.Duration
    Details   map[string]interface{}
}

type HealthCheckResult struct {
    Database    string
    Status      HealthStatus
    Checks      []HealthCheck
    LastChecked time.Time
}

type HealthCheck struct {
    Name      string
    Status    bool
    Message   string
    Duration  time.Duration
    Timestamp time.Time
}
```

### Metrics Types

```go
type QueryMetrics struct {
    TotalQueries    int64
    SuccessfulQueries int64
    FailedQueries   int64
    AverageDuration time.Duration
    MaxDuration     time.Duration
    MinDuration     time.Duration
}

type ConnectionMetrics struct {
    TotalConnections    int64
    ActiveConnections   int64
    IdleConnections     int64
    FailedConnections   int64
    ConnectionErrors    int64
}

type ErrorMetrics struct {
    TotalErrors     int64
    ErrorCounts     map[ErrorType]int64
    LastError       time.Time
    ErrorRate       float64
}
```

### Configuration Types

```go
type DatabaseConfig struct {
    Driver          string
    Host            string
    Port            int
    Database        string
    Username        string
    Password        string
    SSLMode         string
    ConnectTimeout  time.Duration
    QueryTimeout    time.Duration
    MaxConnections  int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
}

type TxOptions struct {
    Isolation IsolationLevel
    ReadOnly  bool
}

type IsolationLevel int

const (
    LevelDefault IsolationLevel = iota
    LevelReadUncommitted
    LevelReadCommitted
    LevelRepeatableRead
    LevelSerializable
)
```

## Usage Examples

### Basic Database Operations

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    // Get database instance (implementation specific)
    db := getDatabaseInstance()
    
    // Basic query
    rows, err := db.Query("SELECT * FROM users WHERE age > ?", 18)
    if err != nil {
        log.Errorf("Query failed: %v", err)
        return
    }
    defer rows.Close()
    
    // Process results
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Age); err != nil {
            log.Errorf("Scan failed: %v", err)
            return
        }
        log.Infof("User: %+v", user)
    }
}
```

### Transaction Management

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    db := getDatabaseInstance()
    
    // Begin transaction
    tx, err := db.Begin()
    if err != nil {
        log.Errorf("Failed to begin transaction: %v", err)
        return
    }
    defer tx.Rollback() // Ensure rollback on error
    
    // Execute operations within transaction
    _, err = tx.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "John", 30)
    if err != nil {
        log.Errorf("Insert failed: %v", err)
        return
    }
    
    _, err = tx.Exec("UPDATE accounts SET balance = balance - ? WHERE user_id = ?", 100, 1)
    if err != nil {
        log.Errorf("Update failed: %v", err)
        return
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        log.Errorf("Commit failed: %v", err)
        return
    }
    
    log.Info("Transaction completed successfully")
}
```

### Health Checking

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    db := getDatabaseInstance()
    
    // Check database health
    health := db.Health()
    if !health.IsHealthy {
        log.Errorf("Database is unhealthy: %s", health.Message)
        return
    }
    
    log.Infof("Database is healthy: %s", health.Message)
    
    // Get detailed health information
    if details, ok := health.Details["connection_pool"]; ok {
        log.Infof("Connection pool status: %+v", details)
    }
}
```

### Metrics Collection

```go
package main

import (
    "time"
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    db := getDatabaseInstance()
    
    // Get metrics
    metrics := db.Metrics()
    
    // Get query metrics
    queryMetrics := metrics.GetQueryMetrics()
    log.Infof("Total queries: %d, Success rate: %.2f%%", 
        queryMetrics.TotalQueries, 
        float64(queryMetrics.SuccessfulQueries)/float64(queryMetrics.TotalQueries)*100)
    
    // Get connection metrics
    connMetrics := metrics.GetConnectionMetrics()
    log.Infof("Active connections: %d, Idle connections: %d", 
        connMetrics.ActiveConnections, 
        connMetrics.IdleConnections)
    
    // Get error metrics
    errorMetrics := metrics.GetErrorMetrics()
    log.Infof("Total errors: %d, Error rate: %.2f%%", 
        errorMetrics.TotalErrors, 
        errorMetrics.ErrorRate)
}
```

## Error Handling

### Error Categorization

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    err := performDatabaseOperation()
    if err != nil {
        // Categorize error
        var dbErr *interfaces.DatabaseError
        if errors.As(err, &dbErr) {
            switch dbErr.Type {
            case interfaces.ErrConnectionFailed:
                log.Errorf("Connection failed: %v", dbErr)
            case interfaces.ErrQueryTimeout:
                log.Errorf("Query timeout: %v", dbErr)
            case interfaces.ErrConstraintViolation:
                log.Errorf("Constraint violation: %v", dbErr)
            default:
                log.Errorf("Unknown error: %v", dbErr)
            }
        } else {
            log.Errorf("Non-database error: %v", err)
        }
    }
}
```

### Error Recovery

```go
package main

import (
    "time"
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    maxRetries := 3
    retryDelay := time.Second
    
    for i := 0; i < maxRetries; i++ {
        err := performDatabaseOperation()
        if err == nil {
            break // Success
        }
        
        var dbErr *interfaces.DatabaseError
        if errors.As(err, &dbErr) {
            switch dbErr.Type {
            case interfaces.ErrConnectionFailed:
                // Retry connection
                time.Sleep(retryDelay)
                retryDelay *= 2
                continue
            case interfaces.ErrQueryTimeout:
                // Retry query
                time.Sleep(retryDelay)
                continue
            default:
                // Don't retry other errors
                log.Errorf("Non-retryable error: %v", dbErr)
                return
            }
        } else {
            log.Errorf("Non-database error: %v", err)
            return
        }
    }
}
```

## Best Practices

### 1. Interface Implementation
- Implement all required interfaces
- Provide meaningful error messages
- Handle edge cases gracefully

### 2. Error Handling
- Use typed errors for better error handling
- Provide context in error messages
- Implement proper error recovery

### 3. Health Monitoring
- Implement comprehensive health checks
- Provide detailed health information
- Monitor health metrics

### 4. Metrics Collection
- Collect relevant metrics
- Use appropriate metric types
- Monitor metric trends

## Dependencies

- `github.com/go-lynx/lynx`: Lynx framework core
- `github.com/go-kratos/kratos/v2`: Kratos framework

## License

Apache License 2.0

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Framework Docs](https://github.com/go-lynx/lynx/docs)
