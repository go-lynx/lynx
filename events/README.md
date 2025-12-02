# Lynx Unified Event System

The Lynx Unified Event System is a high-performance, multi-bus event processing system built on `kelindar/event`, providing unified event notification and management for the Lynx framework.

## Features

- **Multi-bus architecture**: 8 built-in buses for isolation and performance
- **High performance**: Built on `kelindar/event`, 4 E0x faster than traditional channels
- **Type safety**: Go generics and compile-time checks
- **Event filtering**: Complex filtering and routing
- **Event history**: Optional in-memory history
- **Monitoring metrics**: Rich performance metrics and health checks
- **Backward compatible**: Works with the existing plugin system
- **Easy ops**: One-click `PauseAll/ResumeAll` batch control
- **Enhanced observability**: New `PauseCount/IsDegraded/DegradationDuration/Worker*` stats

## Architecture

### Multi-bus Architecture

The system uses multiple dedicated buses to route different categories of events:

- **Plugin Bus**: Plugin lifecycle events
- **System Bus**: Internal system events
- **Business Bus**: Business events
- **Health Bus**: Health check events
- **Config Bus**: Configuration change events
- **Resource Bus**: Resource management events
- **Security Bus**: Security events
- **Metrics Bus**: Monitoring metric events

### Core Components

1. **EventBusManager**: Manages all event buses
2. **EventClassifier**: Event classification and routing
3. **LynxEventBus**: Single event bus implementation
4. **EventFilter**: Event filtering functionality
5. **EventListenerManager**: Listener management
6. **EventMonitor**: Monitoring and health checks

## Quick Start

### Initialization

```go
import "github.com/go-lynx/lynx/events"

// Initialize with default configuration
err := events.InitWithDefaultConfig()
if err != nil {
    log.Fatal(err)
}

// Initialize with custom configuration
configs := events.DefaultBusConfigs()
configs.Plugin.MaxQueue = 20000
err = events.Init(configs)
```

### Publishing Events

```go
// Create an event
event := events.NewLynxEvent(
    events.EventPluginInitialized,
    "my-plugin",
    "example",
).WithPriority(events.PriorityHigh).
  WithMetadata("version", "1.0.0")

// Publish to unified event bus
err := events.PublishEvent(event)
```

### Subscribing to Events

```go
// Subscribe to specific event type
err := events.SubscribeTo(events.EventPluginInitialized, func(event events.LynxEvent) {
    fmt.Printf("Plugin initialized: %s\n", event.PluginID)
})

// Subscribe to specific bus
err = events.Subscribe(events.BusTypePlugin, func(event events.LynxEvent) {
    fmt.Printf("Plugin event: %s\n", event.PluginID)
})
```

### Event Filtering

```go
// Create a filter
filter := events.NewEventFilter().
    WithEventType(events.EventPluginInitialized).
    WithPriority(events.PriorityHigh).
    WithPluginID("my-plugin")

// Add listener with filter
err := events.AddGlobalListener("my-listener", filter, func(event events.LynxEvent) {
    fmt.Printf("Filtered event: %s\n", event.PluginID)
}, events.BusTypePlugin)
```

## Configuration

### Bus Configuration

Each bus can be configured independently:

```go
config := events.BusConfig{
    MaxQueue:      10000,                    // Maximum queue size
    FlushInterval: 100 * time.Microsecond,   // Flush interval
    Priority:      events.PriorityNormal,    // Priority
    EnableHistory: true,                     // Enable history
    EnableMetrics: true,                     // Enable metrics
}
```

### Default Configuration

Optimized defaults are provided per bus type:

- **Plugin Bus**: High priority, large queue, history enabled
- **Health Bus**: Critical priority, small queue, history enabled
- **Metrics Bus**: Low priority, medium queue, metrics only
- **System Bus**: High priority, medium queue, history disabled

## Monitoring & Health

### System Health Status

```go
health := events.GetEventSystemHealth()
fmt.Printf("System healthy: %v\n", health.OverallHealthy)
for _, issue := range health.Issues {
    fmt.Printf("Issue: %s\n", issue)
}
```

### Global Performance Metrics

```go
metrics := events.GetGlobalMetrics()
fmt.Printf("Events published: %v\n", metrics["total_events_published"])
fmt.Printf("Events processed: %v\n", metrics["total_events_processed"])
fmt.Printf("Average latency: %v ms\n", metrics["avg_latency_ms"])
```

### Bus Status

