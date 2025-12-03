# Changelog

## [v1.5.0-beta] - 2024-12-02

### 🎯 Breaking Changes

#### Architecture Refactoring
- **Plugin Independence**: Redesigned the overall architecture and fully separated plugins into standalone Git repositories for better modularity and independent version control
- **TypedPluginManager**: Refactored plugin manager interfaces to use `TypedPluginManager` for type-safe plugin management
- **Unified Runtime Implementation**: Introduced unified Runtime implementation, simplifying the runtime system architecture

### ✨ New Features

#### Configuration Center
- **Apollo Configuration Plugin**: Added Apollo configuration center plugin for Lynx framework
- **Polaris Direct Initialization**: Support initializing Polaris SDK context directly from YAML config file
- **Enhanced Configuration Loading**: Enhanced configuration loading with priority and merge strategies

#### Snowflake ID Generator
- **New Snowflake Plugin**: Added snowflake ID generator plugin with:
  - Redis worker node auto-registration
  - Clock drift protection mechanism
  - Configuration validation and fault injection tests

#### gRPC Service
- **Enhanced Connection Pooling**: Multi-channel support and selection strategies
- **Port Availability Caching**: Implemented port availability check caching for improved reliability
- **Message Size Limits**: Added message size limits configuration with sensible defaults
- **gRPC Metadata Toolkit (grpcx)**: Added gRPC metadata operation toolkit
- **Dependency Injection Integration**: Integrated dependency injection for gRPC client and service
- **Health Checks**: Enhanced gRPC service with monitoring, configuration validation, and health checks

#### HTTP Service
- **Circuit Breaker Pattern**: Implemented circuit breaker pattern for HTTP service

#### Plugin System
- **Resource Statistics**: Enhanced plugin management with resource statistics and unload failure tracking
- **Event History**: Added event history retrieval methods for plugins
- **Event Bus Adapter**: Implemented plugin event bus adapter with filter conversion and listener management
- **Configurable Drop Policies**: Added configurable drop policies and context-aware event listeners

#### Logging System
- **Proxy Logger**: Added proxy logger for hot-swappable logging in Kratos app
- **Atomic Timezone Management**: Migrated logging to atomic timezone and consistent level management

#### CLI Tools
- **Doctor Auto-Fix**: Implemented auto-fix functionality for `lynx doctor` command with category-specific fixes

### 🔧 Improvements

#### Performance Optimization
- **Cache Optimization**: Added cache optimization and logging performance monitoring
- **gRPC Connection Management**: Enhanced gRPC connection management and performance settings
- **Connection Pool Retry**: Enhanced connection pool with retry logic and improved cleanup

#### Stability Enhancement
- **Error Recovery Refactoring**: Streamlined error recovery context management
- **Core Architecture Optimization**: Optimized core architecture to improve system resilience and robustness
- **Health Check Enhancement**: Improved health check management and concurrency safety
- **Nacos Atomic Checks**: Enhanced Nacos config and registry watchers with atomic closure checks

#### Code Quality
- **Error Handling Optimization**: Return errors instead of panics for plugin retrieval functions
- **Elasticsearch/MongoDB**: Improved error handling and logging in Elasticsearch and MongoDB plugins
- **Code Formatting**: Cleaned up code formatting and improved readability across multiple files

### 🐛 Bug Fixes

#### Snowflake ID
- Fixed Snowflake ID concurrency issue in creation scenario
- Fixed worker ID registration details in Snowflake plugin
- Simplified sequence cache handling in generator

#### Logging System
- Fixed AsyncLogWriter to prevent duplicate log entries during queue migration
- Enhanced AsyncLogWriter with retry mechanism for queue writes

#### Plugin System
- Fixed plugin removal in dependency graph
- Updated Redis test configuration

#### Database Plugins
- Updated MySQL, PostgreSQL, MSSQL service configurations

### 🔒 Security

- **TLS 1.2+ Enforcement**: Enforce TLS 1.2+ for all secure communications
- **Removed Insecure Cipher Suites**: Removed insecure cipher suites from gRPC transport
- **TLS Configuration Enhancement**: Enhanced TLS configuration handling

### 📦 Dependencies

- **Go 1.25**: Upgraded Go version to 1.25 and updated toolchain across all modules
- **Kratos v2.9.1**: Upgraded Kratos framework to v2.9.1
- **Protobuf Update**: Updated protoc-gen-go and protoc versions across multiple files
- **yaml.v3**: Added gopkg.in/yaml.v3 v3.0.1

### 📝 Documentation

- Updated TLS README for enhanced clarity and accuracy
- Updated comments for clarity and consistency across multiple files
- Removed obsolete analysis reports and documentation

### 🗑️ Deprecated

- Removed deprecated gRPC server implementation and related tests
- Removed unused binary files

### 🏗️ Infrastructure

- Merged test and production environment docker-compose configurations
- Added entgo dependency support
- Implemented comprehensive testing and validation for SQL plugins

---

## Upgrade Guide

### Upgrading from v1.2.3 to v1.5.0-beta

1. **Update Go Version**
   ```bash
   # Ensure Go 1.25+ is installed
   go version
   ```

2. **Update Dependencies**
   ```bash
   go get github.com/go-lynx/lynx@v1.5.0-beta
   go mod tidy
   ```

3. **Plugin Migration**
   - Plugins are now in separate Git repositories
   - Update plugin import paths if necessary

4. **Configuration Updates**
   - Review gRPC configuration for message size limits
   - Ensure TLS configuration meets new security requirements

5. **API Changes**
   - Migrate from `PluginManager` to `TypedPluginManager` if applicable
   - Check error handling: some functions now return errors instead of panicking

---

## Contributors

Thanks to all contributors who made this release possible!

---

[Full Commit History](https://github.com/go-lynx/lynx/compare/v1.2.3...v1.5.0-beta)
