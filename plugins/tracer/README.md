# Lynx Tracer Plugin

Lynx distributed tracing plugin, implementing distributed tracing functionality based on OpenTelemetry.

## Features

- ✅ OpenTelemetry standard compliant
- ✅ Export protocols: OTLP gRPC, OTLP HTTP
- ✅ Transport capabilities: TLS (including mutual), timeout, retry, compression (gzip), custom headers
- ✅ **Connection management**: Connection pooling, load balancing, health checking, automatic reconnection
- ✅ Batch processing: configurable queue, batch size, export timeout, and scheduling delay
- ✅ Propagators: W3C tracecontext, baggage, B3 (single/multi-header), Jaeger
- ✅ Samplers: AlwaysOn/AlwaysOff/TraceIDRatio/ParentBased-TraceIDRatio
- ✅ Resources and limits: service.name/attributes and SpanLimits (attributes/events/links/length)
- ✅ Graceful shutdown and resource cleanup

## Quick Start

### 1. Minimal Configuration (gRPC, recommended)

Add the following configuration to your application configuration file:

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      protocol: PROTOCOL_OTLP_GRPC
      insecure: true
      batch:
        enabled: true
      propagators: [W3C_TRACE_CONTEXT, W3C_BAGGAGE]
```


### 2. Start Application

```bash
go run main.go
```


### 3. View Tracing Data

Access your tracing system (such as Jaeger, Zipkin, etc.) to view tracing data.

## Configuration Guide

### Modular Configuration Options

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      protocol: PROTOCOL_OTLP_GRPC | PROTOCOL_OTLP_HTTP
      http_path: /v1/traces           # Used only for HTTP
      insecure: true                  # Or use TLS
      tls:
        ca_file: /path/ca.pem
        cert_file: /path/client.crt
        key_file: /path/client.key
        insecure_skip_verify: false
      headers:
        Authorization: Bearer ${OTEL_TOKEN}
      timeout: 10s
      retry:
        enabled: true
        initial_interval: 500ms
        max_interval: 5s
      batch:
        enabled: true
        max_queue_size: 2048
        scheduled_delay: 200ms
        export_timeout: 30s
        max_batch_size: 512
      sampler:
        type: SAMPLER_TRACEID_RATIO   # Also supports ALWAYS_ON/ALWAYS_OFF/PARENT_BASED_TRACEID_RATIO
        ratio: 0.1
      propagators: [W3C_TRACE_CONTEXT, W3C_BAGGAGE, B3, B3_MULTI, JAEGER]
      resource:
        service_name: my-service
        attributes:
          env: prod
          team: core
      limits:
        attribute_count_limit: 128
        attribute_value_length_limit: 2048
        event_count_limit: 128
        link_count_limit: 128
      # Connection management (gRPC only)
      connection:
        max_conn_idle_time: 30s
        max_conn_age: 10m
        max_conn_age_grace: 5s
        connect_timeout: 10s
        reconnection_period: 5s
      # Load balancing (gRPC only)
      load_balancing:
        policy: "round_robin"
        health_check: true
```

### Connection Management (gRPC Only)

The Tracer plugin supports advanced connection management for gRPC exporters, including connection pooling, load balancing, and automatic reconnection.

#### Connection Options

- **`max_conn_idle_time`**: Maximum time a connection can be idle before being closed
  - Default: gRPC default (typically 30 minutes)
  - Recommended: 30s - 5m for production environments
  
- **`max_conn_age`**: Maximum age of a connection before it is closed
  - Default: gRPC default (typically unlimited)
  - Recommended: 10m - 1h for production environments
  
- **`max_conn_age_grace`**: Additional grace period for connection closure
  - Default: gRPC default (typically 10s)
  - Recommended: 5s - 30s
  
- **`connect_timeout`**: Time to wait for connection establishment
  - Default: gRPC default (typically 20s)
  - Recommended: 5s - 30s
  
- **`reconnection_period`**: Minimum time between reconnection attempts
  - Default: 5s
  - Recommended: 5s - 30s

#### Load Balancing Options

- **`policy`**: Load balancing policy
  - `"pick_first"`: Use the first available connection (default)
  - `"round_robin"`: Distribute requests across all connections
  - `"least_conn"`: Use the connection with the least active requests
  
