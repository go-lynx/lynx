# Lynx Application Core Package

<p align="center">
  <strong>Core Application Framework for Lynx Microservices</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/go-lynx/lynx/app"><img src="https://pkg.go.dev/badge/github.com/go-lynx/lynx/app" alt="GoDoc"></a>
  <a href="https://github.com/go-lynx/lynx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-lynx/lynx" alt="License"></a>
</p>

---

## 🚀 Overview

The **app** package is the heart of the Lynx framework, providing the core application infrastructure for building microservices. It manages the application lifecycle, plugin system, configuration, logging, and runtime environment in a unified and extensible way.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Lynx Application                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   LynxApp   │  │  Plugin     │  │  Control    │       │
│  │ (Singleton) │  │ Manager     │  │   Plane     │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Runtime   │  │  Factory    │  │  Config     │       │
│  │  System     │  │  System     │  │ Management  │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Logging   │  │     TLS     │  │ Subscribe  │       │
│  │   System    │  │  Support    │  │  System     │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

## ✨ Core Components

### 🎯 LynxApp - Application Core

The `LynxApp` struct is the central coordinator that manages all application components:

```go
type LynxApp struct {
    host               string                    // Application hostname
    name               string                    // Application name
    version            string                    // Application version
    cert               CertificateProvider       // TLS certificate management
    bootConfig         *conf.Bootstrap          // Bootstrap configuration
    globalConf         config.Config            // Global configuration
    controlPlane       ControlPlane             // Control interface
    pluginManager      TypedPluginManager       // Plugin lifecycle management
    typedPluginManager TypedPluginManager       // Type-safe plugin management
    grpcSubs           map[string]*grpc.ClientConn // gRPC subscriptions
    configVersion      uint64                   // Configuration version
}
```

**Key Features:**
- **Singleton Pattern**: Thread-safe global instance management
- **Configuration Management**: Centralized configuration handling
- **Plugin Coordination**: Unified plugin lifecycle management
- **Service Discovery**: Built-in service registration and discovery

### 🔌 Plugin Management System

The plugin system provides a flexible and extensible architecture:

#### Plugin Manager
```go
type PluginManager interface {
    LoadPlugins(config.Config) error
    UnloadPlugins()
    GetPlugin(name string) plugins.Plugin
    GetRuntime() plugins.Runtime
    SetConfig(config.Config)
    StopPlugin(pluginName string) error
    GetResourceStats() map[string]any
    ListResources() []*plugins.ResourceInfo
}
```

#### Typed Plugin Manager
- **Type Safety**: Generic-based plugin management
- **Concurrent Access**: Thread-safe operations with RW locks
- **Resource Management**: Automatic resource cleanup and statistics

### ⚙️ Runtime System

The runtime system manages event processing and resource coordination:

```go
type TypedRuntimePlugin struct {
    resources          sync.Map                  // Shared resources
    eventListeners     []listenerEntry          // Event listeners
    eventHistory       []plugins.PluginEvent    // Event history
    eventCh            chan plugins.PluginEvent // Event channel
    workerCount        int                      // Worker goroutines
    listenerQueueSize  int                      // Per-listener queue size
    maxHistorySize     int                      // History retention
    drainTimeout       time.Duration           // Shutdown timeout
}
```

**Runtime Features:**
- **Event-Driven Architecture**: Asynchronous event processing
- **Worker Pool**: Configurable worker goroutines
- **Event History**: Configurable event retention
- **Graceful Shutdown**: Controlled resource cleanup

### 🏭 Factory System

The factory system provides plugin creation and registration:

#### Plugin Registry
- **Global Registration**: Centralized plugin registration
- **Configuration Mapping**: Plugin-to-configuration binding
- **Type Safety**: Generic-based plugin creation

#### Typed Factory
```go
// Get global factory instance
typedFactory := factory.GlobalTypedFactory()

// Register type-safe plugins
factory.RegisterTypedPlugin(typedFactory, "redis", "cache", func() *redis.Plugin {
    return redis.New()
})

// Get typed plugin instances
redisPlugin, err := factory.GetTypedPlugin[*redis.Plugin](typedFactory, "redis")
```

## 🔧 Configuration

### Bootstrap Configuration

The application uses a bootstrap configuration system defined in `conf/boot.proto`:

```yaml
lynx:
  application:
    name: "my-service"
    version: "v1.0.0"
  
  polaris:
    namespace: "dev"
    token: "polaris-token"
    weight: 100
    ttl: 5
    timeout: "5s"
  
  runtime:
    event:
      queue_size: 1024
      worker_count: 10
      listener_queue_size: 256
      history_size: 1000
      drain_timeout_ms: 500
```

### Environment Variables

- `LYNX_LANG`: Language setting (zh/en)
- `LYNX_LOG_LEVEL`: Log level configuration
- `LYNX_LAYOUT_REPO`: Template repository URL

## 📖 Usage Examples

### Basic Application Initialization

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-kratos/kratos/v2/config"
)

