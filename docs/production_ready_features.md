# Production-Ready Features Guide

This document provides detailed information about the production-ready features newly added to the Lynx framework, including signal handling, error recovery, and monitoring mechanisms.

## 🚀 High Priority Improvements

### 1. Signal Handling and Graceful Shutdown

#### Feature Overview
- Complete signal handling mechanism (SIGTERM, SIGINT, SIGQUIT)
- Graceful shutdown timeout configuration
- Resource cleanup guarantees

#### Usage

```go
package main

import (
    "github.com/go-lynx/lynx/boot"
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/log"
)

func main() {
    // Create application instance
    application := boot.NewApplication(wireApp)
    
    // Run application (automatically handles signals)
    if err := application.Run(); err != nil {
        log.Fatal(err)
    }
}

func wireApp(logger log.Logger) (*kratos.App, error) {
    app := kratos.New(
        kratos.ID("my-service"),
        kratos.Name("My Service"),
        kratos.Version("1.0.0"),
        kratos.Logger(logger),
    )
    return app, nil
}
```

#### Configuration Options

```yaml
# config.yml
lynx:
  application:
    name: "my-service"
    version: "1.0.0"
  
  # Graceful shutdown configuration
  shutdown:
    timeout: 30s  # Shutdown timeout
    graceful: true  # Enable graceful shutdown
```

### 2. Error Handling and Recovery Mechanisms

#### Feature Overview
- Global circuit breaker pattern
- Plugin-level health checks
- Automatic error recovery strategies
- Error classification and handling

#### Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/app/observability/metrics"
)

func main() {
    // Create production-level monitoring metrics
    productionMetrics := metrics.NewProductionMetrics()
    productionMetrics.Start()
    defer productionMetrics.Stop()

    // Create error recovery manager
    errorRecoveryManager := app.NewErrorRecoveryManager(productionMetrics)
    defer errorRecoveryManager.Stop()

    // Record error (automatically triggers recovery)
    errorRecoveryManager.RecordError(
        "database",           // Error type
        "connection timeout", // Error message
        "mysql-plugin",       // Component name
        app.ErrorSeverityMedium, // Error severity
        map[string]interface{}{   // Context information
            "timeout": "5s",
            "retries": 3,
        },
    )

    // Get error statistics
    stats := errorRecoveryManager.GetErrorStats()
    fmt.Printf("Error Statistics: %+v\n", stats)

    // Check health status
    if !errorRecoveryManager.IsHealthy() {
        fmt.Println("System is unhealthy")
    }
}
```

#### Custom Recovery Strategies

```go
// Custom database recovery strategy
type DatabaseRecoveryStrategy struct {
    name    string
    timeout time.Duration
}

func (s *DatabaseRecoveryStrategy) Name() string {
    return s.name
}

func (s *DatabaseRecoveryStrategy) CanRecover(errorType string, severity app.ErrorSeverity) bool {
    return errorType == "database" && severity <= app.ErrorSeverityMedium
}

func (s *DatabaseRecoveryStrategy) Recover(ctx context.Context, record app.ErrorRecord) (bool, error) {
    // Implement database reconnection logic
    select {
    case <-ctx.Done():
        return false, ctx.Err()
    case <-time.After(2 * time.Second):
        // Simulate successful reconnection
        return true, nil
    }
}

func (s *DatabaseRecoveryStrategy) GetTimeout() time.Duration {
    return s.timeout
}

