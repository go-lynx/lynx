# gRPC Client Plugin for Lynx Framework

This plugin provides comprehensive gRPC client functionality for the Lynx framework, including connection management, retry mechanisms, monitoring, and integration with the existing subscription system.

## Features

- **Connection Management**: Automatic connection pooling and lifecycle management
- **Service Discovery**: Integration with Lynx service discovery
- **TLS Support**: Full TLS/SSL support with client authentication
- **Retry Mechanism**: Configurable retry logic with exponential backoff
- **Monitoring**: Comprehensive Prometheus metrics collection
- **Health Checking**: Built-in health check functionality
- **Load Balancing**: Support for various load balancing strategies
- **Middleware Support**: Pluggable middleware for logging, tracing, and metrics
- **Integration**: Seamless integration with `app/subscribe`

## Installation

The gRPC client plugin is automatically registered when imported:

```go
import _ "github.com/go-lynx/lynx/plugins/service/grpc"
```

## Configuration

### Basic Configuration

```yaml
lynx:
  grpc:
    client:
      # Default timeout for gRPC client requests
      default_timeout: "10s"
      
      # Default keep-alive interval for gRPC connections
      default_keep_alive: "30s"
      
      # Maximum number of retries for failed requests
      max_retries: 3
      
      # Backoff duration between retries
      retry_backoff: "1s"
      
      # Maximum number of connections per service (channel pool size)
      # This controls how many connections (channels) are maintained for each service
      # Multiple connections improve performance and fault tolerance
      # Recommended: 3-10 for most services, 10-50 for high-traffic services
      max_connections: 5
      
      # Enable TLS for gRPC client connections
      tls_enable: false
      
      # TLS authentication type (0-4)
      tls_auth_type: 0
      
      # Enable connection pooling (multi-channel pool)
      # When enabled, each service maintains a pool of multiple connections
      # Connections are selected using round-robin, random, or least-used strategies
      connection_pooling: true
      
      # Maximum number of services in the pool
      # This limits the total number of services that can have connection pools
      pool_size: 10
      
      # Connection idle timeout
      idle_timeout: "60s"
      
      # Enable health checking for connections
      health_check_enabled: true
      
      # Health check interval
      health_check_interval: "30s"
      
      # Enable metrics collection
      metrics_enabled: true
      
      # Enable distributed tracing
      tracing_enabled: true
      
      # Enable request logging
      logging_enabled: true
      
      # Maximum message size in bytes (default: 4MB)
      max_message_size: 4194304
      
      # Enable compression
      compression_enabled: false
      
      # Compression type (gzip, deflate, etc.)
      compression_type: "gzip"
```

### Service Discovery Configuration (Recommended)

When using service discovery (like Polaris), configure services using `subscribe_services`:

```yaml
lynx:
  grpc:
    client:
      # Global settings...
      
      # Service subscription configurations (recommended)
      subscribe_services:
        - name: "user-service"
          # No endpoint needed - discovered via service registry
          timeout: "5s"
          required: true
          load_balancer: "round_robin"
          circuit_breaker_enabled: true
          circuit_breaker_threshold: 5
          metadata:
            version: "v1.0"
            
        - name: "order-service"
          timeout: "8s"
          max_retries: 5
          required: false
          load_balancer: "weighted_round_robin"
          
        - name: "payment-service"
          # Fallback endpoint (used only if service discovery fails)
          endpoint: "payment-fallback.internal:9093"
          timeout: "12s"
          required: true
          tls_enable: true
          tls_auth_type: 2
```

### Legacy Static Configuration (Deprecated)

For static endpoints without service discovery:

```yaml
lynx:
  grpc:
    client:
      # Legacy static service configurations (deprecated)
      services:
        - name: "legacy-service"
          endpoint: "legacy.internal:9094"
          timeout: "10s"
          tls_enable: false
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/plugins/service/grpc"
    pb "your/protobuf/package"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // Get gRPC client connection
    conn, err := grpc.GetGrpcClientConnection("user-service")
    if err != nil {
        log.Fatalf("Failed to get gRPC connection: %v", err)
    }
    
    // Create gRPC client
    client := pb.NewUserServiceClient(conn)
    
    // Make a request
    resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{
        UserId: "123",
    })
    if err != nil {
        log.Errorf("Failed to get user: %v", err)
        return
    }
    
    log.Infof("User: %+v", resp.User)
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

### Using with Custom Configuration

```go
package main

