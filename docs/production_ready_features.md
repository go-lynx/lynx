# Lynx Production-Ready Features Guide

This document provides comprehensive information about Lynx framework's production-ready features, including the plugin ecosystem, observability, error recovery, and deployment best practices.

## 🔌 Plugin Ecosystem

Lynx provides a rich plugin ecosystem covering all aspects of microservice development:

### Service Plugins

#### HTTP Plugin (`lynx-http`)

Full-featured HTTP server with production-ready capabilities:

```yaml
lynx:
  http:
    network: tcp
    addr: :8080
    timeout: 30s
    tls_enable: false
    
    middleware:
      enable_tracing: true
      enable_logging: true
      enable_recovery: true
      enable_validation: true
      enable_rate_limit: true
      enable_metrics: true
    
    security:
      max_request_size: 10485760  # 10MB
      cors:
        enabled: true
        allowed_origins: ["*"]
      rate_limit:
        enabled: true
        rate_per_second: 100
        burst_limit: 200
    
    performance:
      max_connections: 1000
      read_timeout: 30s
      write_timeout: 30s
      idle_timeout: 60s
    
    graceful_shutdown:
      shutdown_timeout: 30s
      wait_for_ongoing_requests: true
```

**Features:**
- TLS/HTTPS support with client authentication
- Built-in middleware (tracing, logging, rate limiting, recovery)
- Custom response encoding and error handling
- Connection pooling and buffer optimization
- Prometheus metrics integration
- CORS and security headers

#### gRPC Plugin (`lynx-grpc`)

Complete gRPC server and client implementation:

```yaml
lynx:
  grpc:
    service:
      network: "tcp"
      addr: ":9090"
      timeout: 10
      tls_enable: true
      tls_auth_type: 4  # Mutual TLS
      max_concurrent_streams: 1000
    
    client:
      default_timeout: "10s"
      max_retries: 3
      retry_backoff: "1s"
      connection_pooling: true
      pool_size: 5
```

**Features:**
- Full gRPC server with TLS and middleware
- Client-side connection pooling and load balancing
- Automatic retry with exponential backoff
- Service discovery integration
- Circuit breaker pattern
- Prometheus metrics for server and client

### Database Plugins

#### MySQL Plugin (`lynx-mysql`)

```yaml
lynx:
  mysql:
    driver: "mysql"
    dsn: "user:password@tcp(localhost:3306)/database?charset=utf8mb4&parseTime=True"
    min_conn: 10
    max_conn: 100
    max_idle_time: 300s
    max_life_time: 3600s
    
    health_check_interval: 30s
    
    tls:
      enabled: true
      cert_file: "/path/to/client-cert.pem"
      key_file: "/path/to/client-key.pem"
      ca_file: "/path/to/ca-cert.pem"
    
    monitoring:
      enable_metrics: true
      slow_query_threshold: 1s
      enable_slow_query_log: true
```

**Features:**
- Connection pooling with configurable sizes
- SSL/TLS encryption support
- Prometheus metrics (connection pool, query performance)
- Health checks and slow query logging
- Transaction support with isolation levels

#### PostgreSQL Plugin (`lynx-pgsql`)

Similar configuration and features as MySQL, optimized for PostgreSQL.

#### MongoDB Plugin (`lynx-mongodb`)

```yaml
lynx:
  mongodb:
    uri: "mongodb://localhost:27017"
    database: "mydb"
    max_pool_size: 100
    min_pool_size: 10
    connect_timeout: 10s
```

### Cache Plugins

#### Redis Plugin (`lynx-redis`)

Unified support for standalone, cluster, and sentinel topologies:

```yaml
lynx:
  redis:
    addrs: ["127.0.0.1:6379"]
    db: 0
    min_idle_conns: 10
    max_active_conns: 100
    dial_timeout: { seconds: 5 }
    read_timeout: { seconds: 5 }
    write_timeout: { seconds: 5 }
    
    # For Sentinel
    sentinel:
      master_name: mymaster
    
    # For TLS
    tls:
      enabled: true
      insecure_skip_verify: false
```

**Features:**
- Auto-detection of topology (single/cluster/sentinel)
- Command-level Prometheus metrics
- Connection pool statistics
- TLS support
- Health checks with cluster status monitoring

