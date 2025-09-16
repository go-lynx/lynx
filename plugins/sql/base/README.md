# SQL Base Plugin for Lynx Framework

The SQL Base plugin provides common functionality and interfaces for all SQL database plugins in the Lynx framework. It serves as the foundation for MySQL, PostgreSQL, and MSSQL plugins, offering shared features like health checking, metrics collection, and common database operations.

## Features

### Core Functionality
- **Health Checking**: Comprehensive database health monitoring
- **Metrics Collection**: Prometheus metrics for database operations
- **Connection Management**: Common connection handling patterns
- **Error Handling**: Standardized error handling across SQL plugins
- **Configuration Validation**: Shared configuration validation logic

### Health Monitoring
- **Connection Health**: Monitor database connection status
- **Query Performance**: Track query execution times
- **Error Rates**: Monitor database error frequencies
- **Resource Usage**: Track connection pool usage

### Metrics Integration
- **Prometheus Metrics**: Built-in Prometheus metrics collection
- **Custom Metrics**: Support for custom metric definitions
- **Performance Tracking**: Query duration and throughput metrics
- **Error Tracking**: Database error categorization and counting

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    SQL Base Plugin                         │
├─────────────────────────────────────────────────────────────┤
│  Health Checker  │  Metrics Collector  │  Base Plugin     │
├─────────────────────────────────────────────────────────────┤
│  Connection Mgmt │  Error Handling     │  Config Validation│
├─────────────────────────────────────────────────────────────┤
│              Common SQL Interfaces                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Specific SQL Plugins                           │
│  MySQL Plugin  │  PostgreSQL Plugin  │  MSSQL Plugin      │
└─────────────────────────────────────────────────────────────┘
```

## Installation

The SQL Base plugin is automatically included when you import any SQL plugin:

```go
import _ "github.com/go-lynx/lynx/plugins/sql/mysql"     // Includes base
import _ "github.com/go-lynx/lynx/plugins/sql/pgsql"     // Includes base
import _ "github.com/go-lynx/lynx/plugins/sql/mssql"     // Includes base
```

## Configuration

### Basic Configuration

```yaml
lynx:
  sql:
    # Common configuration for all SQL plugins
    health_check:
      enabled: true
      interval: "30s"
      timeout: "5s"
    
    metrics:
      enabled: true
      namespace: "lynx_sql"
      
    connection_pool:
      max_open_conns: 100
      max_idle_conns: 10
      conn_max_lifetime: "1h"
      conn_max_idle_time: "30m"
```

### Advanced Configuration

```yaml
lynx:
  sql:
    health_check:
      enabled: true
      interval: "30s"
      timeout: "5s"
      retry_count: 3
      retry_interval: "1s"
      
    metrics:
      enabled: true
      namespace: "lynx_sql"
      subsystem: "database"
      labels:
        environment: "production"
        service: "user-service"
        
    connection_pool:
      max_open_conns: 100
      max_idle_conns: 10
      conn_max_lifetime: "1h"
      conn_max_idle_time: "30m"
      conn_max_idle_count: 5
      
    logging:
      enabled: true
      level: "info"
      slow_query_threshold: "1s"
```

## Usage

### Health Checking

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/base"
    "github.com/go-lynx/lynx/app"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // Get health checker
    healthChecker := base.GetHealthChecker()
    
    // Check database health
    health, err := healthChecker.CheckHealth("mysql")
    if err != nil {
        log.Errorf("Health check failed: %v", err)
        return
    }
    
    if health.IsHealthy {
        log.Infof("Database is healthy: %s", health.Message)
    } else {
        log.Errorf("Database is unhealthy: %s", health.Message)
    }
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

### Metrics Collection

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/base"
    "github.com/prometheus/client_golang/prometheus"
)

func main() {
    // Get metrics collector
    metrics := base.GetMetricsCollector()
    
    // Register custom metrics
    customCounter := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "custom_sql_operations_total",
            Help: "Total number of custom SQL operations",
        },
        []string{"operation", "status"},
    )
    
    metrics.RegisterCustomMetric("custom_operations", customCounter)
    
    // Increment custom metric
    customCounter.WithLabelValues("select", "success").Inc()
}
```

### Error Handling

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/base"
    "github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
    // Get error handler
    errorHandler := base.GetErrorHandler()
    
    // Handle database errors
    err := someDatabaseOperation()
    if err != nil {
        // Categorize error
        errorType := errorHandler.CategorizeError(err)
        
        switch errorType {
        case interfaces.ErrConnectionFailed:
            log.Errorf("Connection failed: %v", err)
        case interfaces.ErrQueryTimeout:
            log.Errorf("Query timeout: %v", err)
        case interfaces.ErrConstraintViolation:
            log.Errorf("Constraint violation: %v", err)
        default:
            log.Errorf("Unknown error: %v", err)
        }
        
        // Record error metrics
        errorHandler.RecordError(errorType)
    }
}
```

## API Reference

### Health Checker

```go
type HealthChecker interface {
    // CheckHealth checks the health of a specific database
    CheckHealth(database string) (*HealthStatus, error)
    
    // CheckAllHealth checks the health of all databases
    CheckAllHealth() (map[string]*HealthStatus, error)
    
    // RegisterHealthCheck registers a custom health check
    RegisterHealthCheck(database string, check func() error)
}

