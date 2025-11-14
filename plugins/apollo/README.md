# Apollo Plugin for Lynx Framework

This plugin provides Apollo configuration center integration for the Lynx framework, offering features such as configuration management, dynamic configuration updates, and configuration change notifications.

## Features

- **Configuration Management**: Dynamic configuration loading from Apollo
- **Configuration Watching**: Real-time configuration change monitoring
- **Multi-Namespace Support**: Load configurations from multiple namespaces
- **Local Cache**: Optional local caching to reduce network requests
- **Health Checking**: Service health monitoring
- **Metrics**: Prometheus metrics integration
- **Retry Management**: Configurable retry policies
- **Circuit Breaker**: Fault tolerance with circuit breaker pattern

## Installation

```bash
go get github.com/go-lynx/lynx/plugins/apollo
```

## Configuration

The plugin can be configured through the Lynx configuration system. Here's an example configuration:

```yaml
lynx:
  apollo:
    # Basic Configuration
    app_id: "your-app-id"                      # Application ID (required)
    cluster: "default"                         # Cluster name (default: "default")
    namespace: "application"                   # Namespace (default: "application")
    meta_server: "http://localhost:8080"       # Apollo Meta Server address (required)
    token: "your-apollo-token"                 # Authentication token (optional)
    timeout: "10s"                             # Operation timeout
    
    # Notification Configuration
    enable_notification: true                  # Enable configuration change notification
    notification_timeout: "30s"                 # Notification timeout
    
    # Cache Configuration
    enable_cache: true                          # Enable local cache
    cache_dir: "/tmp/apollo-cache"             # Cache directory
    
    # Advanced Feature Configuration
    enable_metrics: true                        # Enable monitoring metrics
    enable_retry: true                          # Enable retry mechanism
    max_retry_times: 3                          # Maximum retry times
    retry_interval: "1s"                       # Retry interval
    enable_circuit_breaker: true               # Enable circuit breaker
    circuit_breaker_threshold: 0.5              # Circuit breaker threshold
    enable_graceful_shutdown: true             # Enable graceful shutdown
    shutdown_timeout: "30s"                    # Graceful shutdown timeout
    enable_logging: true                        # Enable detailed logging
    log_level: "info"                          # Log level (debug, info, warn, error)
    
    # Service Configuration for remote configuration loading
    service_config:
      namespace: "application"                   # Main namespace
      additional_namespaces:                    # Additional namespaces to load
        - "shared-config"
        - "feature-flags"
      priority: 0                               # Merge priority
      merge_strategy: "override"                # Merge strategy (override, merge, append)
```

### Configuration Options

- `app_id`: Apollo application ID (required)
- `cluster`: Cluster name (default: "default")
- `namespace`: Namespace name (default: "application")
- `meta_server`: Apollo Meta Server address (required)
- `token`: Authentication token (optional)
- `timeout`: Operation timeout (default: 10s)
- `enable_notification`: Enable configuration change notification (default: true)
- `notification_timeout`: Notification timeout (default: 30s)
- `enable_cache`: Enable local cache (default: true)
- `cache_dir`: Cache directory (default: "/tmp/apollo-cache")
- `enable_metrics`: Enable monitoring metrics (default: true)
- `enable_retry`: Enable retry mechanism (default: true)
- `max_retry_times`: Maximum retry times (default: 3)
- `retry_interval`: Retry interval (default: 1s)
- `enable_circuit_breaker`: Enable circuit breaker (default: true)
- `circuit_breaker_threshold`: Circuit breaker threshold (default: 0.5)
- `enable_graceful_shutdown`: Enable graceful shutdown (default: true)
- `shutdown_timeout`: Graceful shutdown timeout (default: 30s)
- `enable_logging`: Enable detailed logging (default: true)
- `log_level`: Log level (default: "info")

## Usage

### Basic Usage

The plugin automatically registers itself when imported. You can access it through the plugin manager:

```go
import (
    "github.com/go-lynx/lynx/plugins/apollo"
    "github.com/go-lynx/lynx/app"
)

// Get Apollo plugin
plugin := app.Lynx().GetPluginManager().GetPlugin("apollo.config.center")
if plugin != nil {
    apolloPlugin := plugin.(*apollo.PlugApollo)
    // Use the plugin
}
```

### Configuration Management

The Apollo plugin supports both single and multiple namespace loading:

#### Single Namespace Loading

```go
// Get configuration value
value, err := plugin.GetConfigValue("application", "config.key")
if err != nil {
    log.Errorf("Failed to get config: %v", err)
}
```

#### Multiple Namespace Loading

When `service_config` is configured, the plugin automatically loads multiple namespaces:

1. **Main Configuration**: Loaded based on `service_config.namespace`
   - If `namespace` is not specified, uses main apollo namespace

2. **Additional Configurations**: Loaded from `additional_namespaces` list
   - Each entry specifies a separate namespace to load
   - Namespace defaults to `service_config.namespace` if not specified

The plugin implements the `MultiConfigControlPlane` interface to support this functionality, allowing the Lynx framework to load and merge multiple configuration sources automatically.

### Configuration Watching

```go
// Watch configuration changes
watcher, err := plugin.WatchConfig("application")
if err != nil {
    log.Errorf("Failed to watch config: %v", err)
}

// Set up callbacks for configuration changes
watcher.SetOnConfigChanged(func(namespace, key, value string) {
    log.Infof("Config changed - Namespace: %s, Key: %s, Value: %s", namespace, key, value)
})

watcher.SetOnError(func(err error) {
    log.Errorf("Config watch error: %v", err)
})

// Start watching
watcher.Start()
defer watcher.Stop()
```

## Implementation Notes

This plugin provides a complete framework for Apollo integration. The actual Apollo client implementation depends on the Apollo Go SDK being used. The following methods need to be implemented based on the specific Apollo SDK:

1. `initApolloClient()` - Initialize Apollo client
2. `getConfigValueFromApollo()` - Get configuration value from Apollo
3. `ApolloConfigSource.Load()` - Load configurations from Apollo
4. `ApolloConfigSource.Watch()` - Watch configuration changes
5. `ConfigWatcher` notification listener - Listen to Apollo notifications

## Metrics

The plugin exposes the following Prometheus metrics:

- `lynx_apollo_client_operations_total` - Total number of client operations
- `lynx_apollo_client_operations_duration_seconds` - Duration of client operations
- `lynx_apollo_client_errors_total` - Total number of client errors
- `lynx_apollo_config_operations_total` - Total number of configuration operations
- `lynx_apollo_config_operations_duration_seconds` - Duration of configuration operations
- `lynx_apollo_config_changes_total` - Total number of configuration changes
- `lynx_apollo_notification_total` - Total number of notifications
- `lynx_apollo_notification_duration_seconds` - Duration of notification operations
- `lynx_apollo_health_check_total` - Total number of health checks
- `lynx_apollo_health_check_duration_seconds` - Duration of health checks
- `lynx_apollo_cache_hits_total` - Total number of cache hits
- `lynx_apollo_cache_misses_total` - Total number of cache misses

## License

This plugin is part of the Lynx framework and follows the same license.