#### Redis Lock Plugin (`lynx-redis-lock`)

Distributed locking implementation:

```go
lock := redislock.NewLock("my-resource", time.Second*30)
if err := lock.Lock(ctx); err != nil {
    return err
}
defer lock.Unlock(ctx)
// Critical section
```

### Message Queue Plugins

#### Kafka Plugin (`lynx-kafka`)

```yaml
lynx:
  kafka:
    brokers: ["localhost:9092"]
    producer:
      batch_size: 100
      linger_ms: 5
      compression: "gzip"
    consumer:
      group_id: "my-group"
      auto_offset_reset: "earliest"
    
    sasl:
      enabled: true
      mechanism: "SCRAM-SHA-256"
    
    tls:
      enabled: true
```

**Features:**
- Producer with batching and compression
- Consumer groups with offset management
- SASL authentication (PLAIN, SCRAM)
- TLS encryption
- Prometheus metrics
- Retry and error handling

#### Pulsar Plugin (`lynx-pulsar`)

Apache Pulsar producer/consumer with similar enterprise features.

#### RabbitMQ Plugin (`lynx-rabbitmq`)

```yaml
lynx:
  rabbitmq:
    url: "amqp://user:pass@localhost:5672/"
    connection_pool:
      max_size: 10
      min_size: 2
```

### Service Governance Plugins

#### Polaris Plugin (`lynx-polaris`)

Tencent Polaris integration for service mesh:

```yaml
lynx:
  polaris:
    namespace: "default"
    token: "your-polaris-token"
    weight: 100
    ttl: 30
    timeout: "10s"
    
    service_config:
      group: DEFAULT_GROUP
      filename: application.yaml
      additional_configs:
        - group: SHARED_GROUP
          filename: shared-config.yaml
          priority: 10
```

**Features:**
- Service discovery and registration
- Dynamic configuration with hot reload
- Rate limiting (HTTP and gRPC)
- Circuit breaking
- Multi-configuration loading with merge strategies

#### Nacos Plugin (`lynx-nacos`)

Alibaba Nacos for service discovery and configuration:

```yaml
lynx:
  nacos:
    server_addr: "127.0.0.1:8848"
    namespace_id: "public"
    group: "DEFAULT_GROUP"
```

#### Apollo Plugin (`lynx-apollo`)

Apollo configuration center integration with validation and health monitoring.

### Observability Plugins

#### Tracer Plugin (`lynx-tracer`)

OpenTelemetry-based distributed tracing:

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      protocol: PROTOCOL_OTLP_GRPC
      insecure: true
      
      batch:
        enabled: true
        max_queue_size: 2048
        max_batch_size: 512
      
      sampler:
        type: SAMPLER_TRACEID_RATIO
        ratio: 0.1
      
      propagators: [W3C_TRACE_CONTEXT, W3C_BAGGAGE]
      
      connection:
        max_conn_idle_time: 30s
        reconnection_period: 5s
      
      load_balancing:
        policy: "round_robin"
        health_check: true
```

**Features:**
- OTLP gRPC and HTTP exporters
- Multiple samplers (AlwaysOn, TraceIDRatio, ParentBased)
- Propagators (W3C, B3, Jaeger)
- Connection pooling and load balancing
- Graceful shutdown

### Distributed Transaction Plugins

#### DTM Plugin (`lynx-dtm`)

DTM distributed transaction support:

```yaml
lynx:
  dtm:
    server_addr: "localhost:36790"
    barrier:
      driver: "mysql"
      dsn: "user:pass@tcp(localhost:3306)/dtm_barrier"
```

**Features:**
- SAGA pattern support
- TCC pattern support
- XA transactions
- Barrier mechanism for idempotency

#### Seata Plugin (`lynx-seata`)

Alibaba Seata for distributed transactions.

## 🚀 Application Bootstrap

### Basic Application Setup

```go
package main

import (
    "github.com/go-lynx/lynx/boot"
    "github.com/go-kratos/kratos/v2"
    "github.com/go-kratos/kratos/v2/log"
)

func main() {
    app := boot.NewApplication(wireApp)
    
    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}