```go
status := events.GetGlobalBusStatus()
for busType, s := range status {
    fmt.Printf("Bus %d: Healthy=%v, Paused=%v, Degraded=%v, Queue=%d, Subs=%d, PauseCount=%d, PauseDur=%s, DegradeDur=%s, Workers(cap/run/free/wait)=%d/%d/%d/%d\n",
        busType, s.IsHealthy, s.IsPaused, s.IsDegraded, s.QueueSize, s.Subscribers,
        s.PauseCount, s.PauseDuration, s.DegradationDuration, s.WorkerCap, s.WorkerRunning, s.WorkerFree, s.WorkerWaiting)
}

// You can also fetch a snapshot of all bus metrics (includes fields above and custom metrics)
all := events.GetGlobalEventBus().GetAllBusesMetrics()
```

## Integration with Plugin System

### Automatic Adaptation

The system automatically integrates with existing plugin systems without requiring changes to existing plugin code:

```go
// Events published in plugins are automatically routed to the unified event bus
runtime := plugins.NewSimpleRuntime()
runtime.EmitEvent(plugins.PluginEvent{
    Type:     plugins.EventPluginInitialized,
    PluginID: "my-plugin",
    // ... other fields
})
```

### Event Conversion

The system provides automatic event type conversion:

```go
// PluginEvent -> LynxEvent
pluginEvent := plugins.PluginEvent{...}
lynxEvent := events.ConvertPluginEvent(pluginEvent)

// LynxEvent -> PluginEvent
pluginEvent = events.ConvertLynxEvent(lynxEvent)
```

## Performance Optimization

### Queue Configuration

Adjust queue sizes based on business requirements:

```go
configs := events.DefaultBusConfigs()
// High throughput scenarios
configs.Business.MaxQueue = 50000
configs.Business.FlushInterval = 50 * time.Microsecond

// Low latency scenarios
configs.Health.MaxQueue = 1000
configs.Health.FlushInterval = 10 * time.Microsecond
```

### Listener Management

```go
// Dynamically add listeners
err := events.AddGlobalListener("dynamic-listener", nil, handler, events.BusTypePlugin)

// Remove listeners
err = events.RemoveGlobalListener("dynamic-listener")

// List all listeners
listeners := events.ListGlobalListeners()
```

## Best Practices

### 1. Event Categorization

- Send plugin lifecycle events to Plugin Bus
- Send health check events to Health Bus
- Send error events to System Bus
- Send business events to Business Bus

### 2. Priority Settings

- Use `PriorityCritical` for events requiring immediate processing
- Use `PriorityHigh` for important but non-urgent events
- Use `PriorityNormal` for regular events
- Use `PriorityLow` for events that can be delayed

### 3. Listener Design

- Set appropriate filters for each listener
- Avoid time-consuming operations in listeners
- Use listener IDs for easy management and debugging

### 4. Error Handling

```go
err := events.PublishEvent(event)
if err != nil {
    // Log error but don't block main flow
    log.Printf("Failed to publish event: %v", err)
}
```

## Examples

Complete usage examples can be found in:

- [Basic Example](examples/unified_event_system/main.go)
- [Advanced Example](examples/unified_event_system/advanced/main.go)

## API Reference

Detailed API documentation can be found in each package:

- [types.go](types.go) - Event types and basic structures
- [config.go](config.go) - Configuration management
- [classifier.go](classifier.go) - Event classification
- [lynx_event_bus.go](lynx_event_bus.go) - Event bus implementation
- [bus_manager.go](bus_manager.go) - Bus management
- [filters.go](filters.go) - Event filtering
- [listeners.go](listeners.go) - Listener management
- [monitor.go](monitor.go) - Monitoring and health checks
- [global.go](global.go) - Global interfaces
- [init.go](init.go) - Initialization functions

---

## Runtime Ops

- **Batch control**:

```go
// Pause all buses (publishing still enqueues)
paused, err := events.GetGlobalEventBus().PauseAll()
// Resume all buses
resumed, err := events.GetGlobalEventBus().ResumeAll()
```

- **Single bus control**: `Pause(busType)` / `Resume(busType)`

---

## Status & Metrics

- **New BusStatus fields** (see `bus_manager.go`):
  - `IsDegraded bool`
  - `DegradationDuration time.Duration`
  - `PauseCount int64`
  - `WorkerCap/WorkerRunning/WorkerFree/WorkerWaiting int`
- **Aggregation**: `GetBusStatus()` returns all bus statuses; `GetAllBusesMetrics()` returns a snapshot including the above and custom metrics
- **Per-bus custom metrics**: If metrics are enabled, see/extend `EventMetrics` in `LynxEventBus`

---

## Configuration Extensions (Throttling/Degradation)

