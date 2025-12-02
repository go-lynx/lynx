# Configuration Package

The `conf` package contains Protocol Buffer definitions for the Lynx framework's bootstrap configuration. These proto files define the structure of application configuration that is loaded at startup.

## Overview

This package provides:

- **Bootstrap Configuration** - Top-level application configuration structure
- **Application Settings** - Name, version, host, and banner settings
- **Service Subscriptions** - Upstream gRPC service dependencies
- **Runtime Configuration** - Event system and runtime settings

## File Structure

| File | Description |
|------|-------------|
| `boot.proto` | Protocol Buffer definition for bootstrap configuration |
| `boot.pb.go` | Generated Go code from boot.proto |
| `boot-example.yml` | Example configuration file |

## Configuration Structure

### Bootstrap

The top-level configuration message:

```protobuf
message Bootstrap {
  Lynx lynx = 1;  // Lynx framework configuration
}
```

### Lynx Configuration

```protobuf
message Lynx {
  Application application = 1;      // Application settings
  Subscriptions subscriptions = 2;  // Service subscriptions
  Runtime runtime = 3;              // Runtime configuration
}
```

### Application Settings

```protobuf
message Application {
  string name = 1;         // Application name
  string version = 2;      // Application version
  string host = 3;         // Host address
  bool close_banner = 4;   // Disable startup banner
}
```

### Service Subscriptions

```protobuf
message Subscriptions {
  repeated GrpcSubscription grpc = 1;  // gRPC services to subscribe
}

message GrpcSubscription {
  string service = 1;     // Service name in discovery
  bool required = 2;      // Required at startup
  bool tls = 3;           // Enable TLS
  string ca_name = 4;     // CA certificate filename
  string ca_group = 5;    // CA certificate group
}
```

### Runtime Configuration

```protobuf
message Runtime {
  Event event = 1;  // Event system configuration
}

message Event {
  int32 queue_size = 1;           // Event queue size
  int32 worker_count = 2;         // Event worker goroutines
  int32 listener_queue_size = 3;  // Listener queue size
  int32 history_size = 4;         // Event history size
  int32 drain_timeout_ms = 5;     // Shutdown drain timeout
}
```

## Example Configuration

### YAML Format

```yaml
# bootstrap.yaml
lynx:
  application:
    name: "my-service"
    version: "1.0.0"
    host: "0.0.0.0"
    close_banner: false
  
  subscriptions:
    grpc:
      - service: "user-service"
        required: true
        tls: false
      - service: "payment-service"
        required: true
        tls: true
        ca_name: "payment-ca.crt"
        ca_group: "certs"
  
  runtime:
    event:
      queue_size: 1024
      worker_count: 10
      listener_queue_size: 256
      history_size: 1000
      drain_timeout_ms: 500
```

## Usage in Go

### Loading Configuration

```go
import (
    "github.com/go-kratos/kratos/v2/config"
    "github.com/go-kratos/kratos/v2/config/file"
    "github.com/go-lynx/lynx/conf"
)

func loadConfig() (*conf.Bootstrap, error) {
    c := config.New(
        config.WithSource(file.NewSource("bootstrap.yaml")),
    )
    
    if err := c.Load(); err != nil {
        return nil, err
    }
    
    var bc conf.Bootstrap
    if err := c.Scan(&bc); err != nil {
        return nil, err
    }
    
    return &bc, nil
}
```

### Accessing Configuration

```go
bootstrap, _ := loadConfig()

// Application info
appName := bootstrap.Lynx.Application.Name
appVersion := bootstrap.Lynx.Application.Version

// Subscriptions
for _, sub := range bootstrap.Lynx.Subscriptions.Grpc {
    fmt.Printf("Service: %s, Required: %v\n", sub.Service, sub.Required)
}

// Event configuration
eventQueueSize := bootstrap.Lynx.Runtime.Event.QueueSize
```

## Regenerating Go Code

To regenerate the Go code from proto files:

```bash
make config
```

Or manually:

```bash
protoc --proto_path=conf \
       --go_out=paths=source_relative:conf \
       conf/boot.proto
```

## Configuration Sources

The Lynx framework supports multiple configuration sources:

| Source | Description |
|--------|-------------|
| Local File | YAML/JSON files on disk |
| Polaris | Tencent Polaris config center |
| Nacos | Alibaba Nacos config center |
| Apollo | Apollo config center |
| Environment | Environment variables |

## Best Practices

1. **Use meaningful names** - Application name should match service discovery registration
2. **Set appropriate versions** - Use semantic versioning
3. **Configure subscriptions** - Declare all upstream dependencies
4. **Tune event system** - Adjust queue sizes based on load
5. **Enable TLS in production** - Use encrypted connections
6. **Use config centers** - Avoid hardcoded configuration in production

## Related Packages

- `boot/` - Bootstrap and application startup
- `subscribe/` - gRPC subscription implementation
- `events/` - Event system implementation

## License

Apache License 2.0

