# HTTP Plugin for Lynx Framework

This plugin provides comprehensive HTTP server functionality for the Lynx framework, built on top of the Kratos framework for high performance and reliability.

## Features

- **HTTP/HTTPS Server Support**: Full HTTP/1.1 and HTTPS support with TLS configuration
- **Middleware Integration**: Built-in tracing, logging, rate limiting, validation, and recovery
- **TLS Support**: Complete TLS/SSL configuration with client certificate authentication
- **Custom Response Encoding**: Flexible response encoding and error handling
- **Health Checking**: Built-in health check endpoints with detailed status information
- **Event Emission**: Integration with Lynx event system for monitoring and observability
- **Performance Optimization**: Connection pooling, timeout management, and buffer optimization
- **Security Features**: Rate limiting, request size limits, and security headers
- **Monitoring**: Comprehensive Prometheus metrics and observability

## Installation

```bash
go get github.com/go-lynx/lynx/plugins/service/http
```

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/plugins/service/http"
    "github.com/go-lynx/lynx/plugins/service/http/conf"
    "google.golang.org/protobuf/types/known/durationpb"
)

func main() {
    // Create HTTP plugin with configuration
    httpPlugin := http.NewServiceHttp()
    
    // Configure the plugin
    config := &conf.Http{
        Network: "tcp",
        Addr:    ":8080",
        Timeout: &durationpb.Duration{Seconds: 30},
        Monitoring: &conf.MonitoringConfig{
            EnableMetrics: true,
            MetricsPath:   "/metrics",
            HealthPath:    "/health",
        },
        Middleware: &conf.MiddlewareConfig{
            EnableTracing:    true,
            EnableLogging:    true,
            EnableMetrics:    true,
            EnableRecovery:   true,
            EnableValidation: true,
            EnableRateLimit:  true,
        },
    }
    
    // Apply configuration
    err := httpPlugin.Configure(config)
    if err != nil {
        panic(err)
    }
    
    // Register with Lynx application
    app.Lynx().GetPluginManager().RegisterPlugin(httpPlugin)
}
```

### Configuration

The plugin can be configured through YAML configuration files or programmatically:

```yaml
lynx:
  http:
    network: tcp
    addr: :8080
    timeout: 30s
    tls_enable: false
    tls_auth_type: 0
    
    monitoring:
      enable_metrics: true
      metrics_path: /metrics
      health_path: /health
      enable_request_logging: true
      enable_error_logging: true
      enable_route_metrics: true
      enable_connection_metrics: true
      enable_queue_metrics: true
    
    security:
      max_request_size: 10485760  # 10MB
      cors:
        enabled: true
        allowed_origins: ["*"]
        allowed_methods: ["GET", "POST", "PUT", "DELETE"]
        allowed_headers: ["*"]
      rate_limit:
        enabled: true
        rate_per_second: 100
        burst_limit: 200
      security_headers:
        enabled: true
        content_security_policy: "default-src 'self'"
        x_frame_options: "DENY"
        x_content_type_options: "nosniff"
        x_xss_protection: "1; mode=block"
    
    performance:
      max_connections: 1000
      max_concurrent_requests: 500
      read_buffer_size: 4096
      write_buffer_size: 4096
      read_timeout: 30s
      write_timeout: 30s
      idle_timeout: 60s
      read_header_timeout: 20s
      connection_pool:
        max_idle_conns: 100
        max_idle_conns_per_host: 10
        max_conns_per_host: 100
        keep_alive_duration: 30s
    
    middleware:
      enable_tracing: true
      enable_logging: true
      enable_recovery: true
      enable_validation: true
      enable_rate_limit: true
      enable_metrics: true
    
    graceful_shutdown:
      shutdown_timeout: 30s
      wait_for_ongoing_requests: true
      max_wait_time: 60s
```

### TLS Configuration

For HTTPS support, configure TLS settings:

```yaml
lynx:
  http:
    tls_enable: true
    tls_auth_type: 2  # 0: No client auth, 1: Request client cert, 2: Require client cert
```

Ensure your Lynx application has a certificate provider configured:

```go
// Configure certificate provider in your Lynx application
app.Lynx().SetCertificateProvider(certProvider)
```

### Middleware Configuration

The HTTP plugin includes several built-in middlewares that can be enabled/disabled:

```go
middlewareConfig := &conf.MiddlewareConfig{
    EnableTracing:    true,  // Distributed tracing
    EnableLogging:    true,  // Request/response logging
    EnableMetrics:    true,  // Prometheus metrics
    EnableRecovery:   true,  // Panic recovery
    EnableValidation: true,  // Request validation
    EnableRateLimit:  true,  // Rate limiting
}
```

### Custom Handlers

Add custom HTTP handlers to your server:

```go
// Get the HTTP server instance
server := httpPlugin.GetServer()

// Add custom routes
server.HandleFunc("GET", "/api/users", func(w http.ResponseWriter, r *http.Request) {
    // Your handler logic
})

// Add middleware to specific routes
server.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Custom middleware logic
        next.ServeHTTP(w, r)
    })
})
```

## Monitoring and Observability

### Health Check Endpoint

The plugin automatically provides a health check endpoint at `/health`:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "uptime": "1h2m3s",
  "version": "v2.0.0"
}
```

### Metrics Endpoint

