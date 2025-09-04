# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go-Lynx is a plugin-based Go microservice framework built on Seata, Polaris, and Kratos. The framework adopts a layered Runtime architecture design, providing unified resource management, event systems, and plugin lifecycle management.

## Development Commands

### Common Make Commands
```bash
# Initialize development environment (install protoc, wire, and other tools)
make init

# Generate protobuf configuration files
make config

# View all available commands
make help

# Module version tagging and release
make tag MODULES_VERSION=v2.0.0 MODULES="plugins/xxx plugins/yyy"
make push-tags MODULES_VERSION=v2.0.0 MODULES="plugins/xxx plugins/yyy"
make release MODULES_VERSION=v2.0.0 MODULES="plugins/xxx plugins/yyy"
```

### Testing Commands
```bash
# Run all tests
go test -v ./...

# Run tests for specific packages
go test -v ./app/cache/
go test -v ./app/events/
go test -v ./plugins/polaris/

# Run benchmark tests
go test -bench=. -benchmem ./app/events/

# Run integration tests
go test -v ./app/cache/integration_test.go
```

### CLI Tool Commands
```bash
# Install CLI tool
go install github.com/go-lynx/lynx/cmd/lynx@latest

# Create new project
lynx new my-service

# Create multiple projects
lynx new service1 service2 service3

# Create project with specific configuration
lynx new demo --module github.com/acme/demo --post-tidy --ref v1.2.3

# CLI environment variable configuration
# LYNX_LOG_LEVEL: error|warn|info|debug (default: info)
# LYNX_QUIET: 1/true for error-only output
# LYNX_VERBOSE: 1/true for detailed output
# LYNX_LANG: zh|en multi-language settings
```

### Code Quality Checks
```bash
# Use Qodana for code quality checks (configured in qodana.yaml)
# Project uses jetbrains/qodana-go:2025.1 for static code analysis
```

## Layered Runtime Architecture

The project adopts a four-layer architecture design:

1. **Application Layer** - LynxApp, Boot, Control Plane
2. **Plugin Management Layer** - PluginManager, TypedPluginManager, PluginFactory  
3. **Runtime Layer** - Runtime interface, TypedRuntimePlugin, SimpleRuntime
4. **Resource Management Layer** - Private/Shared Resources, Resource Info

### Core Module Structure
- `app/` - Framework core components
  - `plugin_manager.go` - Plugin manager, implements DefaultPluginManager[T]
  - `runtime.go` - TypedRuntimePlugin, provides resource management and event system
  - `events/` - Unified event system (EventBusManager)
  - `cache/` - Cache management system, based on Ristretto
  - `utils/` - Utility packages (errx, netx, auth/jwt, collection, etc.)
- `boot/` - Application startup and configuration management
- `plugins/` - Plugin implementation directory, each plugin has independent go.mod
- `cmd/lynx/` - CLI tool implementation

### Plugin System Features
- Each plugin has `plug.go` as entry point, using factory.RegisterPlugin for registration
- Plugin configuration uses protobuf definitions
- Supports hot-plugging and complete lifecycle management
- Provides type-safe generic resource access

## Development Standards

### Plugin Development Conventions
- Implement `plugins.Plugin` interface, including Name(), Initialize(), Start(), Stop() and other methods
- Use `factory.GlobalTypedFactory().RegisterPlugin()` in `plug.go` for plugin registration
- Configuration structures use protobuf definitions, supporting YAML format configuration files
- Plugin example code references `examples/production_ready/main.go`

### Testing Conventions
- Unit tests: `*_test.go`
- Integration tests: `integration_test.go` 
- Performance tests: `benchmark_test.go`
- Example tests: `example_test.go`

### Error Handling and Recovery
- Framework provides ErrorRecoveryManager for error recovery
- Supports custom RecoveryStrategy
- Integrates production-level monitoring metrics (ProductionMetrics)

## Important Implementation Details

### Resource Management
- Private resources: Independent resource namespace for each plugin
- Shared resources: Global resources shared by all plugins
- Type safety: Uses generics `GetTypedResource[T]` and `RegisterTypedResource[T]`
- Complete resource tracking and statistics

### Event System
- Unified event bus: EventBusManager manages inter-plugin communication
- Event isolation: Plugin namespace avoids event conflicts  
- Supports event filtering, history records, and concurrent safe processing
- Compatible with legacy plugin event interfaces

### Go Modules Structure
- Root module: `github.com/go-lynx/lynx`
- Each plugin has independent go.mod file for version management
- Uses Go 1.24.3 version
- Core dependencies: Kratos v2.8.4, Zerolog, Prometheus, Ristretto, etc.

## Monitoring and Observability

- Production-level metrics collection: `app/observability/metrics.ProductionMetrics`
- Supports plugin health checks and latency statistics
- Built-in Grafana dashboard configurations (`grafana/` directory)
- Event publishing and request processing metrics

## Important Notes

- When running tests, some plugin tests may fail due to external dependencies, which is normal
- Plugins support hot updates, ensure proper resource isolation and cleanup during development
- Event system design avoids circular dependencies
- Distributed transaction plugins (DTM, Seata) require corresponding external service support