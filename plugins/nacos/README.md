# Nacos Plugin for Lynx Framework

This plugin provides Nacos service registration, discovery, and configuration management functionality for the Lynx framework.

## Features

- ✅ **Service Registration**: Register service instances to Nacos
- ✅ **Service Discovery**: Discover service instances from Nacos
- ✅ **Configuration Management**: Get and watch configuration from Nacos
- ✅ **Multi-Config Support**: Load multiple configuration sources
- ✅ **Dynamic Configuration**: Real-time configuration updates
- ✅ **Health Check**: Built-in health check support
- ✅ **Authentication**: Support username/password and access key/secret key
- ✅ **Namespace Support**: Multi-tenant namespace isolation

## Installation

```bash
go get github.com/go-lynx/lynx/plugins/nacos
```

## Quick Start

### 1. Configuration

```yaml
lynx:
  nacos:
    # Nacos server addresses
    server_addresses: "127.0.0.1:8848"
    
    # Namespace
    namespace: "public"
    
    # Authentication (optional)
    username: "nacos"
    password: "nacos"
    
    # Feature flags
    enable_register: true
    enable_discovery: true
    enable_config: true
    
    # Service configuration
    service_config:
      service_name: "my-service"
      group: "DEFAULT_GROUP"
      cluster: "DEFAULT"
```

### 2. Usage

```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/boot"
    _ "github.com/go-lynx/lynx/plugins/nacos" // Import to register plugin
)

func main() {
    boot.LynxApplication(wireApp).Run()
}

func wireApp() (*kratos.App, error) {
    // Nacos plugin will be automatically loaded from configuration
    // ...
}
```

## Configuration Reference

### Basic Configuration

```yaml
lynx:
  nacos:
    # Required: Nacos server addresses (comma-separated for cluster)
    server_addresses: "127.0.0.1:8848,127.0.0.1:8849"
    
    # Optional: Use endpoint instead of server_addresses
    # endpoint: "http://nacos.example.com"
    
    # Namespace configuration
    namespace_id: ""  # Use namespace_id or namespace
    namespace: "public"  # Default: public
    
    # Authentication
    username: "nacos"
    password: "nacos"
    # Or use access_key and secret_key
    # access_key: ""
    # secret_key: ""
    
    # Service instance configuration
    weight: 1.0
    metadata:
      version: "v1.0.0"
      env: "production"
    
    # Feature flags
    enable_register: true
    enable_discovery: true
    enable_config: true
    
    # Connection settings
    timeout: 5  # seconds
    notify_timeout: 3000  # milliseconds
    
    # Logging
    log_level: "info"  # debug, info, warn, error
    log_dir: "./logs/nacos"
    cache_dir: "./cache/nacos"
    
    # Service registration
    service_config:
      service_name: "my-service"
      group: "DEFAULT_GROUP"
      cluster: "DEFAULT"
      health_check: true
      health_check_interval: 5
      health_check_timeout: 3
      health_check_type: "tcp"  # none, tcp, http, mysql
      health_check_url: ""
    
    # Additional configuration sources
    additional_configs:
      - data_id: "database-config"
        group: "DEFAULT_GROUP"
        format: "yaml"
      - data_id: "redis-config"
        group: "DEFAULT_GROUP"
        format: "yaml"
```

## API Reference

### Service Registration and Discovery

```go
// Get Nacos plugin
nacosPlugin := app.Lynx().GetPluginManager().GetPlugin("nacos.control.plane")
if nacosPlugin == nil {
    return fmt.Errorf("nacos plugin not found")
}

nacos, ok := nacosPlugin.(*nacos.PlugNacos)
if !ok {
    return fmt.Errorf("invalid plugin type")
}

// Get service registry
registrar := nacos.NewServiceRegistry()

// Register service
instance := &registry.ServiceInstance{
    ID:       "instance-1",
    Name:     "my-service",
    Version:  "v1.0.0",
    Metadata: map[string]string{"env": "production"},
    Endpoints: []string{"http://127.0.0.1:8080"},
}
err := registrar.Register(context.Background(), instance)

// Get service discovery
discovery := nacos.NewServiceDiscovery()

// Get service instances
instances, err := discovery.GetService(context.Background(), "my-service")

// Watch service changes
watcher, err := discovery.Watch(context.Background(), "my-service")
```

### Configuration Management

```go
// Get configuration
configSource, err := nacos.GetConfig("application.yaml", "DEFAULT_GROUP")
if err != nil {
    return err
}

// Load configuration
kvs, err := configSource.Load()

// Watch configuration changes
watcher, err := configSource.Watch()
if err != nil {
    return err
}

// Get next change
kvs, err := watcher.Next()
```

### Multi-Config Loading

```go
// Get all configuration sources
sources, err := nacos.GetConfigSources()
if err != nil {
    return err
}

// Load all configurations
for _, source := range sources {
    kvs, err := source.Load()
    // Process configuration
}
```

## Health Check

Nacos supports multiple health check types:

- **none**: No health check
- **tcp**: TCP connection check
- **http**: HTTP endpoint check
- **mysql**: MySQL connection check

```yaml
service_config:
  health_check: true
  health_check_type: "http"
  health_check_url: "http://127.0.0.1:8080/health"
  health_check_interval: 5
  health_check_timeout: 3
```

## Authentication

Nacos supports two authentication methods:

### Username/Password

```yaml
nacos:
  username: "nacos"
  password: "nacos"
```

### Access Key/Secret Key

```yaml
nacos:
  access_key: "your-access-key"
  secret_key: "your-secret-key"
```

## Namespace Support

Nacos supports namespace isolation:

```yaml
nacos:
  namespace_id: "your-namespace-id"
  # Or use namespace name
  namespace: "production"
```

## Cluster Support

Nacos supports service clustering:

```yaml
nacos:
  service_config:
    cluster: "cluster-a"  # Default: DEFAULT
```

## Best Practices

1. **Use Namespace for Environment Isolation**
   ```yaml
   namespace: "production"  # or "development", "staging"
   ```

2. **Enable Health Check**
   ```yaml
   service_config:
     health_check: true
     health_check_type: "http"
     health_check_url: "/health"
   ```

3. **Use Multiple Server Addresses for High Availability**
   ```yaml
   server_addresses: "nacos1:8848,nacos2:8848,nacos3:8848"
   ```

4. **Configure Appropriate Timeouts**
   ```yaml
   timeout: 5  # Connection timeout
   notify_timeout: 3000  # Notification timeout
   ```

5. **Use Metadata for Service Information**
   ```yaml
   metadata:
     version: "v1.0.0"
     region: "us-east-1"
     zone: "zone-a"
   ```

## Troubleshooting

### Connection Issues

- Check Nacos server is running and accessible
- Verify server addresses are correct
- Check network connectivity
- Verify authentication credentials

### Configuration Not Loading

- Verify `enable_config` is set to `true`
- Check dataId and group are correct
- Verify namespace configuration
- Check Nacos console for configuration existence

### Service Registration Failed

- Verify `enable_register` is set to `true`
- Check service name is valid
- Verify endpoint format is correct
- Check Nacos server logs

## Examples

See `examples/` directory for complete examples.

## License

Apache License 2.0