- **`health_check`**: Enable health checking for load balancing
  - `true`: Enable health checking (recommended for production)
  - `false`: Disable health checking

#### Implementation Details

The plugin uses gRPC's built-in connection pool management with:
- **Automatic Connection Pooling**: gRPC automatically manages connection pools
- **Connection Reuse**: Connections are reused across multiple requests
- **Load Balancing**: Built-in support for multiple backend instances
- **Health Checking**: Automatic health checking of connections

If connection management is not configured, the plugin uses sensible defaults:
- **Reconnection Period**: 5 seconds
- **Load Balancing**: Round-robin policy
- **Connection Timeout**: gRPC default (20 seconds)
```


### Best Practices

### Production Environments

1. **Set Connection Limits**: Configure `max_conn_age` and `max_conn_idle_time`
2. **Enable Health Checking**: Set `health_check: true`
3. **Use Round-Robin**: Set `policy: "round_robin"` for better load distribution
4. **Configure Timeouts**: Set appropriate `connect_timeout` values

### High Availability

1. **Multiple Endpoints**: Use multiple collector endpoints for redundancy
2. **Connection Pooling**: Let gRPC handle connection pooling automatically
3. **Health Monitoring**: Monitor connection health and reconnection events

### Performance Tuning

1. **Batch Processing**: Enable batch processing to reduce connection overhead
2. **Compression**: Use gzip compression to reduce bandwidth usage
3. **Connection Limits**: Balance between connection reuse and resource usage

## Example Configurations

### Minimal Configuration

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      connection:
        reconnection_period: 10s
      load_balancing:
        policy: "round_robin"
```

### Production Configuration

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      connection:
        max_conn_idle_time: 1m
        max_conn_age: 30m
        max_conn_age_grace: 10s
        connect_timeout: 15s
        reconnection_period: 10s
      load_balancing:
        policy: "round_robin"
        health_check: true
      batch:
        enabled: true
        max_queue_size: 10000
        max_batch_size: 1000
```

### High-Performance Configuration

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      connection:
        max_conn_idle_time: 5m
        max_conn_age: 1h
        max_conn_age_grace: 30s
        connect_timeout: 10s
        reconnection_period: 5s
      load_balancing:
        policy: "round_robin"
        health_check: true
      batch:
        enabled: true
        max_queue_size: 50000
        max_batch_size: 5000
        scheduled_delay: 1s
        export_timeout: 10s
      compression: COMPRESSION_GZIP
```

## HTTP Export Example (OTLP/HTTP)

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4318"
    config:
      protocol: PROTOCOL_OTLP_HTTP
      http_path: /v1/traces
      insecure: true
      compression: COMPRESSION_GZIP
      batch:
        enabled: true
      propagators: [B3, W3C_BAGGAGE]
```


## Environment Configuration

It is recommended to adjust exporter addresses and sampling strategies (config.sampler) based on "modular configuration" in different environments, instead of using the legacy ratio field.

## Usage Examples

### Using in Code

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/app"
    "go.opentelemetry.io/otel"
)

func main() {
    // Start Lynx application
    app := app.New()
    
    // Get tracer
    tracer := otel.Tracer("my-service")
    
    // Create span
    ctx, span := tracer.Start(context.Background(), "my-operation")
    defer span.End()
    
    // Your business logic
    // ...
}
```


### Using in HTTP Service

```go
package main

import (
    "net/http"
    "github.com/go-lynx/lynx/app"
    "go.opentelemetry.io/otel"
)

func main() {
    app := app.New()
    
    http.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        tracer := otel.Tracer("http-server")
        ctx, span := tracer.Start(r.Context(), "handle-request")
        defer span.End()
        
        // Handle request
        w.Write([]byte("Hello, World!"))
    })
    
    http.ListenAndServe(":8080", nil)
}
```


## Supported Exporters

### OTLP gRPC/HTTP Exporters

Support the following features:

- Compression: gzip
- Timeout: configurable timeout
- TLS: one-way or mutual
- Retry: initial/max retry interval
- Batch processing: queue/batch size/export timeout/scheduling delay

### Supported Collectors