func wireApp(logger log.Logger) (*kratos.App, error) {
    return kratos.New(
        kratos.ID("my-service"),
        kratos.Name("My Service"),
        kratos.Version("1.0.0"),
        kratos.Logger(logger),
    ), nil
}
```

### Configuration

```yaml
# bootstrap.yaml
lynx:
  application:
    name: "production-service"
    version: "1.0.0"
  
  shutdown:
    timeout: 30s
    graceful: true
  
  error_recovery:
    max_error_history: 1000
    error_threshold: 10
    recovery_timeout: 30s
  
  metrics:
    enabled: true
    update_interval: 30s
    prometheus:
      enabled: true
      port: 9090
  
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
```

## 📊 Monitoring and Observability

### Prometheus Metrics

The framework automatically exports comprehensive metrics:

**Application Metrics**
- `lynx_app_start_time_seconds` - Application start time
- `lynx_app_uptime_seconds` - Application uptime
- `lynx_app_version_info` - Version information

**System Metrics**
- `lynx_system_memory_bytes` - Memory usage
- `lynx_system_goroutines` - Goroutine count
- `lynx_system_cpus` - CPU count

**Plugin Metrics**
- `lynx_plugin_count` - Total plugins
- `lynx_plugin_health_status` - Health status per plugin
- `lynx_plugin_errors_total` - Error count per plugin
- `lynx_plugin_latency_seconds` - Operation latency

**HTTP/gRPC Metrics**
- `lynx_requests_total` - Total requests
- `lynx_request_duration_seconds` - Request duration
- `lynx_request_size_bytes` - Request size
- `lynx_response_size_bytes` - Response size

**Database Metrics**
- `lynx_mysql_connection_pool_*` - Connection pool stats
- `lynx_mysql_query_duration_seconds` - Query duration
- `lynx_mysql_slow_queries_total` - Slow query count

**Cache Metrics**
- `lynx_redis_commands_*` - Command latency and errors
- `lynx_redis_pool_*` - Connection pool stats

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Lynx Framework Dashboard",
    "panels": [
      {
        "title": "Application Health",
        "type": "stat",
        "targets": [
          {"expr": "lynx_app_uptime_seconds", "legendFormat": "Uptime"}
        ]
      },
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {"expr": "rate(lynx_requests_total[5m])", "legendFormat": "{{method}} {{path}}"}
        ]
      },
      {
        "title": "Plugin Health",
        "type": "table",
        "targets": [
          {"expr": "lynx_plugin_health_status", "legendFormat": "{{plugin_name}}"}
        ]
      },
      {
        "title": "Database Connections",
        "type": "graph",
        "targets": [
          {"expr": "lynx_mysql_connection_pool_open", "legendFormat": "Open"},
          {"expr": "lynx_mysql_connection_pool_in_use", "legendFormat": "In Use"}
        ]
      }
    ]
  }
}
```

### Prometheus Alert Rules

```yaml
groups:
  - name: lynx-alerts
    rules:
      - alert: LynxAppUnhealthy
        expr: lynx_health_check_status == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Lynx application is unhealthy"
      
      - alert: HighErrorRate
        expr: rate(lynx_plugin_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
      
      - alert: DatabaseConnectionPoolExhausted
        expr: lynx_mysql_connection_pool_open >= lynx_mysql_connection_pool_max_open * 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database connection pool near exhaustion"
      
      - alert: HighRequestLatency
        expr: histogram_quantile(0.95, rate(lynx_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High request latency (p95 > 1s)"
```

## 🔄 Error Handling and Recovery

### Circuit Breaker

```go
import (
    "github.com/go-lynx/lynx"
    "github.com/go-lynx/lynx/observability/metrics"
)

func main() {
    productionMetrics := metrics.NewProductionMetrics()
    productionMetrics.Start()
    defer productionMetrics.Stop()

    errorRecoveryManager := lynx.NewErrorRecoveryManager(productionMetrics)
    defer errorRecoveryManager.Stop()

    // Record error
    errorRecoveryManager.RecordError(
        "database",
        "connection timeout",
        "mysql-plugin",
        lynx.ErrorSeverityMedium,
        map[string]interface{}{
            "timeout": "5s",
            "retries": 3,
        },
    )

    // Check health
    if !errorRecoveryManager.IsHealthy() {
        log.Warn("System is unhealthy")
    }
}
```

### Custom Recovery Strategy