New throttling defaults in `config.go` (enable as needed):

```go
EnableThrottling: false,
ThrottleRate:     1000, // events per second
ThrottleBurst:    100,  // burst size
```

Degradation switches and strategies are also supported (example):

```go
EnableDegradation:    true,
DegradationThreshold: 90, // queue usage threshold (%)
DegradationMode:      events.DegradationModeDrop,
```

---

## Changelog

- Added: `PauseAll()/ResumeAll()` batch ops capability
- Added: `PauseCount/IsDegraded/DegradationDuration/Worker*` status metrics
- Enhanced: `GetAllBusesMetrics()` with more observability fields
- Added: throttling config `EnableThrottling/ThrottleRate/ThrottleBurst`

---

## Prometheus Export (Example)

Below is a minimal example to export key metrics via Prometheus. It periodically scrapes event metrics and exposes them.

```go
package main

import (
    "net/http"
    "time"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/go-lynx/lynx/events"
)

var (
    evPublished = promauto.NewCounter(prometheus.CounterOpts{
        Name: "lynx_events_published_total",
        Help: "Total number of events published",
    })
    evProcessed = promauto.NewCounter(prometheus.CounterOpts{
        Name: "lynx_events_processed_total",
        Help: "Total number of events processed",
    })
    evDropped = promauto.NewCounter(prometheus.CounterOpts{
        Name: "lynx_events_dropped_total",
        Help: "Total number of events dropped",
    })
    evFailed = promauto.NewCounter(prometheus.CounterOpts{
        Name: "lynx_events_failed_total",
        Help: "Total number of events failed",
    })
    evLatencyMs = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "lynx_events_latency_ms",
        Help: "Current processing latency in milliseconds",
    })
)

func main() {
    // scrape loop
    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            gm := events.GetGlobalMetrics()
            // NOTE: keys are subject to your monitor implementation
            if v, ok := gm["total_events_published"].(int64); ok { evPublished.Add(float64(v)) }
            if v, ok := gm["total_events_processed"].(int64); ok { evProcessed.Add(float64(v)) }
            if v, ok := gm["total_events_dropped"].(int64); ok { evDropped.Add(float64(v)) }
            if v, ok := gm["total_events_failed"].(int64); ok { evFailed.Add(float64(v)) }
            if v, ok := gm["avg_latency_ms"].(int64); ok { evLatencyMs.Set(float64(v)) }
        }
    }()

    http.Handle("/metrics", promhttp.Handler())
    _ = http.ListenAndServe(":2112", nil)
}
```

### Suggested Alerts (PromQL)

```yaml
groups:
  - name: lynx-events-alerts
    rules:
      - alert: LynxEventsDroppedSpike
        expr: increase(lynx_events_dropped_total[5m]) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High dropped events in last 5m"
      - alert: LynxEventsLatencyHigh
        expr: lynx_events_latency_ms > 50
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Event processing latency > 50ms"
```

---

## Dynamic Config Effectiveness Matrix

The following table summarizes which config fields take effect at runtime and typical behavior. If a field is not hot-applied, restart or re-init may be required.

```
Field                 | Runtime-Effective | Behavior
--------------------- | ----------------- | ----------------------------------------------
MaxQueue              | No                | Requires re-init to resize internal queues
FlushInterval         | Partial           | May apply on next cycle; verify in your build
BatchSize             | Partial           | Applies to subsequent batches
WorkerCount           | Partial           | Depends on worker pool reconfiguration
EnableDegradation     | Yes               | Mode toggled immediately
DegradationThreshold  | Yes               | New threshold evaluated on next checks
DegradationMode       | Yes               | New strategy applied when threshold met
EnableThrottling      | Depends           | If supported, creates/destroys throttler at runtime
ThrottleRate/Burst    | Depends           | If supported, rebuild throttler to apply
ErrorCallback         | Yes               | New callback used for subsequent events
```

Note: For throttling fields, ensure your build supports hot-update. If not, plan a rolling restart.

---

## Ordering & Idempotency Best Practices

- **No strict ordering guarantee**: treat buses as concurrent pipelines. If ordering matters, carry sequence numbers and reorder at consumer.
- **Idempotent handlers**: listeners should tolerate retries; use idempotency keys derived from event content.
- **Avoid blocking I/O**: offload to worker pools; use timeouts and circuit breakers.
- **Compensation path**: on drop/throttle/overflow, record to audit storage in `ErrorCallback` for replay/compensation.
- **Backpressure awareness**: favor filters to reduce fan-out; monitor `WorkerWaiting` and `QueueSize`.