// Register custom strategy
customStrategy := &DatabaseRecoveryStrategy{
    name:    "database-recovery",
    timeout: 10 * time.Second,
}
errorRecoveryManager.RegisterRecoveryStrategy("database", customStrategy)
```

### 3. Monitoring and Observability

#### Feature Overview
- Comprehensive production-level metrics collection
- Prometheus metrics export
- System resource monitoring
- Plugin health status monitoring

#### Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app/observability/metrics"
    "time"
)

func main() {
    // Create production-level monitoring metrics
    productionMetrics := metrics.NewProductionMetrics()
    productionMetrics.Start()
    defer productionMetrics.Stop()

    // Record plugin health status
    productionMetrics.RecordPluginHealth("my-plugin", "plugin-1", true)

    // Record plugin operation latency
    productionMetrics.RecordPluginLatency("my-plugin", "plugin-1", "operation", 100*time.Millisecond)

    // Record event publishing
    productionMetrics.RecordEventPublished("user-created", "business-bus")

    // Record HTTP requests
    productionMetrics.RecordRequest("POST", "/api/users", "200", "http", 50*time.Millisecond, 1024, 2048)

    // Record cache hits
    productionMetrics.RecordCacheHit("user-cache")

    // Record database queries
    productionMetrics.RecordDBQuery("mysql", "read", "get_user", 10*time.Millisecond)

    // Get metrics snapshot
    snapshot := productionMetrics.GetMetrics()
    fmt.Printf("Metrics Snapshot: %+v\n", snapshot)
}
```

#### Prometheus Metrics

The framework automatically exports the following Prometheus metrics:

**Application Metrics**
- `lynx_app_start_time_seconds` - Application start time
- `lynx_app_uptime_seconds` - Application uptime
- `lynx_app_version_info` - Application version information

**System Metrics**
- `lynx_system_memory_bytes` - System memory usage
- `lynx_system_goroutines` - Number of goroutines
- `lynx_system_threads` - Number of threads
- `lynx_system_cpus` - Number of CPUs

**Plugin Metrics**
- `lynx_plugin_count` - Total plugin count
- `lynx_plugin_health_status` - Plugin health status
- `lynx_plugin_errors_total` - Total plugin errors
- `lynx_plugin_latency_seconds` - Plugin operation latency

**Circuit Breaker Metrics**
- `lynx_circuit_breaker_state` - Circuit breaker state
- `lynx_circuit_breaker_failures_total` - Total circuit breaker failures
- `lynx_circuit_breaker_successes_total` - Total circuit breaker successes

**Health Check Metrics**
- `lynx_health_check_status` - Health check status
- `lynx_health_check_latency_seconds` - Health check latency
- `lynx_health_check_errors_total` - Total health check errors

**Event System Metrics**
- `lynx_events_published_total` - Total events published
- `lynx_events_processed_total` - Total events processed
- `lynx_events_dropped_total` - Total events dropped
- `lynx_event_latency_seconds` - Event processing latency

**HTTP/GRPC Metrics**
- `lynx_requests_total` - Total requests
- `lynx_request_duration_seconds` - Request duration
- `lynx_request_size_bytes` - Request size
- `lynx_response_size_bytes` - Response size
- `lynx_error_rate` - Error rate

## 🔧 Configuration Examples

### Complete Configuration Example

```yaml
# config.yml
lynx:
  application:
    name: "production-service"
    version: "1.0.0"
  
  # Graceful shutdown configuration
  shutdown:
    timeout: 30s
    graceful: true
  
  # Error recovery configuration
  error_recovery:
    max_error_history: 1000
    max_recovery_history: 500
    error_threshold: 10
    recovery_timeout: 30s
  
  # Monitoring configuration
  metrics:
    enabled: true
    update_interval: 30s
    prometheus:
      enabled: true
      port: 9090
  
  # Health check configuration
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
  
  # Circuit breaker configuration
  circuit_breaker:
    default_threshold: 5
    default_timeout: 60s
    half_open_limit: 3
```

### Environment Variable Configuration

```bash
# Graceful shutdown timeout
export LYNX_SHUTDOWN_TIMEOUT=30s

# Error threshold
export LYNX_ERROR_THRESHOLD=10

# Monitoring port
export LYNX_METRICS_PORT=9090

# Health check interval
export LYNX_HEALTH_CHECK_INTERVAL=30s
```

## 📊 Monitoring Dashboards

### Grafana Dashboard Configuration

