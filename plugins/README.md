# Lynx Plugin SDK

The `plugins` package provides the core plugin system SDK for the Lynx framework. It defines interfaces, base implementations, and utilities for building extensible plugins.

## Overview

This package contains:

- **Plugin Interface** - Core interface that all plugins must implement
- **BasePlugin** - Base implementation providing common plugin functionality
- **Runtime Interface** - Runtime environment for plugins to access resources and events
- **Event System** - Event types and handling for inter-plugin communication
- **Dependency Management** - Plugin dependency resolution and version constraints
- **Health Checks** - Health monitoring and reporting utilities

## File Structure

| File | Description |
|------|-------------|
| `plugin.go` | Core `Plugin` interface, `Runtime` interface, and related types |
| `base.go` | `TypedBasePlugin` and `BasePlugin` base implementations |
| `unified_runtime.go` | `UnifiedRuntime` implementation with resource and event management |
| `events.go` | Event types and priority definitions |
| `event_adapter.go` | Event bus adapter for unified event handling |
| `deps.go` | `Dependency`, `DependencyGraph`, and version constraint utilities |
| `errors.go` | Error types and error handling utilities |
| `health.go` | `HealthReport` and health check utilities |
| `version.go` | Version comparison and semantic versioning utilities |
| `id.go` | Plugin ID generation and validation utilities |
| `conflict_resolver.go` | Plugin conflict resolution strategies |
| `upg.go` | Plugin upgrade capabilities and strategies |

## Core Interfaces

### Plugin Interface

```go
type Plugin interface {
    Metadata      // ID, Name, Version, Description
    Lifecycle     // Initialize, Start, Stop
    LifecycleSteps // InitializeResources, StartupTasks, CleanupTasks
    DependencyAware // GetDependencies, AddDependency
}
```

### Runtime Interface

```go
type Runtime interface {
    // Resource Management
    GetSharedResource(name string) (any, error)
    RegisterSharedResource(name string, resource any) error
    GetPrivateResource(name string) (any, error)
    RegisterPrivateResource(name string, resource any) error
    
    // Configuration
    GetConfig() config.Config
    SetConfig(config.Config)
    
    // Event System
    EmitEvent(event PluginEvent)
    AddListener(listener EventListener, filter *EventFilter)
    RemoveListener(listener EventListener)
    
    // Plugin Context
    WithPluginContext(pluginName string) Runtime
    GetCurrentPluginContext() string
}
```

## Plugin Lifecycle

```
┌─────────────┐
│  Inactive   │  Plugin loaded, not initialized
└──────┬──────┘
       │ Initialize()
       ▼
┌─────────────┐
│Initializing │  Setting up resources
└──────┬──────┘
       │ Start()
       ▼
┌─────────────┐
│   Active    │  Fully operational
└──────┬──────┘
       │ Stop()
       ▼
┌─────────────┐
│  Stopping   │  Cleaning up
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Terminated  │  Shutdown complete
└─────────────┘
```

### Plugin Status

| Status | Description |
|--------|-------------|
| `StatusInactive` | Loaded but not initialized |
| `StatusInitializing` | Currently initializing |
| `StatusActive` | Fully operational |
| `StatusSuspended` | Temporarily paused |
| `StatusStopping` | Shutting down |
| `StatusTerminated` | Gracefully shut down |
| `StatusFailed` | Fatal error occurred |
| `StatusUpgrading` | Being upgraded |
| `StatusRollback` | Rolling back from failed upgrade |

## Creating a Plugin

### Basic Plugin

```go
package myplugin

import (
    "github.com/go-lynx/lynx/plugins"
)

type MyPlugin struct {
    *plugins.BasePlugin
    // Your fields here
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{
        BasePlugin: plugins.NewBasePlugin(
            "my-plugin-v1",       // ID
            "my-plugin",          // Name
            "My awesome plugin",  // Description
            "1.0.0",              // Version
            "lynx.myplugin",      // Config prefix
            100,                  // Weight (higher = load first)
        ),
    }
}

// InitializeResources sets up plugin resources
func (p *MyPlugin) InitializeResources(rt plugins.Runtime) error {
    // Access shared resources
    db, err := rt.GetSharedResource("database")
    if err != nil {
        return err
    }
    
    // Register your own resources
    return rt.RegisterSharedResource("my-service", myService)
}

// StartupTasks performs startup logic
func (p *MyPlugin) StartupTasks() error {
    // Start your services
    return nil
}

// CleanupTasks performs cleanup logic
func (p *MyPlugin) CleanupTasks() error {
    // Clean up resources
    return nil
}

// CheckHealth performs health check
func (p *MyPlugin) CheckHealth() error {
    // Check if plugin is healthy
    return nil
}
```

