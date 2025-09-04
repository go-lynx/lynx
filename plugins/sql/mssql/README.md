# Microsoft SQL Server Plugin

The Microsoft SQL Server Plugin is a comprehensive database integration plugin for the Lynx framework. It provides full support for Microsoft SQL Server databases, including both on-premise and Azure SQL Database deployments.

## Features

### Core Database Support
- **Full SQL Server Compatibility**: Support for SQL Server 2012 and later versions
- **Azure SQL Database**: Native support for Azure SQL Database
- **Connection Pooling**: Efficient connection management with configurable pool sizes
- **Transaction Support**: Full ACID transaction support with configurable isolation levels
- **Stored Procedures**: Execute stored procedures with parameter support

### Security Features
- **Windows Authentication**: Support for Windows Authentication (on-premise)
- **SQL Authentication**: Username/password authentication
- **Encryption**: TLS/SSL encryption support
- **Certificate Trust**: Configurable server certificate validation

### Performance & Monitoring
- **Prometheus Metrics**: Comprehensive monitoring and alerting
- **Connection Statistics**: Real-time connection pool monitoring
- **Health Checks**: Automated health monitoring and reporting
- **Performance Metrics**: Query and transaction performance tracking

### Configuration Management
- **Flexible Configuration**: YAML and JSON configuration support
- **Environment-Specific**: Different configurations for dev, staging, and production
- **Hot Reloading**: Runtime configuration updates without restart
- **Validation**: Automatic configuration validation and error reporting

## Architecture

The plugin follows the Lynx framework's layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                    MSSQL Plugin Layer                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Client    │  │   Metrics   │  │   Configuration    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Base SQL Layer                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Connection  │  │   Pool      │  │   Health Check      │ │
│  │   Pool      │  │ Management  │  │     System          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Driver Layer                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   MSSQL     │  │   Ent       │  │   Database/SQL     │ │
│  │   Driver    │  │   Driver    │  │     Interface      │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
lynx:
  mssql:
    driver: "mssql"
    min_conn: 5
    max_conn: 20
    max_idle_time: 300s
    max_life_time: 3600s
    
    server_config:
      instance_name: "localhost"
      port: 1433
      database: "your_database"
      username: "your_username"
      password: "your_password"
      encrypt: false
      connection_pooling: true
```

### Advanced Configuration

```yaml
lynx:
  mssql:
    server_config:
      # High availability
      instance_name: "your-availability-group-listener"
      
      # Security
      encrypt: true
      trust_server_certificate: false
      
      # Performance
      connection_timeout: 30
      command_timeout: 30
      
      # Connection pooling
      max_pool_size: 50
      min_pool_size: 10
      pool_blocking_timeout: 30
      pool_lifetime_timeout: 3600
      
      # Application identification
      application_name: "Lynx-MSSQL-Plugin"
      workstation_id: "lynx-server"
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "database/sql"
    "github.com/go-lynx/lynx/plugins/sql/mssql"
)

func main() {
    // Get the MSSQL client instance
    mssqlClient := mssql.GetMssqlClient()
    
    // Get the underlying database connection
    db := mssql.GetMssqlDB()
    
    // Execute a simple query
    ctx := context.Background()
    var result string
    err := db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&result)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("SQL Server Version: %s\n", result)
}
```

### Advanced Usage

```go
// Test connection
err := mssqlClient.TestConnection(ctx)
if err != nil {
    log.Printf("Connection test failed: %v", err)
}

// Get server information
serverInfo, err := mssqlClient.GetServerInfo(ctx)
if err != nil {
    log.Printf("Failed to get server info: %v", err)
} else {
    log.Printf("Server: %s, Database: %s", 
        serverInfo["server_name"], serverInfo["database"])
}

// Execute stored procedure
rows, err := mssqlClient.ExecuteStoredProcedure(ctx, "GetUserById", 123)
if err != nil {
    log.Printf("Failed to execute stored procedure: %v", err)
}
defer rows.Close()

// Begin transaction
tx, err := mssqlClient.BeginTransaction(ctx)
if err != nil {
    log.Printf("Failed to begin transaction: %v", err)
}
defer tx.Rollback()

// Get connection statistics
stats := mssqlClient.GetConnectionStats()
log.Printf("Active connections: %d, Idle: %d", 
    stats["in_use"], stats["idle"])