```json
{
  "dashboard": {
    "title": "Lynx Framework Dashboard",
    "panels": [
      {
        "title": "Application Health",
        "type": "stat",
        "targets": [
          {
            "expr": "lynx_app_uptime_seconds",
            "legendFormat": "Uptime"
          }
        ]
      },
      {
        "title": "System Resources",
        "type": "graph",
        "targets": [
          {
            "expr": "lynx_system_memory_bytes",
            "legendFormat": "Memory Usage"
          },
          {
            "expr": "lynx_system_goroutines",
            "legendFormat": "Goroutines"
          }
        ]
      },
      {
        "title": "Plugin Health",
        "type": "table",
        "targets": [
          {
            "expr": "lynx_plugin_health_status",
            "legendFormat": "{{plugin_name}}"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(lynx_plugin_errors_total[5m])",
            "legendFormat": "{{plugin_name}}"
          }
        ]
      }
    ]
  }
}
```

## 🚨 Alert Configuration

### Prometheus Alert Rules

```yaml
groups:
  - name: lynx-alerts
    rules:
      # Application health alerts
      - alert: LynxAppUnhealthy
        expr: lynx_health_check_status == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Lynx application is unhealthy"
          description: "Application {{ $labels.app }} is unhealthy for more than 1 minute"
      
      # Error rate alerts
      - alert: HighErrorRate
        expr: rate(lynx_plugin_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"
      
      # Circuit breaker alerts
      - alert: CircuitBreakerOpen
        expr: lynx_circuit_breaker_state == 1
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker is open"
          description: "Circuit breaker {{ $labels.breaker_name }} is open"
      
      # Memory usage alerts
      - alert: HighMemoryUsage
        expr: lynx_system_memory_bytes > 1e9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }} bytes"
```

## 🔍 Troubleshooting

### Common Issues

1. **Graceful Shutdown Timeout**
   - Check shutdown timeout configuration
   - Ensure plugins correctly implement CleanupTasks method
   - Review logs for shutdown process

2. **Error Recovery Failure**
   - Check error type and severity
   - Verify recovery strategy configuration
   - Review error statistics and recovery history

3. **Missing Monitoring Metrics**
   - Confirm monitoring is enabled
   - Check Prometheus configuration
   - Verify metrics collection interval

### Debug Commands

```bash
# Check application health status
curl http://localhost:8080/health

# View monitoring metrics
curl http://localhost:9090/metrics

# View error statistics
curl http://localhost:8080/debug/error-stats

# View health report
curl http://localhost:8080/debug/health-report
```

## 📈 Performance Optimization

### Best Practices

1. **Reasonable Timeout Configuration**
   - Adjust shutdown timeout based on business requirements
   - Set appropriate recovery timeout
   - Configure health check intervals

2. **Monitoring Metrics Optimization**
   - Avoid excessive metrics collection
   - Use appropriate metric bucket configurations
   - Regularly clean historical data

3. **Error Handling Strategies**
   - Configure different recovery strategies based on error types
   - Set reasonable error thresholds
   - Implement custom recovery logic

4. **Resource Management**
   - Monitor memory and CPU usage
   - Set resource limits
   - Implement resource cleanup

## 🔄 Upgrade Guide

### Upgrading from Older Versions

1. **Update Dependencies**
   ```bash
   go get -u github.com/go-lynx/lynx
   ```

2. **Update Configuration**
   - Add new configuration items
   - Update existing configuration format

3. **Code Updates**
   - Use new Application structure
   - Integrate monitoring and error recovery features

4. **Test Verification**
   - Verify signal handling
   - Test error recovery
   - Check monitoring metrics

## 📚 Reference Documentation

- [Lynx Framework Documentation](https://go-lynx.cn/docs)
- [Prometheus Monitoring Guide](https://prometheus.io/docs/)
- [Grafana Dashboard Configuration](https://grafana.com/docs/)
- [Go Signal Handling](https://golang.org/pkg/os/signal/)

## 🤝 Contributing

Welcome to submit Issues and Pull Requests to improve these features.

- [GitHub Issues](https://github.com/go-lynx/lynx/issues)
- [GitHub Discussions](https://github.com/go-lynx/lynx/discussions)