### Typed Plugin (with type safety)

```go
type MyTypedPlugin struct {
    *plugins.TypedBasePlugin[*MyConfig]
}

func NewMyTypedPlugin(config *MyConfig) *MyTypedPlugin {
    return &MyTypedPlugin{
        TypedBasePlugin: plugins.NewTypedBasePlugin(
            "my-typed-plugin-v1",
            "my-typed-plugin",
            "Type-safe plugin",
            "1.0.0",
            "lynx.mytypedplugin",
            100,
            config, // Type-safe config
        ),
    }
}

// Access typed config
func (p *MyTypedPlugin) GetConfig() *MyConfig {
    return p.GetTypedInstance()
}
```

### Context-Aware Plugin

The default `TypedBasePlugin` implementations of `StartContext`, `StopContext`, and `InitializeContext` only run the non-context method in a goroutine and return when the context is done; they do **not** cancel the running Initialize/Start/Stop. For real cancellation, implement `LifecycleWithContext` on your plugin and check `ctx.Done()` inside your logic.

For plugins that need to respect context cancellation and timeouts:

```go
type ContextAwarePlugin struct {
    *plugins.BasePlugin
}

// InitializeContext with context support
func (p *ContextAwarePlugin) InitializeContext(ctx context.Context, plugin plugins.Plugin, rt plugins.Runtime) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        return p.Initialize(plugin, rt)
    }
}

// StartContext with context support
func (p *ContextAwarePlugin) StartContext(ctx context.Context, plugin plugins.Plugin) error {
    // Use ctx for cancellation
    return nil
}

// StopContext with context support
func (p *ContextAwarePlugin) StopContext(ctx context.Context, plugin plugins.Plugin) error {
    return nil
}

// IsContextAware marks plugin as truly context-aware
func (p *ContextAwarePlugin) IsContextAware() bool {
    return true
}
```

## Dependencies

### Dependency declaration timing

**Important:** The framework resolves load order by calling `GetDependencies()` on each plugin *before* any plugin is initialized. Therefore:

- Declare **required** dependencies in your plugin's **constructor** (e.g. in `NewMyPlugin()`), or in a method that runs before the plugin list is passed to `TopologicalSort`.
- Do **not** rely on adding dependencies only inside `InitializeResources(rt)` for load-order resolution; by then the sort has already been done. You may still call `AddDependency` in `InitializeResources` for optional/bookkeeping purposes, but required dependencies used for ordering must be available from `GetDependencies()` as soon as the plugin is created.

This ensures the dependency graph is complete when the manager builds the topological order for initialization and start.

### Declaring Dependencies

```go
// Recommended: declare required dependencies in constructor so GetDependencies() is complete before TopologicalSort.
func NewMyPlugin() *MyPlugin {
    p := &MyPlugin{
        BasePlugin: plugins.NewBasePlugin(...),
    }
    p.AddDependency(plugins.Dependency{
        ID: "database-plugin-v1",
        Name: "database-plugin",
        Type: plugins.DependencyTypeRequired,
        Required: true,
        VersionConstraint: &plugins.VersionConstraint{MinVersion: "1.0.0", MaxVersion: "2.0.0"},
    })
    return p
}

func (p *MyPlugin) InitializeResources(rt plugins.Runtime) error {
    // Optional: add more dependencies here only if they are not needed for load order
    p.AddDependency(plugins.Dependency{
        ID:       "cache-plugin-v1",
        Name:     "cache-plugin",
        Type:     plugins.DependencyTypeOptional,
        Required: false,
    })
    return nil
}
```

### Dependency Types

| Type | Description |
|------|-------------|
| `DependencyTypeRequired` | Must be present and loaded first |
| `DependencyTypeOptional` | Optional, loaded if available |
| `DependencyTypeConflicts` | Cannot coexist with this plugin |
| `DependencyTypeProvides` | This plugin provides the capability |

### Version Constraints