type HealthStatus struct {
    IsHealthy bool
    Message   string
    Timestamp time.Time
    Duration  time.Duration
}
```

### Metrics Collector

```go
type MetricsCollector interface {
    // RegisterCustomMetric registers a custom Prometheus metric
    RegisterCustomMetric(name string, metric prometheus.Collector)
    
    // GetMetric returns a registered metric
    GetMetric(name string) prometheus.Collector
    
    // RecordQueryDuration records query execution duration
    RecordQueryDuration(database, query string, duration time.Duration)
    
    // RecordQueryError records query errors
    RecordQueryError(database, query, errorType string)
}
```

### Error Handler

```go
type ErrorHandler interface {
    // CategorizeError categorizes database errors
    CategorizeError(err error) interfaces.ErrorType
    
    // RecordError records error metrics
    RecordError(errorType interfaces.ErrorType)
    
    // GetErrorStats returns error statistics
    GetErrorStats() map[interfaces.ErrorType]int64
}
```

## Monitoring and Metrics

### Health Check Metrics

- `sql_health_check_total`: Total number of health checks performed
- `sql_health_check_duration_seconds`: Duration of health checks
- `sql_health_check_failures_total`: Total number of health check failures

### Connection Metrics

- `sql_connections_total`: Total number of database connections
- `sql_connections_active`: Number of active connections
- `sql_connections_idle`: Number of idle connections
- `sql_connection_errors_total`: Total number of connection errors

### Query Metrics

- `sql_queries_total`: Total number of SQL queries executed
- `sql_query_duration_seconds`: Duration of SQL queries
- `sql_query_errors_total`: Total number of query errors

### Error Metrics

- `sql_errors_total`: Total number of database errors by type
- `sql_connection_errors_total`: Total number of connection errors
- `sql_query_errors_total`: Total number of query errors

## Health Checks

### Built-in Health Checks

1. **Connection Health**: Verifies database connectivity
2. **Query Health**: Executes a simple query to verify database functionality
3. **Pool Health**: Checks connection pool status
4. **Configuration Health**: Validates database configuration

### Custom Health Checks

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/base"
)

func main() {
    healthChecker := base.GetHealthChecker()
    
    // Register custom health check
    healthChecker.RegisterHealthCheck("mysql", func() error {
        // Your custom health check logic
        return performCustomHealthCheck()
    })
}
```

## Error Handling

### Error Types

```go
const (
    ErrConnectionFailed     ErrorType = "connection_failed"
    ErrQueryTimeout        ErrorType = "query_timeout"
    ErrConstraintViolation ErrorType = "constraint_violation"
    ErrTransactionFailed   ErrorType = "transaction_failed"
    ErrConfigurationError  ErrorType = "configuration_error"
    ErrUnknown             ErrorType = "unknown"
)
```

### Error Recovery

```go
package main

import (
    "github.com/go-lynx/lynx/plugins/sql/base"
)

func main() {
    errorHandler := base.GetErrorHandler()
    
    // Implement retry logic based on error type
    err := performDatabaseOperation()
    if err != nil {
        errorType := errorHandler.CategorizeError(err)
        
        if errorType == interfaces.ErrConnectionFailed {
            // Retry connection
            time.Sleep(time.Second)
            err = performDatabaseOperation()
        }
    }
}
```

## Best Practices

### 1. Health Monitoring
- Enable health checks in production
- Set appropriate health check intervals
- Monitor health check metrics

### 2. Metrics Collection
- Use meaningful metric names
- Include relevant labels
- Monitor query performance

### 3. Error Handling
- Categorize errors appropriately
- Implement retry logic for transient errors
- Log errors with context

### 4. Configuration
- Use connection pooling
- Set appropriate timeouts
- Monitor connection usage

## Troubleshooting

### Common Issues

1. **Health Check Failures**
   - Check database connectivity
   - Verify configuration
   - Check network connectivity

2. **Metrics Not Appearing**
   - Verify metrics are enabled
   - Check Prometheus configuration
   - Ensure metrics are registered

3. **Connection Pool Issues**
   - Monitor connection usage
   - Adjust pool settings
   - Check for connection leaks

### Debug Logging

Enable debug logging to troubleshoot issues:

```yaml
lynx:
  log:
    level: debug
```

## Dependencies

- `github.com/go-lynx/lynx`: Lynx framework core
- `github.com/prometheus/client_golang`: Prometheus metrics
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