func main() {
    // Create configuration
    cfg := config.New()
    
    // Initialize Lynx application
    lynxApp, err := app.NewApp(cfg)
    if err != nil {
        panic(err)
    }
    
    // Access global instance
    app := app.Lynx()
    
    // Get application info
    name := app.GetName()
    version := app.GetVersion()
    host := app.GetHost()
}
```

### Plugin Management

```go
// Load plugins from configuration
err := app.pluginManager.LoadPlugins(cfg)
if err != nil {
    log.Fatal(err)
}

// Get specific plugin
plugin := app.pluginManager.GetPlugin("redis")
if plugin != nil {
    // Use plugin
}

// Get plugin runtime
runtime := app.pluginManager.GetRuntime()
```

### Event System Usage

```go
// Create runtime plugin
runtime := app.NewTypedRuntimePlugin()

// Register event listener
runtime.AddEventListener("plugin.event", func(event plugins.PluginEvent) {
    // Handle event
})

// Emit events
runtime.Emit(plugins.PluginEvent{
    PluginID: "my-plugin",
    Type:     "started",
    Data:     map[string]interface{}{"status": "running"},
})
```

## 🚦 Application Lifecycle

### 1. Initialization
```go
// Parse bootstrap configuration
// Initialize core components
// Set up plugin manager
// Configure logging and TLS
```

### 2. Plugin Loading
```go
// Load plugins from configuration
// Resolve dependencies
// Initialize plugin instances
// Start plugin services
```

### 3. Runtime Operation
```go
// Handle configuration updates
// Process events
// Manage resources
// Monitor health status
```

### 4. Shutdown
```go
// Stop all plugins
// Drain event queues
// Clean up resources
// Graceful termination
```

## 🔍 Monitoring & Observability

### Metrics
- **Plugin Statistics**: Load/unload counts, resource usage
- **Event Metrics**: Queue sizes, processing rates
- **Runtime Performance**: Worker utilization, event latency

### Health Checks
- **Plugin Health**: Individual plugin status monitoring
- **System Health**: Overall application health status
- **Resource Health**: Memory, connection pool status

### Logging
- **Structured Logging**: JSON-formatted log output
- **Log Levels**: Configurable verbosity (error, warn, info, debug)
- **Context Logging**: Request-scoped logging support

## 🛡️ Security Features

### TLS Support
- **Certificate Management**: Automatic certificate provisioning
- **Secure Communication**: Encrypted service-to-service communication
- **Certificate Rotation**: Dynamic certificate updates

### Authentication
- **Plugin Authentication**: Secure plugin registration and access
- **Service Authentication**: Service-to-service authentication
- **Token Management**: Polaris token-based authentication

## 🔧 Development

### Building

```bash
# Build the application
go build -o lynx ./cmd/lynx

# Build with custom version
go build -ldflags "-X 'main.release=v1.2.3'" -o lynx ./cmd/lynx
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./app

# Run with coverage
go test -cover ./...
```

### Adding New Components

1. **Create Interface**: Define component interfaces in appropriate files
2. **Implement Component**: Create concrete implementations
3. **Register Component**: Add to plugin manager or factory system
4. **Add Configuration**: Extend configuration schemas as needed

## 📁 Directory Structure

```
app/
├── README.md              # This documentation
├── lynx.go               # Core application logic
├── runtime.go            # Runtime system implementation
├── plugin_manager.go     # Plugin management system
├── plugin_lifecycle.go   # Plugin lifecycle management
├── plugin_ops.go         # Plugin operations
├── plugin_topology.go    # Plugin dependency management
├── control_plane.go      # Control plane interface
├── configuration.go      # Configuration management
├── cert.go               # Certificate management
├── conf/                 # Configuration definitions
│   ├── boot.proto        # Bootstrap configuration schema
│   ├── boot.pb.go        # Generated protobuf code
│   └── boot-example.yml  # Example configuration
├── factory/              # Plugin factory system
│   ├── README.md         # Factory documentation
│   ├── interfaces.go     # Factory interfaces
│   ├── registry.go       # Plugin registry
│   └── typed_factory.go # Type-safe factory
├── log/                  # Logging system
├── kratos/               # Kratos framework integration
├── observability/        # Metrics and monitoring
├── subscribe/            # Subscription system
├── tls/                  # TLS configuration
└── util/                 # Utility functions
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](../../CONTRIBUTING.md) for details.

### Development Guidelines

1. **Code Style**: Follow Go best practices and project conventions
2. **Testing**: Add tests for new functionality
3. **Documentation**: Update documentation for API changes
4. **Backward Compatibility**: Maintain compatibility when possible

## 📄 License

This project is licensed under the [MIT License](../../LICENSE).

## 🔗 Related Links

- [Lynx Framework Documentation](https://go-lynx.cn/)
- [Go-Lynx Main Repository](https://github.com/go-lynx/lynx)
- [Kratos Framework](https://github.com/go-kratos/kratos)
- [Polaris Service Discovery](https://github.com/polarismesh/polaris)
- [Seata Distributed Transactions](https://github.com/seata/seata)

## 📞 Support

- **Discord**: [Join our community](https://discord.gg/2vq2Zsqq)
- **Issues**: [GitHub Issues](https://github.com/go-lynx/lynx/issues)
- **Documentation**: [https://go-lynx.cn/](https://go-lynx.cn/)

---

<p align="center">
  Made with ❤️ by the Lynx Community
</p>
