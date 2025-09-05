# Polaris Plugin for Lynx Framework

This plugin provides Polaris service governance integration for the Lynx framework, offering features such as service discovery, configuration management, rate limiting, and circuit breaking.

## Features

- **Service Discovery**: Automatic service registration and discovery
- **Configuration Management**: Dynamic configuration updates
- **Rate Limiting**: HTTP and gRPC rate limiting with Polaris
- **Circuit Breaking**: Fault tolerance with circuit breaker pattern
- **Health Checking**: Service health monitoring
- **Metrics**: Prometheus metrics integration
- **Retry Management**: Configurable retry policies
- **Service Watching**: Real-time service change monitoring
- **Config Watching**: Real-time configuration change monitoring

## Installation

```bash
go get github.com/go-lynx/lynx/plugins/polaris
```

## Configuration

The plugin can be configured through the Lynx configuration system. Here's an example configuration:

```yaml
lynx:
  polaris:
    namespace: "default"
    token: "your-polaris-token"
    weight: 100
    ttl: 30
    timeout: "10s"
    # Service configuration for remote configuration loading
    service_config:
      # Configuration group name in Polaris (optional, defaults to application name)
      group: DEFAULT_GROUP
      # Configuration file name in Polaris (optional, defaults to application name with .yaml extension)
      filename: application.yaml
      # Namespace for the configuration (optional, uses main polaris namespace if not specified)
      namespace: default
      # Additional configuration files to load
      additional_configs:
        - group: SHARED_GROUP
          filename: shared-config.yaml
          namespace: default
        - group: FEATURE_GROUP
          filename: feature-flags.yaml
          namespace: default
```

### Configuration Options

- `namespace`: Polaris namespace (default: "default")
- `token`: Polaris authentication token
- `weight`: Service weight for load balancing (default: 100)
- `ttl`: Service TTL in seconds (default: 30)
- `timeout`: Request timeout duration (default: 10s)
- `service_config`: Configuration for remote service configuration loading
  - `group`: Configuration group name (optional, defaults to application name)
  - `filename`: Configuration file name (optional, defaults to application name with .yaml extension)
  - `namespace`: Namespace for the configuration (optional, uses main polaris namespace)
  - `additional_configs`: List of additional configuration files to load
    - `group`: Configuration group name
    - `filename`: Configuration file name
    - `namespace`: Namespace for this specific configuration (optional, uses service_config namespace)
    - `priority`: Merge priority (higher number = higher priority, default: 0)
    - `merge_strategy`: How to handle conflicts ("override", "merge", "append", default: "override")

## Usage

### Basic Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/plugins/polaris"
)

func main() {
    // Initialize your Lynx application
    application := app.NewApplication()
    
    // The Polaris plugin will be automatically registered and initialized
    
    // Get the Polaris plugin instance
    plugin := polaris.GetPlugin()
    
    // Get the Polaris client
    client := polaris.GetPolaris()
    
    // Start the application
    if err := application.Run(); err != nil {
        panic(err)
    }
}
```

### Rate Limiting

The plugin automatically integrates with Lynx's HTTP and gRPC servers to provide rate limiting:

```go
// HTTP rate limiting is automatically applied when the plugin is initialized
// The rate limit policies are fetched from Polaris

// gRPC rate limiting is also automatically applied
```

### Service Discovery

```go
// Get service instances
instances, err := plugin.GetServiceInstances("service-name")
if err != nil {
    log.Errorf("Failed to get service instances: %v", err)
}

// Watch service changes
watcher, err := plugin.WatchService("service-name")
if err != nil {
    log.Errorf("Failed to watch service: %v", err)
}

// Set up callbacks for service changes
watcher.SetOnInstancesChanged(func(instances []model.Instance) {
    log.Infof("Service instances changed: %v", instances)
})

watcher.SetOnError(func(err error) {
    log.Errorf("Service watch error: %v", err)
})

// Start watching
watcher.Start()
defer watcher.Stop()
```

### Configuration Management

The Polaris plugin supports both single and multiple configuration file loading:

#### Single Configuration Loading

```go
// Get configuration value
configValue, err := plugin.GetConfigValue("config-file", "group")
if err != nil {
    log.Errorf("Failed to get config: %v", err)
}

