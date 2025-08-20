# Factory Package Improvements

## Overview

The `factory` package is responsible for creating and managing plugins in the Lynx framework. After refactoring, we improved naming, interface design, and separation of concerns to make it clearer and easier to use.

## File Structure

### 1. `interfaces.go` - Interface Definitions
- **Purpose**: Defines the core interfaces for plugin management
- **Key components**:
  - `Registry` interface: plugin registration management
  - `Creator` interface: plugin creation
  - `Factory` interface: full plugin management

### 2. `registry.go` - Plugin Registry
- **Purpose**: Implements plugin registration and configuration mapping
- **Key components**:
  - `PluginRegistry` struct: registry implementation
  - `GlobalPluginRegistry()` function: get the global registry instance
  - Register, unregister, and query capabilities

### 3. `typed_factory.go` - Type-Safe Factory
- **Purpose**: Provides type-safe plugin creation and management
- **Key components**:
  - `TypedFactory` struct: type-safe plugin factory
  - `RegisterTypedPlugin()` function: register type-safe plugins
  - `GetTypedPlugin()` function: get type-safe plugin instances
  - `GlobalTypedFactory()` function: get the global type-safe factory

## Key Improvements

### 1. Naming
- **Before**: `lynx_factory.go`, `plugin_factory.go`
- **After**: `registry.go`, `interfaces.go`, `typed_factory.go`
- **Benefit**: More intuitive names with clearer responsibilities

### 2. Interface Design
- **Problem**: `PluginFactory` interface carried too many responsibilities
- **Solution**:
  - `Registry` interface: focused on registration
  - `Creator` interface: focused on creation
  - `Factory` interface: composes the two

### 3. Type Naming
- **Before**: `LynxPluginFactory`, `TypedPluginFactory`
- **After**: `PluginRegistry`, `TypedFactory`
- **Benefit**: Avoid confusion with the Lynx framework itself; simpler naming

### 4. Separation of Concerns
- **Registry**: registration, deregistration, and queries
- **Factory**: creation and type safety
- **Interfaces**: clear contracts

### 5. Concurrency Safety
- `TypedFactory` uses RW locks to protect concurrent access
- Provides thread-safe plugin management

## Usage Examples

### Basic Registry Usage
```go
// Get the global registry
registry := factory.GlobalPluginRegistry()

// Register a plugin
registry.RegisterPlugin("http_server", "http", func() plugins.Plugin {
    return &httpServerPlugin{}
})

// Create a plugin
plugin, err := registry.CreatePlugin("http_server")
```

### Type-Safe Factory Usage
```golang
// Get the global type-safe factory
typedFactory := factory.GlobalTypedFactory()

// Register a type-safe plugin
factory.RegisterTypedPlugin(typedFactory, "redis", "cache", func() *redis.Plugin {
    return redis.New()
})

// Get a type-safe plugin instance
redisPlugin, err := factory.GetTypedPlugin[*redis.Plugin](typedFactory, "redis")
```

## Backward Compatibility

To maintain backward compatibility, we kept the following:
- `TypedFactory` implements the `Factory` interface
- Compatible methods for legacy APIs

## Interface Hierarchy

```
Factory (full capabilities)
├── Registry (registration)
│   ├── RegisterPlugin()
│   ├── UnregisterPlugin()
│   ├── GetPluginRegistry()
│   └── HasPlugin()
└── Creator (creation)
    └── CreatePlugin()
```

## Design Principles

1. **Single Responsibility**: each interface and struct has a clear responsibility
2. **Interface Segregation**: split large interfaces into smaller specialized ones
3. **Type Safety**: generics to ensure type safety
4. **Concurrency Safety**: proper locking to protect shared state
5. **Backward Compatibility**: keep compatibility with existing code
