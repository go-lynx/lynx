# gRPC Plugin for Lynx Framework

This plugin provides gRPC server functionality for the Lynx framework, offering features such as TLS support, middleware integration, and configuration management.

## Features

- Full gRPC server implementation
- TLS support with client authentication
- Built-in middleware support:
  - Tracing (OpenTelemetry)
  - Logging
  - Rate limiting
  - Request validation
  - Panic recovery
- Dynamic configuration
- Health checking
- Graceful shutdown

## Installation

```bash
go get github.com/go-lynx/plugin-grpc/v2
```

## Configuration

The plugin can be configured through the Lynx configuration system. Here's an example configuration:

```yaml
lynx:
  server:
    network: "tcp"
    addr: ":9090"
    timeout: "1s"
    tls: true
    tls_auth_type: 4  # Mutual TLS authentication
```

### Configuration Options

- `network`: Network type (default: "tcp")
- `addr`: Server address (default: ":9090")
- `timeout`: Request timeout duration
- `tls`: Enable/disable TLS
- `tls_auth_type`: TLS authentication type
  - 0: No client authentication
  - 1: Request client certificate
  - 2: Require any client certificate
  - 3: Verify client certificate if given
  - 4: Require and verify client certificate

## Usage

### Basic Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/plugin-grpc/v2"
    pb "your/protobuf/package"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // The gRPC plugin will be automatically registered and initialized
    
    // Get the gRPC server instance
    server := grpc.GetServer()
    
    // Register your gRPC service
    pb.RegisterYourServiceServer(server, &YourServiceImpl{})
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

### With TLS

To use TLS, you need to:

1. Enable TLS in configuration
2. Provide certificates through the Lynx certificate management system
3. Configure client authentication type if needed

```go
// Your certificates will be automatically loaded from the configuration
// and applied to the gRPC server
```

### Custom Middleware

The plugin comes with several built-in middleware options. You can also add your own middleware:

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/plugin-grpc/v2"
    "google.golang.org/grpc"
)

func main() {
    // Initialize your application
    application := app.NewApplication()
    
    // Get the gRPC server
    server := grpc.GetServer()
    
    // Add your custom middleware
    server.Use(YourCustomMiddleware())
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}

func YourCustomMiddleware() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        // Your middleware logic here
        return handler(ctx, req)
    }
}
```

## Health Checking

The plugin implements health checking through the Lynx plugin system. You can monitor the gRPC server's health status through your application's health checking mechanism.

## Dependencies

- github.com/go-kratos/kratos/v2
- github.com/go-lynx/lynx
- google.golang.org/grpc

## License

Apache License 2.0
