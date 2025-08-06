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
```

### Configuration Options

- `namespace`: Polaris namespace (default: "default")
- `token`: Polaris authentication token
- `weight`: Service weight for load balancing (default: 100)
- `ttl`: Service TTL in seconds (default: 30)
- `timeout`: Request timeout duration (default: 10s)

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