import (
    "context"
    "time"
    
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/plugins/service/grpc"
    pb "your/protobuf/package"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // Create custom client configuration
    config := grpc.ClientConfig{
        ServiceName:    "user-service",
        Discovery:      app.Lynx().GetControlPlane().Discovery(),
        TLS:            true,
        TLSAuthType:    4,
        Timeout:        15 * time.Second,
        KeepAlive:      60 * time.Second,
        MaxRetries:     5,
        RetryBackoff:   2 * time.Second,
        MaxConnections: 20,
    }
    
    // Create connection with custom configuration
    conn, err := grpc.CreateGrpcClientConnection(config)
    if err != nil {
        log.Fatalf("Failed to create gRPC connection: %v", err)
    }
    
    // Create gRPC client
    client := pb.NewUserServiceClient(conn)
    
    // Make requests...
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

### Using with app/subscribe Integration

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/app/subscribe"
    pb "your/protobuf/package"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // Initialize gRPC client integration
    err := subscribe.InitializeGrpcClientIntegration()
    if err != nil {
        log.Warnf("gRPC client integration not available: %v", err)
    }
    
    // Get connection using integrated method
    conn, err := subscribe.GetGrpcConnection("user-service")
    if err != nil {
        log.Fatalf("Failed to get gRPC connection: %v", err)
    }
    
    // Create gRPC client
    client := pb.NewUserServiceClient(conn)
    
    // Make requests...
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

## Advanced Features

### Custom Middleware

```go
package main

import (
    "context"
    "github.com/go-kratos/kratos/v2/middleware"
    "github.com/go-lynx/lynx/plugins/service/grpc"
)

func customMiddleware() middleware.Middleware {
    return func(handler middleware.Handler) middleware.Handler {
        return func(ctx context.Context, req interface{}) (interface{}, error) {
            // Your custom logic here
            log.Infof("Custom middleware: processing request")
            
            resp, err := handler(ctx, req)
            
            // Your custom logic here
            log.Infof("Custom middleware: request completed")
            
            return resp, err
        }
    }
}

func main() {
    // Add custom middleware to client configuration
    config := grpc.ClientConfig{
        ServiceName: "user-service",
        Middleware: []middleware.Middleware{
            customMiddleware(),
        },
    }
    
    conn, err := grpc.CreateGrpcClientConnection(config)
    // ...
}
```

### Health Checking

```go
package main

import (
    "github.com/go-lynx/lynx/app/subscribe"
)

func main() {
    // Check health of all gRPC connections
    err := subscribe.HealthCheckGrpcConnections()
    if err != nil {
        log.Errorf("gRPC connections unhealthy: %v", err)
    }
    
    // Get connection status
    status, err := subscribe.GetGrpcConnectionStatus()
    if err != nil {
        log.Errorf("Failed to get connection status: %v", err)
    } else {
        for service, state := range status {
            log.Infof("Service %s: %s", service, state)
        }
    }
}
```

### Metrics Collection

```go
package main

import (
    "github.com/go-lynx/lynx/app/subscribe"
    "github.com/go-lynx/lynx/plugins/service/grpc"
)

func main() {
    // Get metrics
    metrics, err := subscribe.GetGrpcMetrics()
    if err != nil {
        log.Errorf("Failed to get metrics: %v", err)
        return
    }
    
    // Access various metrics
    connectionCount := metrics.GetConnectionCount()
    activeConnections := metrics.GetActiveConnectionCount()
    requestCount := metrics.GetRequestCount()
    errorCount := metrics.GetErrorCount()
    
    log.Infof("Connections: %f, Active: %f, Requests: %f, Errors: %f", 
        connectionCount, activeConnections, requestCount, errorCount)
}
```

## Monitoring

The plugin provides comprehensive Prometheus metrics:

### Connection Metrics
- `grpc_client_connections_total`: Total number of gRPC client connections
- `grpc_client_connections_active`: Number of active gRPC client connections
- `grpc_client_connections_created_total`: Total number of connections created
- `grpc_client_connections_closed_total`: Total number of connections closed
- `grpc_client_connections_failed_total`: Total number of failed connection attempts

### Request Metrics
- `grpc_client_requests_total`: Total number of gRPC client requests
- `grpc_client_request_duration_seconds`: Duration of gRPC client requests
- `grpc_client_request_errors_total`: Total number of request errors

### Retry Metrics
- `grpc_client_retries_total`: Total number of retries
- `grpc_client_retry_duration_seconds`: Duration of retries

### Health Check Metrics
- `grpc_client_health_checks_total`: Total number of health checks
- `grpc_client_health_check_duration_seconds`: Duration of health checks

### Connection Pool Metrics
- `grpc_client_pool_size`: Number of connections per service (channel pool size)
- `grpc_client_pool_active`: Number of active connections in pool
- `grpc_client_pool_idle`: Number of idle connections in pool

### Multi-Channel Pool Features
- **Multiple Connections Per Service**: Each service can have multiple connections (channels) for better performance
- **Connection Selection Strategies**: 
  - Round-robin: Distributes requests evenly across connections
  - Random: Randomly selects a connection
  - Least-used: Selects the connection with the least usage
  - First-available: Selects the first healthy connection
- **Automatic Health Checks**: Unhealthy connections are automatically removed and replaced
- **Idle Connection Cleanup**: Idle connections are automatically closed after timeout

### Message Metrics
- `grpc_client_message_size_bytes`: Size of gRPC messages

### Circuit Breaker Metrics
- `grpc_client_circuit_breaker_state`: State of circuit breaker
- `grpc_client_circuit_breaker_trips_total`: Total number of circuit breaker trips

## Error Handling

The plugin provides comprehensive error handling:

### Retryable Errors
- `UNAVAILABLE`: Service unavailable
- `DEADLINE_EXCEEDED`: Request timeout
- `RESOURCE_EXHAUSTED`: Resource limits exceeded
- `ABORTED`: Request aborted
- `OUT_OF_RANGE`: Request out of range
- `INTERNAL`: Internal server error
- `DATA_LOSS`: Data loss error

### Non-Retryable Errors
- `UNAUTHENTICATED`: Authentication failed
- `PERMISSION_DENIED`: Permission denied
- `NOT_FOUND`: Resource not found
- `ALREADY_EXISTS`: Resource already exists
- `FAILED_PRECONDITION`: Precondition failed
- `INVALID_ARGUMENT`: Invalid argument

## Best Practices

1. **Connection Reuse**: Always reuse connections when possible to avoid overhead
2. **Timeout Configuration**: Set appropriate timeouts based on your service requirements
3. **Retry Configuration**: Configure retry logic based on your service characteristics
4. **Health Monitoring**: Regularly check connection health and metrics
5. **TLS Configuration**: Use TLS for production environments
6. **Metrics Collection**: Monitor metrics to identify performance issues
7. **Error Handling**: Implement proper error handling for different error types

## Troubleshooting

### Common Issues

1. **Connection Failures**: Check service discovery configuration and network connectivity
2. **TLS Errors**: Verify certificate configuration and TLS settings
3. **Timeout Issues**: Adjust timeout settings based on service response times
4. **Retry Loops**: Check retry configuration and error types
5. **Memory Issues**: Monitor connection pool size and adjust if necessary

### Debug Logging

Enable debug logging to troubleshoot issues:

```yaml
lynx:
  log:
    level: debug
```

### Health Check Endpoints

Use the health check endpoints to monitor connection status:

```go
// Check all connections
err := subscribe.HealthCheckGrpcConnections()

// Get specific connection status
status, err := subscribe.GetGrpcConnectionStatus()
```

## License

Apache License 2.0