```

### Health Monitoring

```go
// Check plugin health
err := mssqlClient.CheckHealth()
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Check connection status
if mssqlClient.IsConnected() {
    log.Println("MSSQL plugin is connected")
} else {
    log.Println("MSSQL plugin is not connected")
}
```

## API Reference

### DBMssqlClient

The main client interface providing access to all MSSQL functionality.

#### Methods

- `GetMssqlConfig() *conf.Mssql` - Returns the current configuration
- `TestConnection(ctx context.Context) error` - Tests database connectivity
- `GetServerInfo(ctx context.Context) (map[string]interface{}, error)` - Gets server information
- `ExecuteStoredProcedure(ctx context.Context, procName string, args ...interface{}) (*sql.Rows, error)` - Executes stored procedures
- `BeginTransaction(ctx context.Context) (*sql.Tx, error)` - Begins a new transaction
- `IsConnected() bool` - Checks connection status
- `GetConnectionStats() map[string]interface{}` - Gets connection statistics

### Configuration

See `conf/mssql.go` for detailed configuration structure definitions.

## Connection String Building

The plugin automatically builds connection strings from configuration:

### Basic Connection String
```
server=localhost;port=1433;database=your_database;user id=your_username;password=your_password
```

### Windows Authentication
```
server=localhost;port=1433;database=your_database;trusted_connection=true
```

### Azure SQL Database
```
server=your-server.database.windows.net;port=1433;database=your_database;user id=your_username@your-server;password=your_password;encrypt=true
```

### Named Instance
```
server=localhost\\SQLEXPRESS;port=1433;database=your_database;user id=your_username;password=your_password
```

## Monitoring and Metrics

### Prometheus Metrics

The plugin exposes comprehensive Prometheus metrics:

#### Connection Pool Metrics
- `lynx_mssql_connection_pool_max_open` - Maximum open connections
- `lynx_mssql_connection_pool_open` - Current open connections
- `lynx_mssql_connection_pool_in_use` - Connections in use
- `lynx_mssql_connection_pool_idle` - Idle connections
- `lynx_mssql_connection_pool_wait_count_total` - Total wait count
- `lynx_mssql_connection_pool_wait_duration_seconds` - Wait duration

#### Health Check Metrics
- `lynx_mssql_health_check_total` - Total health checks
- `lynx_mssql_health_check_success_total` - Successful health checks
- `lynx_mssql_health_check_failure_total` - Failed health checks

#### Configuration Metrics
- `lynx_mssql_config_max_connections` - Configured max connections
- `lynx_mssql_config_min_connections` - Configured min connections
- `lynx_mssql_config_encryption_enabled` - Encryption status
- `lynx_mssql_config_connection_pooling` - Connection pooling status

### Grafana Dashboard

The plugin includes a Grafana dashboard for monitoring:

```json
{
  "dashboard": {
    "title": "Lynx MSSQL Plugin",
    "panels": [
      {
        "title": "Connection Pool Status",
        "type": "stat",
        "targets": [
          {
            "expr": "lynx_mssql_connection_pool_open",
            "legendFormat": "Open Connections"
          }
        ]
      }
    ]
  }
}
```

## Deployment

### Local Development

```bash
cd plugins/sql/mssql
go mod tidy
go build
```

### Docker Deployment

```dockerfile
FROM mcr.microsoft.com/mssql-tools:latest
COPY . /app
WORKDIR /app
RUN go build -o mssql-plugin .
CMD ["./mssql-plugin"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lynx-mssql-plugin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: lynx-mssql-plugin
  template:
    metadata:
      labels:
        app: lynx-mssql-plugin
    spec:
      containers:
      - name: mssql-plugin
        image: lynx-mssql-plugin:latest
        ports:
        - containerPort: 8080
        env:
        - name: MSSQL_SERVER
          value: "your-sql-server"
        - name: MSSQL_DATABASE
          value: "your_database"
```

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Check server address and port
   - Verify firewall settings
   - Check SQL Server service status

2. **Authentication Errors**
   - Verify username/password
   - Check Windows Authentication settings
   - Verify database permissions

3. **Connection Pool Issues**
   - Check max_conn and min_conn settings
   - Monitor connection pool metrics
   - Check for connection leaks

4. **Performance Issues**
   - Monitor query performance metrics
   - Check connection pool utilization
   - Review timeout settings

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
lynx:
  mssql:
    server_config:
      application_name: "Lynx-MSSQL-Plugin-Debug"
      connection_timeout: 60
      command_timeout: 60
```

## Best Practices

### Connection Management
- Use appropriate pool sizes for your workload
- Monitor connection pool metrics regularly
- Implement proper error handling and retry logic

### Security
- Use Windows Authentication when possible
- Enable encryption for production environments
- Regularly rotate database passwords

### Performance
- Use connection pooling effectively
- Monitor query performance
- Implement proper indexing strategies

### Monitoring
- Set up Prometheus alerts for critical metrics
- Monitor connection pool utilization
- Track query performance trends

## Contributing

Contributions are welcome! Please see the main Lynx framework contribution guidelines.

## License

This plugin is part of the Lynx framework and follows the same license terms.

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Documentation](https://lynx.go-lynx.com)
- Community: [Lynx Community](https://community.go-lynx.com)
