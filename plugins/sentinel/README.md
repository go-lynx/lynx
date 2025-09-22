# Sentinel Plugin for Lynx

The Sentinel plugin provides traffic control, circuit breaking, and system protection capabilities for the Lynx framework.

## Features

- **Traffic Control**: Flow control based on QPS, concurrency, and other metrics
- **Circuit Breaking**: Automatic circuit breaking when services encounter exceptions to protect system stability
- **System Protection**: System-level protection based on system load, CPU usage, and other metrics
- **Real-time Monitoring**: Real-time traffic and performance metrics monitoring
- **Dashboard**: Built-in web console for visual rule management and monitoring data

## Configuration

Configure the Sentinel plugin under the `lynx.sentinel` configuration node:

```yaml
lynx:
  sentinel:
    enabled: true
    app_name: "my-app"
    log_level: "info"
    log_dir: "./logs/sentinel"
    
    # Flow control rules
    flow_rules:
      - resource: "/api/users"
        threshold: 100
        token_calculate_strategy: "direct"
        control_behavior: "reject"
        enabled: true
    
    # Circuit breaker rules
    circuit_breaker_rules:
      - resource: "/api/orders"
        strategy: "error_ratio"
        threshold: 0.5
        min_request_amount: 10
        retry_timeout_ms: 5000
        enabled: true
    
    # System rules
    system_rules:
      - metric_type: "load"
        threshold: 2.0
        strategy: "bbr"
        enabled: true
    
    # Metrics collection
    metrics:
      enabled: true
      interval: "1s"
    
    # Dashboard
    dashboard:
      enabled: true
      port: 8719
```

## Usage

### 1. Resource Protection

```go
import "github.com/go-lynx/lynx/plugins/sentinel"

// Method 1: Using Entry/Exit pattern
entry, err := sentinel.Entry("my-resource")
if err != nil {
    // Request blocked by flow control or circuit breaker
    return err
}
defer entry.Exit()

// Execute business logic
doSomething()

// Method 2: Using Execute wrapper
err := sentinel.Execute("my-resource", func() error {
    return doSomething()
})
```

### 2. HTTP Middleware

```go
// Create HTTP middleware
middleware, err := sentinel.CreateHTTPMiddleware(func(req interface{}) string {
    // Extract resource name from request
    return req.(*http.Request).URL.Path
})
```

### 3. gRPC Interceptor

```go
// Create gRPC interceptor
interceptor, err := sentinel.CreateGRPCInterceptor()
```

### 4. Dynamic Rule Management

```go
// Add flow control rule
err := sentinel.AddFlowRule(&sentinel.FlowRule{
    Resource:  "new-api",
    Threshold: 50,
    // ...
})

// Remove rule
err := sentinel.RemoveFlowRule("new-api")
```

### 5. Monitoring Data Retrieval

```go
// Get all metrics
metrics, err := sentinel.GetMetrics()

// Get specific resource statistics
stats, err := sentinel.GetResourceStats("my-resource")

// Get circuit breaker state
state, err := sentinel.GetCircuitBreakerState("my-resource")
```

## Rule Configuration Files

The plugin supports loading rule configurations from files:

- `conf/rules/flow_rules.json`: Flow control rules
- `conf/rules/circuit_breaker_rules.json`: Circuit breaker rules  
- `conf/rules/system_rules.json`: System protection rules

## Dashboard

After enabling the Dashboard, you can access `http://localhost:8719` via browser to view:

- Real-time traffic monitoring
- Rule configuration management
- System performance metrics
- Circuit breaker status

## Dependencies

- [Sentinel-Golang](https://github.com/alibaba/sentinel-golang): Alibaba's open-source traffic control component

## Notes

1. Ensure the Dashboard port doesn't conflict with other services
2. Set flow control thresholds reasonably to avoid excessive throttling
3. Circuit breaker rule thresholds should be adjusted based on actual business scenarios
4. It's recommended to enable metrics collection and monitoring in production environments