```go
type DatabaseRecoveryStrategy struct {
    name    string
    timeout time.Duration
}

func (s *DatabaseRecoveryStrategy) Name() string {
    return s.name
}

func (s *DatabaseRecoveryStrategy) CanRecover(errorType string, severity lynx.ErrorSeverity) bool {
    return errorType == "database" && severity <= lynx.ErrorSeverityMedium
}

func (s *DatabaseRecoveryStrategy) Recover(ctx context.Context, record lynx.ErrorRecord) (bool, error) {
    // Implement reconnection logic
    return true, nil
}

func (s *DatabaseRecoveryStrategy) GetTimeout() time.Duration {
    return s.timeout
}

// Register
errorRecoveryManager.RegisterRecoveryStrategy("database", &DatabaseRecoveryStrategy{
    name:    "database-recovery",
    timeout: 10 * time.Second,
})
```

## 🏭 Deployment Best Practices

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lynx-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: lynx-service
  template:
    metadata:
      labels:
        app: lynx-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      containers:
      - name: lynx-service
        image: lynx-service:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
        - containerPort: 9091
          name: metrics
        
        env:
        - name: LYNX_ENV
          value: "production"
        - name: LYNX_LOG_LEVEL
          value: "info"
        
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 10"]
---
apiVersion: v1
kind: Service
metadata:
  name: lynx-service
spec:
  selector:
    app: lynx-service
  ports:
  - name: http
    port: 8080
  - name: grpc
    port: 9090
  - name: metrics
    port: 9091
```

### Docker Compose (Local Development)

```yaml
version: '3.8'
services:
  lynx-service:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - LYNX_ENV=development
      - MYSQL_HOST=mysql
      - REDIS_HOST=redis
    depends_on:
      - mysql
      - redis
      - otel-collector
  
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: lynx
    volumes:
      - mysql-data:/var/lib/mysql
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  otel-collector:
    image: otel/opentelemetry-collector:latest
    ports:
      - "4317:4317"
      - "4318:4318"
  
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
  
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

volumes:
  mysql-data:
```

### Environment Variables

```bash
# Application
export LYNX_APP_NAME=my-service
export LYNX_APP_VERSION=1.0.0
export LYNX_ENV=production

# Graceful shutdown
export LYNX_SHUTDOWN_TIMEOUT=30s

# Monitoring
export LYNX_METRICS_PORT=9090
export LYNX_HEALTH_CHECK_INTERVAL=30s

# Database
export MYSQL_HOST=mysql-primary
export MYSQL_USER=lynx
export MYSQL_PASSWORD=secret
export MYSQL_DATABASE=lynx

# Redis
export REDIS_ADDRS=redis-cluster:6379

# Tracing
export OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
export OTEL_SERVICE_NAME=my-service
```

## 🔍 Troubleshooting

### Debug Commands

```bash
# Check application health
curl http://localhost:8080/health

# View metrics
curl http://localhost:9090/metrics

# View error statistics
curl http://localhost:8080/debug/error-stats

# View plugin status
curl http://localhost:8080/debug/plugins
```

### Common Issues

1. **Plugin Initialization Failure**
   - Check configuration syntax
   - Verify external service connectivity
   - Review plugin dependencies

2. **Connection Pool Exhaustion**
   - Increase pool size
   - Check for connection leaks
   - Monitor connection usage patterns

3. **High Memory Usage**
   - Adjust buffer sizes
   - Check event history limits
   - Monitor goroutine count

4. **Graceful Shutdown Timeout**
   - Increase shutdown timeout
   - Ensure plugins implement cleanup correctly
   - Check for blocked operations

### Debug Mode

```go
import "github.com/go-lynx/lynx/log"

// Enable debug logging
log.SetLevel(log.DebugLevel)
```

## 📚 Reference

- [Lynx Framework Documentation](https://go-lynx.cn/docs)
- [Plugin Development Guide](https://go-lynx.cn/docs/plugins)
- [Prometheus Monitoring](https://prometheus.io/docs/)
- [OpenTelemetry](https://opentelemetry.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/)

## 🤝 Contributing

- [GitHub Repository](https://github.com/go-lynx/lynx)
- [GitHub Issues](https://github.com/go-lynx/lynx/issues)
- [GitHub Discussions](https://github.com/go-lynx/lynx/discussions)
