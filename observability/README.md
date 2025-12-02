# Observability Package

The `observability` package provides comprehensive monitoring, metrics, and observability utilities for the Lynx framework. It enables production-ready application monitoring with Prometheus metrics integration.

## Overview

This package provides:

- **Production Metrics** - Comprehensive Prometheus metrics for all aspects of the application
- **System Monitoring** - Memory, goroutines, CPU, and thread tracking
- **Plugin Monitoring** - Plugin health, errors, and latency metrics
- **Request Metrics** - HTTP/gRPC request tracking
- **Event System Metrics** - Event publishing and processing statistics

## Package Structure

```
observability/
└── metrics/
    ├── handler.go            # HTTP handler for metrics endpoint
    ├── production_metrics.go # Comprehensive production metrics
    └── registry.go           # Prometheus registry management
```

## Metrics Categories

### Application Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_app_start_time_seconds` | Gauge | Application start time (Unix timestamp) |
| `lynx_app_uptime_seconds` | Gauge | Application uptime in seconds |
| `lynx_app_version_info` | GaugeVec | Application version information |

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_system_memory_bytes` | Gauge | Current memory usage |
| `lynx_system_goroutines` | Gauge | Number of goroutines |
| `lynx_system_threads` | Gauge | Number of OS threads |
| `lynx_system_cpus` | Gauge | Number of CPUs |

### Plugin Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_plugin_count` | Gauge | Total number of plugins |
| `lynx_plugin_health_status` | GaugeVec | Plugin health status (0/1) |
| `lynx_plugin_errors_total` | CounterVec | Total plugin errors |
| `lynx_plugin_latency_seconds` | HistogramVec | Plugin operation latency |

### Circuit Breaker Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_circuit_breaker_state` | GaugeVec | Circuit breaker state (0=closed, 1=open) |
| `lynx_circuit_breaker_failures_total` | CounterVec | Total failures |
| `lynx_circuit_breaker_successes_total` | CounterVec | Total successes |

### Health Check Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_health_check_status` | GaugeVec | Health check status |
| `lynx_health_check_latency_seconds` | HistogramVec | Health check latency |
| `lynx_health_check_errors_total` | CounterVec | Health check errors |

### Event System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_events_published_total` | CounterVec | Total events published |
| `lynx_events_processed_total` | CounterVec | Total events processed |
| `lynx_events_dropped_total` | CounterVec | Total events dropped |
| `lynx_event_latency_seconds` | HistogramVec | Event processing latency |

### HTTP/gRPC Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_requests_total` | CounterVec | Total requests |
| `lynx_request_duration_seconds` | HistogramVec | Request duration |
| `lynx_request_size_bytes` | HistogramVec | Request size |
| `lynx_response_size_bytes` | HistogramVec | Response size |
| `lynx_error_rate` | GaugeVec | Error rate |

### Cache Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_cache_hits_total` | CounterVec | Cache hits |
| `lynx_cache_misses_total` | CounterVec | Cache misses |
| `lynx_cache_size` | GaugeVec | Current cache size |
| `lynx_cache_evictions_total` | CounterVec | Cache evictions |

### Database Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `lynx_db_connections` | GaugeVec | Active database connections |
| `lynx_db_queries_total` | CounterVec | Total database queries |
| `lynx_db_query_duration_seconds` | HistogramVec | Query duration |
| `lynx_db_errors_total` | CounterVec | Database errors |

## Usage

### Basic Setup

```go
import (
    "github.com/go-lynx/lynx/observability/metrics"
)

func main() {
    // Create production metrics
    pm := metrics.NewProductionMetrics()
    
    // Start background metric collection
    pm.Start()
    defer pm.Stop()
    
    // Your application code...
}
```

### Recording Metrics

```go
// Record plugin health
pm.RecordPluginHealth("my-plugin", "plugin-1", true)

// Record plugin latency
pm.RecordPluginLatency("my-plugin", "plugin-1", "operation", 100*time.Millisecond)

// Record event
pm.RecordEventPublished("user-created", "main-bus")

// Record HTTP request
pm.RecordRequest("POST", "/api/users", "200", "http", 50*time.Millisecond, 1024, 2048)

// Record cache hit
pm.RecordCacheHit("user-cache")

// Record database query
pm.RecordDBQuery("mysql", "read", "get_user", 10*time.Millisecond)
```

### Getting Metrics Snapshot

```go
snapshot := pm.GetMetrics()
fmt.Printf("Uptime: %v\n", snapshot["uptime"])
fmt.Printf("Goroutines: %v\n", snapshot["goroutines"])
```

### Exposing Metrics Endpoint

```go
import (
    "net/http"
    "github.com/go-lynx/lynx/observability/metrics"
)

func main() {
    // Register metrics handler
    http.Handle("/metrics", metrics.Handler())
    http.ListenAndServe(":9090", nil)
}
```

## Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'lynx-app'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Grafana Dashboard

Example Grafana panel queries:

```promql
# Application uptime
lynx_app_uptime_seconds

# Request rate
rate(lynx_requests_total[5m])

# Error rate
rate(lynx_plugin_errors_total[5m])

# P95 latency
histogram_quantile(0.95, rate(lynx_request_duration_seconds_bucket[5m]))

# Memory usage
lynx_system_memory_bytes
```

## Alert Rules

Example Prometheus alert rules:

```yaml
groups:
  - name: lynx-alerts
    rules:
      - alert: HighErrorRate
        expr: rate(lynx_plugin_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          
      - alert: HighMemoryUsage
        expr: lynx_system_memory_bytes > 1e9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage (>1GB)"
```

## Best Practices

1. **Start metrics collection early** - Initialize before loading plugins
2. **Use appropriate labels** - Don't create high-cardinality labels
3. **Set up alerting** - Monitor critical metrics
4. **Use dashboards** - Visualize metrics in Grafana
5. **Clean up on shutdown** - Call `Stop()` to clean up resources

## License

Apache License 2.0

