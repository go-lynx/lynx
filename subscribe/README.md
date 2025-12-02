# Subscribe Package

The `subscribe` package provides gRPC service subscription and connection management for the Lynx framework. It enables applications to connect to upstream gRPC services with service discovery, TLS support, and connection health monitoring.

## Overview

This package handles:

- **gRPC Service Subscription** - Connect to upstream services by name
- **Service Discovery Integration** - Automatic service node discovery
- **TLS Support** - Secure connections with certificate management
- **Connection Health Monitoring** - Automatic connection state tracking
- **Load Balancing** - Node filtering and routing support

## File Structure

| File | Description |
|------|-------------|
| `subscribe.go` | Core `GrpcSubscribe` struct and connection management |
| `loader.go` | Configuration-based subscription loading |
| `loader_integrated.go` | Integrated loader with framework support |
| `tls.go` | TLS configuration and certificate handling |

## Usage

### Basic Subscription

```go
import (
    "github.com/go-lynx/lynx/subscribe"
)

// Create a gRPC subscription
sub := subscribe.NewGrpcSubscribe(
    subscribe.WithServiceName("user-service"),
    subscribe.WithDiscovery(discoveryInstance),
)

// Connect to the service
conn, err := sub.NewGrpcConn()
if err != nil {
    return err
}
defer conn.Close()

// Use the connection
client := pb.NewUserServiceClient(conn)
```

### With TLS

```go
sub := subscribe.NewGrpcSubscribe(
    subscribe.WithServiceName("secure-service"),
    subscribe.WithDiscovery(discoveryInstance),
    subscribe.EnableTls(),
    subscribe.WithRootCAFileName("ca.crt"),
    subscribe.WithRootCAFileGroup("certificates"),
)

conn, err := sub.NewGrpcConn()
```

### Required Dependencies

Mark a service as required to ensure it's available at startup:

```go
sub := subscribe.NewGrpcSubscribe(
    subscribe.WithServiceName("critical-service"),
    subscribe.WithDiscovery(discoveryInstance),
    subscribe.Required(), // Will fail startup if unavailable
)
```

### With Custom Routing

```go
sub := subscribe.NewGrpcSubscribe(
    subscribe.WithServiceName("my-service"),
    subscribe.WithDiscovery(discoveryInstance),
    subscribe.WithRouterFactory(func(service string) selector.NodeFilter {
        return myCustomNodeFilter
    }),
)
```

## Configuration

### YAML Configuration

```yaml
lynx:
  subscriptions:
    grpc:
      - service: "user-service"
        required: true
        tls: false
      - service: "payment-service"
        required: true
        tls: true
        ca_name: "payment-ca.crt"
        ca_group: "certificates"
      - service: "notification-service"
        required: false
        tls: false
```

### Proto Definition

```protobuf
message GrpcSubscription {
  string service = 1;    // Service name in service discovery
  bool required = 2;     // Strong dependency check at startup
  bool tls = 3;          // Enable TLS
  string ca_name = 4;    // CA certificate filename
  string ca_group = 5;   // CA certificate file group
}
```

## Options

| Option | Description |
|--------|-------------|
| `WithServiceName(name)` | Set the service name to subscribe to |
| `WithDiscovery(discovery)` | Set the service discovery instance |
| `EnableTls()` | Enable TLS encryption |
| `WithRootCAFileName(name)` | Set the root CA certificate filename |
| `WithRootCAFileGroup(group)` | Set the CA certificate file group |
| `Required()` | Mark as required dependency |
| `WithRouterFactory(factory)` | Set custom node routing |
| `WithConfigProvider(provider)` | Set configuration source provider |
| `WithDefaultRootCA(provider)` | Set default root CA provider |

## Connection States

The package monitors gRPC connection states:

| State | Description |
|-------|-------------|
| `IDLE` | Connection is idle |
| `CONNECTING` | Establishing connection |
| `READY` | Connection is ready |
| `TRANSIENT_FAILURE` | Temporary failure, will retry |
| `SHUTDOWN` | Connection is shut down |

## Built-in Middleware

Connections are created with standard middleware:

- **Tracing** - Distributed tracing support
- **Logging** - Request/response logging
- **Recovery** - Panic recovery

## Integration with Lynx

The subscription system integrates with the Lynx framework:

```go
// In your plugin
func (p *MyPlugin) InitializeResources(rt plugins.Runtime) error {
    // Subscriptions are automatically built from configuration
    // Access them through the application context
    return nil
}
```

## Best Practices

1. **Mark critical services as required** - Ensure dependencies are available at startup
2. **Use TLS in production** - Enable encryption for secure communication
3. **Configure appropriate timeouts** - Prevent hanging connections
4. **Monitor connection states** - Track connectivity for observability
5. **Use service discovery** - Avoid hardcoded endpoints

## License

Apache License 2.0