```go
VersionConstraint{
    MinVersion:      "1.0.0",          // Minimum version (>=)
    MaxVersion:      "2.0.0",          // Maximum version (<=)
    ExactVersion:    "1.5.0",          // Exact version required
    ExcludeVersions: []string{"1.3.0"}, // Excluded versions
}
```

## Event System

### Event Types

**Lifecycle Events:**
- `EventPluginInitializing`, `EventPluginInitialized`
- `EventPluginStarting`, `EventPluginStarted`
- `EventPluginStopping`, `EventPluginStopped`

**Health Events:**
- `EventHealthCheckStarted`, `EventHealthCheckDone`, `EventHealthCheckFailed`
- `EventHealthStatusOK`, `EventHealthStatusWarning`, `EventHealthStatusCritical`

**Configuration Events:**
- `EventConfigurationChanged`, `EventConfigurationApplied`, `EventConfigurationInvalid`

**Upgrade Events:**
- `EventUpgradeInitiated`, `EventUpgradeCompleted`, `EventUpgradeFailed`
- `EventRollbackInitiated`, `EventRollbackCompleted`, `EventRollbackFailed`

### Emitting Events

```go
p.EmitEvent(plugins.PluginEvent{
    Type:     plugins.EventPluginStarted,
    Priority: plugins.PriorityNormal,
    Source:   "MyPlugin",
    Category: "lifecycle",
    Metadata: map[string]any{
        "startup_time_ms": startupTime,
    },
})
```

### Listening for Events

```go
type MyListener struct {
    id string
}

func (l *MyListener) HandleEvent(event plugins.PluginEvent) {
    // Handle the event
}

func (l *MyListener) GetListenerID() string {
    return l.id
}

// Register listener
runtime.AddListener(&MyListener{id: "my-listener"}, &plugins.EventFilter{
    Types: []plugins.EventType{plugins.EventPluginStarted},
})
```

## Resource Management

### Shared Resources

```go
// Register shared resource (accessible by all plugins)
runtime.RegisterSharedResource("database", dbConnection)

// Get shared resource
db, err := runtime.GetSharedResource("database")
```

### Private Resources

```go
// Get plugin-scoped runtime
pluginRuntime := runtime.WithPluginContext("my-plugin")

// Register private resource (isolated per plugin)
pluginRuntime.RegisterPrivateResource("local-cache", cache)

// Get private resource
cache, err := pluginRuntime.GetPrivateResource("local-cache")
```

### Type-Safe Resource Access

```go
// Get resource with type safety
db, err := plugins.GetTypedResource[*sql.DB](runtime, "database")
if err != nil {
    return err
}
// db is already *sql.DB type
```

## Health Reporting

```go
type HealthReport struct {
    Status    string          // "healthy", "unhealthy", "suspended", etc.
    Message   string          // Human-readable message
    Details   map[string]any  // Additional details
    Timestamp int64           // Unix timestamp
}

// Get health report
report := plugin.GetHealth()
```

## Upgrade Capabilities

```go
// Supported upgrade capabilities
const (
    UpgradeNone    // No runtime upgrades supported
    UpgradeConfig  // Can update config without restart
    UpgradeVersion // Can upgrade version without restart
    UpgradeReplace // Can be completely replaced at runtime
)

// Prepare for upgrade
plugin.PrepareUpgrade("2.0.0")

// Execute upgrade
plugin.ExecuteUpgrade("2.0.0")

// Rollback if needed
plugin.RollbackUpgrade("1.0.0")
```

## Best Practices

1. **Always declare dependencies** - Use `AddDependency()` for explicit dependency management
2. **Implement health checks** - Override `CheckHealth()` for proper monitoring
3. **Use context-aware methods** - Implement `LifecycleWithContext` for timeout support
4. **Emit lifecycle events** - Help other plugins track your state
5. **Clean up resources** - Implement proper `CleanupTasks()` to avoid leaks
6. **Use typed resources** - Prefer `GetTypedResource[T]()` for type safety
7. **Set appropriate weight** - Higher weight plugins load first

## Example Plugins

See the [lynx-plugins](https://github.com/go-lynx/lynx-plugins) repository for production-ready plugin examples:

- `lynx-http` - HTTP server plugin
- `lynx-grpc` - gRPC server/client plugin
- `lynx-mysql` - MySQL database plugin
- `lynx-redis` - Redis client plugin
- `lynx-polaris` - Polaris service mesh plugin

## License

Apache License 2.0

