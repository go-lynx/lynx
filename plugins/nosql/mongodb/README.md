# MongoDB Plugin for Lynx Framework

The MongoDB plugin provides complete MongoDB integration support for the Lynx framework, including document storage, queries, aggregation, and other functionalities.

## Features

- ✅ **Complete MongoDB client support**
- ✅ **Connection pool management**
- ✅ **Multiple authentication methods**
- ✅ **TLS/SSL secure connections**
- ✅ **Read/Write concern configuration**
- ✅ **Automatic retry mechanisms**
- ✅ **Health checks**
- ✅ **Metrics monitoring**
- ✅ **Hot configuration updates**

## Quick Start

### 1. Configuration

Add MongoDB configuration in `config.yml`:

```yaml
lynx:
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

### 2. Usage

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/go-lynx/lynx/app/boot"
    "github.com/go-lynx/lynx/plugins/nosql/mongodb"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
    // Start Lynx application
    boot.LynxApplication(wireApp).Run()
    
    // Get MongoDB client
    client := mongodb.GetMongoDB()
    if client == nil {
        log.Fatal("failed to get mongodb client")
    }
    
    // Get database
    db := mongodb.GetMongoDBDatabase()
    if db == nil {
        log.Fatal("failed to get mongodb database")
    }
    
    // Get collection
    collection := mongodb.GetMongoDBCollection("users")
    if collection == nil {
        log.Fatal("failed to get mongodb collection")
    }
    
    // Insert document
    insertDocument(collection)
    
    // Query documents
    findDocuments(collection)
    
    // Update document
    updateDocument(collection)
    
    // Delete document
    deleteDocument(collection)
}

func insertDocument(collection *mongo.Collection) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Document data
    doc := bson.M{
        "name":      "Zhang San",
        "email":     "zhangsan@example.com",
        "age":       25,
        "created_at": time.Now(),
    }
    
    result, err := collection.InsertOne(ctx, doc)
    if err != nil {
        log.Printf("failed to insert document: %v", err)
        return
    }
    
    log.Printf("inserted document with ID: %v", result.InsertedID)
}

func findDocuments(collection *mongo.Collection) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Query conditions
    filter := bson.M{"age": bson.M{"$gte": 20}}
    
    // Query options
    opts := options.Find().SetLimit(10)
    
    cursor, err := collection.Find(ctx, filter, opts)
    if err != nil {
        log.Printf("failed to find documents: %v", err)
        return
    }
    defer cursor.Close(ctx)
    
    var results []bson.M
    if err = cursor.All(ctx, &results); err != nil {
        log.Printf("failed to decode results: %v", err)
        return
    }
    
    log.Printf("found %d documents", len(results))
    for _, result := range results {
        log.Printf("document: %v", result)
    }
}

func updateDocument(collection *mongo.Collection) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Query conditions
    filter := bson.M{"name": "Zhang San"}
    
    // Update content
    update := bson.M{
        "$set": bson.M{
            "age":        26,
            "updated_at": time.Now(),
        },
    }
    
    result, err := collection.UpdateOne(ctx, filter, update)
    if err != nil {
        log.Printf("failed to update document: %v", err)
        return
    }
    
    log.Printf("updated %d document", result.ModifiedCount)
}

func deleteDocument(collection *mongo.Collection) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Query conditions
    filter := bson.M{"name": "Zhang San"}
    
    result, err := collection.DeleteOne(ctx, filter)
    if err != nil {
        log.Printf("failed to delete document: %v", err)
        return
    }
    
    log.Printf("deleted %d document", result.DeletedCount)
}
```

## Configuration Options

