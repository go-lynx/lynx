# Lynx Tracer Plugin

Lynx distributed tracing plugin, implementing distributed tracing functionality based on OpenTelemetry.

## Features

- ✅ OpenTelemetry standard compliant
- ✅ Export protocols: OTLP gRPC, OTLP HTTP
- ✅ Transport capabilities: TLS (including mutual), timeout, retry, compression (gzip), custom headers
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
```


### HTTP Export Example (OTLP/HTTP)

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
[INFO] Initializing link monitoring component
[INFO] Tracing component successfully initialized
[INFO] Tracer provider shutdown successfully
```


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

### v1.0.0

- ✅ Basic tracing functionality
- ✅ OTLP gRPC exporter
- ✅ Configurable sampling rate