Prometheus metrics are available at `/metrics`:

```bash
curl http://localhost:8080/metrics
```

Available metrics:
- `lynx_http_requests_total`: Total HTTP requests
- `lynx_http_request_duration_seconds`: Request duration histogram
- `lynx_http_response_size_bytes`: Response size histogram
- `lynx_http_request_size_bytes`: Request size histogram
- `lynx_http_errors_total`: Error count by type
- `lynx_http_active_connections`: Active connections gauge
- `lynx_http_connection_pool_usage`: Connection pool usage
- `lynx_http_request_queue_length`: Request queue length

### Logging

The plugin integrates with Lynx's logging system:

```go
import "github.com/go-lynx/lynx/app/log"

// Logs are automatically generated for:
// - Request/response details
// - Errors and panics
// - Configuration changes
// - Server lifecycle events
```

## Performance Tuning

### Connection Pooling

Configure connection pooling for optimal performance:

```yaml
performance:
  connection_pool:
    max_idle_conns: 100
    max_idle_conns_per_host: 10
    max_conns_per_host: 100
    keep_alive_duration: 30s
```

### Buffer Sizes

Optimize buffer sizes for your workload:

```yaml
performance:
  read_buffer_size: 4096   # 4KB read buffer
  write_buffer_size: 4096  # 4KB write buffer
```

### Timeouts

Configure appropriate timeouts:

```yaml
performance:
  read_timeout: 30s        # Time to read entire request
  write_timeout: 30s       # Time to write response
  idle_timeout: 60s        # Time to keep idle connections
  read_header_timeout: 20s # Time to read request headers
```

## Security Best Practices

### Rate Limiting

Enable rate limiting to prevent abuse:

```yaml
security:
  rate_limit:
    enabled: true
    rate_per_second: 100   # Requests per second
    burst_limit: 200       # Burst allowance
```

### Request Size Limits

Set appropriate request size limits:

```yaml
security:
  max_request_size: 10485760  # 10MB limit
```

### Security Headers

Enable security headers:

```yaml
security:
  security_headers:
    enabled: true
    content_security_policy: "default-src 'self'"
    x_frame_options: "DENY"
    x_content_type_options: "nosniff"
    x_xss_protection: "1; mode=block"
```

### CORS Configuration

Configure CORS for web applications:

```yaml
security:
  cors:
    enabled: true
    allowed_origins: ["https://yourdomain.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Content-Type", "Authorization"]
    allow_credentials: true
    max_age: 86400
```

## Error Handling

The plugin provides comprehensive error handling:

```go
// Custom error encoder
func customErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
    // Your custom error handling logic
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(map[string]string{
        "error": err.Error(),
        "code":  "INTERNAL_ERROR",
    })
}

// Configure custom error handling
config := &conf.Http{
    // ... other config
}
```

## Graceful Shutdown

The plugin supports graceful shutdown:

```yaml
graceful_shutdown:
  shutdown_timeout: 30s
  wait_for_ongoing_requests: true
  max_wait_time: 60s
```

## Testing

### Unit Tests

```bash
go test ./plugins/service/http -v
```

### Integration Tests

```bash
go test ./plugins/service/http -v -tags=integration
```

### Stress Tests

```bash
go test ./plugins/service/http -v -run=TestHTTPPluginStress
```

### Benchmarks

```bash
go test ./plugins/service/http -bench=. -benchmem
```

## Troubleshooting

### Common Issues

1. **TLS Configuration Errors**
   - Ensure certificate provider is configured
   - Verify certificate and private key are valid
   - Check TLS authentication type settings

2. **Port Already in Use**
   - Check if another service is using the port
   - Use `:0` for dynamic port allocation
   - Verify firewall settings

3. **High Memory Usage**
   - Adjust connection pool settings
   - Reduce buffer sizes
   - Monitor request size limits

4. **Performance Issues**
   - Enable metrics to identify bottlenecks
   - Adjust timeout settings
   - Optimize middleware configuration

### Debug Mode

Enable debug logging:

```go
import "github.com/go-lynx/lynx/app/log"

log.SetLevel(log.DebugLevel)
```

### Health Check Failures

Check the health endpoint for detailed status:

```bash
curl -v http://localhost:8080/health
```

## API Reference

### Configuration Types

- `conf.Http`: Main HTTP server configuration
- `conf.MonitoringConfig`: Monitoring and metrics configuration
- `conf.SecurityConfig`: Security settings
- `conf.PerformanceConfig`: Performance tuning options
- `conf.MiddlewareConfig`: Middleware configuration
- `conf.GracefulShutdownConfig`: Shutdown behavior

### Plugin Methods

- `NewServiceHttp()`: Create new HTTP plugin instance
- `Configure(config)`: Apply configuration
- `validateConfig()`: Validate configuration
- `buildMiddlewares()`: Build middleware chain
- `healthCheckHandler()`: Get health check handler

## Dependencies

- github.com/go-kratos/kratos/v2 v2.8.4
- github.com/go-lynx/lynx v1.2.1
- github.com/prometheus/client_golang v1.23.0
- golang.org/x/time v0.12.0

## License

Apache License 2.0

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## Support

For issues and questions:
- GitHub Issues: [Create an issue](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Framework Docs](https://lynx.dev)
- Community: [Discord Server](https://discord.gg/lynx)