| Configuration Item | Type | Default Value | Description |
|-------------------|------|---------------|-------------|
| `uri` | `string` | `"mongodb://localhost:27017"` | MongoDB connection string |
| `database` | `string` | `"test"` | Database name |
| `username` | `string` | `""` | Username |
| `password` | `string` | `""` | Password |
| `auth_source` | `string` | `"admin"` | Authentication database |
| `max_pool_size` | `uint64` | `100` | Maximum connection pool size |
| `min_pool_size` | `uint64` | `5` | Minimum connection pool size |
| `connect_timeout` | `string` | `"30s"` | Connection timeout |
| `server_selection_timeout` | `string` | `"30s"` | Server selection timeout |
| `socket_timeout` | `string` | `"30s"` | Socket timeout |
| `heartbeat_interval` | `string` | `"10s"` | Heartbeat interval |
| `enable_metrics` | `bool` | `false` | Whether to enable metrics collection |
| `enable_health_check` | `bool` | `false` | Whether to enable health checks |
| `health_check_interval` | `string` | `"30s"` | Health check interval |
| `enable_tls` | `bool` | `false` | Whether to enable TLS |
| `tls_cert_file` | `string` | `""` | TLS certificate file |
| `tls_key_file` | `string` | `""` | TLS key file |
| `tls_ca_file` | `string` | `""` | TLS CA file |
| `enable_compression` | `bool` | `false` | Whether to enable compression |
| `compression_level` | `int` | `6` | Compression level |
| `enable_retry_writes` | `bool` | `false` | Whether to enable retry writes |
| `enable_read_concern` | `bool` | `false` | Whether to enable read concern |
| `read_concern_level` | `string` | `"local"` | Read concern level |
| `enable_write_concern` | `bool` | `false` | Whether to enable write concern |
| `write_concern_w` | `int` | `1` | Write concern acknowledgment count |
| `write_concern_timeout` | `string` | `"5s"` | Write concern timeout |

## API Reference

### Get Client

```go
// Get MongoDB client
client := mongodb.GetMongoDB()

// Get database instance
db := mongodb.GetMongoDBDatabase()

// Get collection instance
collection := mongodb.GetMongoDBCollection("users")

// Get plugin instance
plugin := mongodb.GetMongoDBPlugin()

// Get connection statistics
stats := plugin.GetConnectionStats()
```

### Plugin Options

```go
// Configure plugin using option pattern
plugin := mongodb.NewMongoDBClient(
    mongodb.WithURI("mongodb://localhost:27017"),
    mongodb.WithDatabase("myapp"),
    mongodb.WithCredentials("admin", "password", "admin"),
    mongodb.WithPoolSize(100, 5),
    mongodb.WithTimeouts(30*time.Second, 30*time.Second, 30*time.Second),
    mongodb.WithMetrics(true),
    mongodb.WithHealthCheck(true, 30*time.Second),
    mongodb.WithTLS(true, "cert.pem", "key.pem", "ca.pem"),
    mongodb.WithCompression(true, 6),
    mongodb.WithRetryWrites(true),
    mongodb.WithReadConcern(true, "local"),
    mongodb.WithWriteConcern(true, 1, 5*time.Second),
)
```

## Monitoring and Metrics

The plugin provides the following monitoring capabilities:

- **Connection pool status monitoring**
- **Database operation statistics**
- **Query performance metrics**
- **Error rate statistics**
- **Connection health status**

## Health Checks

The plugin supports automatic health checks and can monitor:

- Database connection status
- Server availability
- Authentication status
- Query response time

## Error Handling

The plugin provides comprehensive error handling mechanisms:

- Automatic retry on connection failures
- Network timeout handling
- Authentication failure handling
- Cluster failover
- Read/Write concern error handling

## Best Practices

1. **Connection Pool Configuration**: Adjust connection pool size based on load
2. **Timeout Settings**: Reasonably set connection and operation timeout times
3. **Read/Write Concerns**: Configure appropriate read/write concern levels based on business requirements
4. **Index Optimization**: Create indexes for commonly queried fields
5. **Monitoring Alerts**: Enable metrics collection and health checks
6. **Security Configuration**: Use TLS and appropriate authentication methods

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check server address and port
   - Verify network connectivity
   - Confirm firewall settings

2. **Authentication Failures**
   - Check username and password
   - Verify authentication database configuration
   - Confirm user permissions

3. **Performance Issues**
   - Adjust connection pool size
   - Optimize query statements
   - Check index configuration

### Log Debugging

Enable debug logging:

```yaml
lynx:
  mongodb:
    log_level: "debug"
```

## License

This project is licensed under the Apache License 2.0.