// Watch configuration changes
configWatcher, err := plugin.WatchConfig("config-file", "group")
if err != nil {
    log.Errorf("Failed to watch config: %v", err)
}
```

#### Multiple Configuration Loading

When `service_config` is configured, the plugin automatically loads multiple configuration files:

1. **Main Configuration**: Loaded based on `service_config` settings
   - If `filename` is not specified, defaults to `{application_name}.yaml`
   - If `group` is not specified, defaults to application name
   - If `namespace` is not specified, uses main polaris namespace

2. **Additional Configurations**: Loaded from `additional_configs` list
   - Each entry can specify its own group, filename, and namespace
   - Namespace defaults to `service_config.namespace` if not specified

The plugin implements the `MultiConfigControlPlane` interface to support this functionality, allowing the Lynx framework to load and merge multiple configuration sources automatically.

#### Configuration Merge Strategy

When multiple configuration files are loaded, the plugin handles conflicts using the following strategies:

1. **Priority-based Loading**: Configuration files are loaded in priority order (ascending)
   - Lower priority configs are loaded first
   - Higher priority configs override lower priority ones
   - Default priority is 0

2. **Merge Strategies**:
   - **`override`** (default): Later configs completely override earlier ones for the same field
   - **`merge`**: Merge nested objects, override leaf values
   - **`append`**: Append to arrays, override other values

3. **Loading Order**:
   ```
   Main Config (priority: 0) → Additional Configs (by priority) → Final Config
   ```

4. **Conflict Resolution Example**:
   ```yaml
   # Main config (application.yaml)
   lynx:
     http:
       addr: "0.0.0.0:8080"
       timeout: "5s"
   
   # Additional config 1 (shared-config.yaml, priority: 10)
   lynx:
     http:
       addr: "0.0.0.0:9090"  # Overrides main config
     db:
       driver: "mysql"
   
   # Additional config 2 (env-specific.yaml, priority: 20)
   lynx:
     http:
       timeout: "10s"        # Overrides both previous configs
     feature:
       new_api: true
   ```

5. **Logging**: The plugin logs detailed information about configuration loading, including:
   - File names and groups
   - Namespaces
   - Priorities
   - Merge strategies
   - Any conflicts or overrides

// Set up callbacks for config changes
configWatcher.SetOnConfigChanged(func(config polaris.ConfigFile) {
    log.Infof("Config changed: %v", config)
})

configWatcher.SetOnError(func(err error) {
    log.Errorf("Config watch error: %v", err)
})

// Start watching
configWatcher.Start()
defer configWatcher.Stop()
```

### Circuit Breaker

```go
// Create a circuit breaker
circuitBreaker := polaris.NewCircuitBreaker(0.5)

// Use the circuit breaker to protect operations
err := circuitBreaker.Do(func() error {
    // Your operation here
    return nil
})

if err != nil {
    log.Errorf("Circuit breaker error: %v", err)
}
```

### Retry Management

```go
// Create a retry manager
retryManager := polaris.NewRetryManager(3, 100*time.Millisecond)

// Use retry with exponential backoff
err := retryManager.DoWithRetry(func() error {
    // Your operation here
    return nil
})

if err != nil {
    log.Errorf("Retry failed: %v", err)
}
```

### Metrics

The plugin provides comprehensive Prometheus metrics:

```go
// Get metrics
metrics := plugin.GetMetrics()

// Metrics include:
// - SDK operations (success/failure counts)
// - Service discovery operations
// - Configuration operations
// - Rate limiting operations
// - Health check results
// - Connection status
```

## Architecture

The plugin is organized into several domains:

- **Core (`polaris.go`)**: Main plugin implementation and Lynx framework integration
- **Rate Limiting (`limit.go`)**: HTTP and gRPC rate limiting middleware
- **Metrics (`metrics.go`)**: Prometheus metrics collection
- **Resilience (`resilience.go`)**: Circuit breaker and retry mechanisms
- **Watchers (`watchers.go`)**: Service and configuration change monitoring
- **Configuration (`conf/`)**: Configuration management and validation
- **Errors (`errors/`)**: Structured error handling
- **Validation (`validator.go`)**: Configuration validation

## Events

The plugin emits the following events:

- `EventPluginStarted`: When the Polaris plugin is successfully initialized
- `EventPluginStopping`: When the plugin is about to stop
- `EventPluginStopped`: When the plugin has been stopped

## Health Checks

The plugin provides health check information including:
- Plugin initialization status
- Polaris connection status
- Service registration status
- Configuration synchronization status

## Dependencies

- github.com/go-kratos/kratos/contrib/polaris/v2 v2.0.0
- github.com/go-lynx/lynx v1.2.1
- github.com/prometheus/client_golang v1.17.0

## License

Apache License 2.0 