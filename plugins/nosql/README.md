# NoSQL Plugins for Lynx Framework

This directory contains NoSQL database plugins for the Lynx framework, currently supporting the following databases:

## Supported Plugins

### 1. Redis Plugin
- **Location**: `plugins/nosql/redis/`
- **Functionality**: Redis caching and message queue support
- **Features**: Connection pooling, cluster support, distributed locks, health checks

### 2. Elasticsearch Plugin (New)
- **Location**: `plugins/nosql/elasticsearch/`
- **Functionality**: Full-text search and data analytics
- **Features**: 
  - Complete Elasticsearch client support
  - Multiple authentication methods (username/password, API Key, service tokens)
  - TLS/SSL secure connections
  - Connection pool management
  - Automatic retry mechanisms
  - Health checks and metrics monitoring

### 3. MongoDB Plugin (New)
- **Location**: `plugins/nosql/mongodb/`
- **Functionality**: Document database support
- **Features**:
  - Complete MongoDB client support
  - Connection pool management
  - Multiple authentication methods
  - TLS/SSL secure connections
  - Read/Write concern configuration
  - Automatic retry mechanisms
  - Health checks and metrics monitoring

## Quick Start

### 1. Configuration Example

Add configuration in `config.yml`:

```yaml
lynx:
  # Redis configuration
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    pool_size: 10
    min_idle_conns: 5
    max_retries: 3
    enable_metrics: true
    enable_health_check: true

  # Elasticsearch configuration
  elasticsearch:
    addresses:
      - "http://localhost:9200"
    username: "elastic"
    password: "changeme"
    max_retries: 3
    connect_timeout: "30s"
    enable_metrics: true
    enable_health_check: true
    health_check_interval: "30s"
    compress_request_body: true
    index_prefix: "myapp"

  # MongoDB configuration
  mongodb:
    uri: "mongodb://localhost:27017"
    database: "myapp"
    username: "admin"
    password: "password"
    auth_source: "admin"
    max_pool_size: 100
    min_pool_size: 5
    connect_timeout: "30s"
    server_selection_timeout: "30s"
    socket_timeout: "30s"
    heartbeat_interval: "10s"
    enable_metrics: true
    enable_health_check: true
    health_check_interval: "30s"
    enable_tls: false
    enable_compression: true
    enable_retry_writes: true
    enable_read_concern: true
    read_concern_level: "local"
    enable_write_concern: true
    write_concern_w: 1
    write_concern_timeout: "5s"
```

### 2. Usage Example

```go
package main

import (
    "github.com/go-lynx/lynx/app/boot"
    "github.com/go-lynx/lynx/plugins/nosql/redis"
    "github.com/go-lynx/lynx/plugins/nosql/elasticsearch"
    "github.com/go-lynx/lynx/plugins/nosql/mongodb"
)

func main() {
    // Start Lynx application
    boot.LynxApplication(wireApp).Run()

    // Use Redis
    redisClient := redis.GetUniversalRedis()
    if redisClient != nil {
        // Redis operations
    }

    // Use Elasticsearch
    esClient := elasticsearch.GetElasticsearch()
    if esClient != nil {
        // Elasticsearch operations
    }

    // Use MongoDB
    mongoClient := mongodb.GetMongoDB()
    mongoDB := mongodb.GetMongoDBDatabase()
    collection := mongodb.GetMongoDBCollection("users")
    if mongoClient != nil && mongoDB != nil && collection != nil {
        // MongoDB operations
    }
}
```

## Plugin Architecture

All NoSQL plugins follow the Lynx framework's plugin architecture:

### 1. Plugin Structure
```
plugins/nosql/[plugin-name]/
├── go.mod                    # Module dependencies
├── plugin_meta.go           # Plugin metadata
├── types.go                 # Type definitions
├── options.go               # Option configurations
├── [plugin-name].go         # Main implementation
├── plug.go                  # Plugin registration
├── conf/                    # Configuration structures
│   └── [plugin-name].go
├── README.md                # Plugin documentation
└── [other files]            # Health checks, metrics, etc.
```

### 2. Core Interfaces

All plugins implement the following interfaces:

```go
type Plugin interface {
    Metadata
    Lifecycle
    LifecycleSteps
    DependencyAware
}
```

### 3. Lifecycle Management

- **Initialize**: Initialize plugin, parse configuration, create client
- **Start**: Start plugin, test connection
- **Stop**: Stop plugin, clean up resources
- **Status**: Get plugin status

## Feature Comparison

| Feature | Redis | Elasticsearch | MongoDB |
|---------|-------|---------------|---------|
| Connection Pool | ✅ | ✅ | ✅ |
| Health Checks | ✅ | ✅ | ✅ |
| Metrics Monitoring | ✅ | ✅ | ✅ |
| TLS Support | ✅ | ✅ | ✅ |
| Authentication | ✅ | ✅ | ✅ |
| Cluster Support | ✅ | ✅ | ✅ |
| Retry Mechanisms | ✅ | ✅ | ✅ |
| Hot Config Update | ✅ | ✅ | ✅ |

## Monitoring and Metrics

All plugins support:

- **Connection Status Monitoring**
- **Operation Performance Metrics**
- **Error Rate Statistics**
- **Resource Usage Information**
- **Health Check Status**

## Best Practices

### 1. Configuration Management
- Use environment variables for sensitive information
- Set reasonable connection pool sizes
- Configure appropriate timeout values

### 2. Error Handling
- Implement retry mechanisms
- Monitor connection status
- Handle network exceptions

### 3. Performance Optimization
- Use connection pooling
- Enable compression (if supported)
- Configure read/write concern levels

### 4. Security Configuration
- Use TLS encryption
- Configure appropriate authentication
- Restrict network access

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check server address and port
   - Verify network connectivity
   - Confirm firewall settings

2. **Authentication Failures**
   - Check username and password
   - Verify permission configuration
   - Confirm authentication database

3. **Performance Issues**
   - Adjust connection pool size
   - Optimize query statements
   - Check index configuration

### Debugging Methods

1. **Enable debug logging**
2. **Check health status**
3. **Monitor metrics data**
4. **View connection statistics**

## Extension Development

To develop new NoSQL plugins, refer to the structure of existing plugins:

1. Create plugin directory and file structure
2. Implement core interfaces
3. Add configuration support
4. Implement health checks and metrics
5. Write documentation and examples

## License

This project is licensed under the Apache License 2.0.