- **OpenTelemetry Collector**
- **Jaeger**
- **Zipkin**
- **Prometheus** (via Collector)

## Sampling Strategy

### Sampling Rate Description

| Sampling Rate | Description | Use Case |
|---------------|-------------|----------|
| 0.0 | No sampling | Performance testing |
| 0.1 | 10% sampling | Production environment |
| 0.5 | 50% sampling | Testing environment |
| 1.0 | Full sampling | Development environment |

### Sampling Recommendations

- **Development Environment**: Use 1.0 full sampling for easier debugging
- **Testing Environment**: Use 0.5 sampling to balance performance and observability
- **Production Environment**: Use 0.1-0.3 sampling to avoid performance impact

## Monitoring and Debugging

### Log Output

The plugin outputs detailed log information:

```
[INFO] Initializing tracing component
[INFO] Tracing component successfully initialized
[INFO] Tracer provider shutdown successfully
```

### Connection Management Metrics

Monitor the following connection-related metrics:

- **Connection Establishment**: Time to establish new connections
- **Reconnection Frequency**: How often reconnections occur
- **Connection Pool Size**: Current number of active connections
- **Load Balancing Distribution**: How requests are distributed across connections
- **Export Success/Failure Rates**: Success rate of trace exports
- **Connection Health**: Health status of individual connections

### Recommended Monitoring Tools

- **Prometheus + Grafana**: For metrics collection and visualization
- **Jaeger**: For distributed tracing visualization
- **OpenTelemetry Collector**: For metrics aggregation and export


## Troubleshooting

### Common Issues

#### 1. Connection Failure

**Issue**: Unable to connect to collector
```
failed to create OTLP exporter: context deadline exceeded
```


**Solution**:
- Check if collector address is correct
- Confirm network connection is normal
- Check firewall settings

#### 2. Invalid Sampling Rate

**Issue**: Sampling rate configuration invalid
```
sampling ratio must be between 0 and 1, got 1.5
```


**Solution**:
- Ensure sampling rate is within 0.0-1.0 range
- Check configuration file format

#### 3. Address Configuration Error

**Issue**: Tracing enabled but address not configured
```
tracer address is required when tracing is enabled
```


**Solution**:
- Set correct `addr` configuration item (gRPC: 4317 / HTTP: 4318 + http_path)
- Or disable tracing functionality

#### 4. Connection Management Issues

**Issue**: Connection failures or poor performance
```
failed to create OTLP exporter: connection timeout
```


**Solution**:
- Check `connect_timeout` and network connectivity
- Verify connection pool settings (`max_conn_age`, `max_conn_idle_time`)
- Ensure load balancing policy is appropriate
- Monitor reconnection frequency

#### 5. Load Balancing Problems

**Issue**: Uneven load distribution or connection failures
```
load balancing policy not working as expected
```


**Solution**:
- Verify load balancing policy configuration
- Enable health checking (`health_check: true`)
- Check if multiple endpoints are available
- Monitor connection pool size and distribution

### Debug Mode

Enable detailed log output:

```go
// Set log level
log.SetLevel(log.DebugLevel)
```


## Performance Considerations

### Performance Impact

- **Sampling Rate**: Higher sampling rate leads to greater performance impact
- **Network Latency**: Exporter network latency affects application performance
- **Memory Usage**: Tracing data occupies certain memory

### Optimization Recommendations

1. **Set Sampling Rate Appropriately**: Production environment recommends using 0.1-0.3
2. **Use Local Collector**: Reduce network latency
3. **Configure Timeout**: Avoid long blocking
4. **Monitor Resource Usage**: Regularly check memory and CPU usage

## Version History

### v2.0.0

- ✅ Modular configuration (protocol/TLS/retry/compression/batch processing/propagators/resources/limits)
- ✅ Exporter supports OTLP HTTP
- ✅ Configurable samplers and propagators
- ✅ Graceful shutdown mechanism
- ✅ **Advanced connection management** (connection pooling, load balancing, health checking)
- ✅ **Automatic reconnection** and connection lifecycle management
- ✅ **Production-ready configurations** with best practices

### v1.0.0

- ✅ Basic tracing functionality
- ✅ OTLP gRPC exporter
- ✅ Configurable sampling rate