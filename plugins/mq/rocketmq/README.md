# RocketMQ Plugin for Lynx Framework

## Overview

The RocketMQ plugin provides integration with Apache RocketMQ message queue system for the Lynx framework. It supports both producer and consumer functionality with comprehensive monitoring, health checks, and retry mechanisms.

## Features

- **Multi-instance Support**: Support for multiple producer and consumer instances
- **Health Monitoring**: Built-in health checks and connection management
- **Metrics Collection**: Comprehensive metrics for monitoring and observability
- **Retry Mechanism**: Configurable retry logic with exponential backoff
- **Connection Pooling**: Efficient connection and resource management
- **Graceful Shutdown**: Proper cleanup and resource management

## Configuration

### Basic Configuration

```yaml
rocketmq:
  name_server:
    - "127.0.0.1:9876"
    - "127.0.0.1:9877"
  access_key: "your-access-key"
  secret_key: "your-secret-key"
  dial_timeout: "3s"
  request_timeout: "30s"
  
  producers:
    - name: "default-producer"
      enabled: true
      group_name: "lynx-producer-group"
      max_retries: 3
      retry_backoff: "100ms"
      send_timeout: "3s"
      enable_trace: false
      
  consumers:
    - name: "default-consumer"
      enabled: true
      group_name: "lynx-consumer-group"
      consume_model: "CLUSTERING"
      consume_order: "CONCURRENTLY"
      max_concurrency: 1
      pull_batch_size: 32
      pull_interval: "100ms"
      enable_trace: false
```

### Producer Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable the producer |
| `group_name` | string | "lynx-producer-group" | Producer group name |
| `max_retries` | int | 3 | Maximum number of retries |
| `retry_backoff` | duration | "100ms" | Retry interval |
| `send_timeout` | duration | "3s" | Send message timeout |
| `name` | string | "" | Producer instance name |
| `topics` | []string | [] | Allowed topics for routing/permissions |
| `enable_trace` | bool | false | Whether to enable trace |

### Consumer Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable the consumer |
| `group_name` | string | "lynx-consumer-group" | Consumer group name |
| `consume_model` | string | "CLUSTERING" | Consumption model (CLUSTERING/BROADCASTING) |
| `consume_order` | string | "CONCURRENTLY" | Message consumption order (CONCURRENTLY/ORDERLY) |
| `max_concurrency` | int | 1 | Maximum processing concurrency |
| `pull_batch_size` | int | 32 | Pull batch size |
| `pull_interval` | duration | "100ms" | Pull interval |
| `name` | string | "" | Consumer instance name |
| `topics` | []string | [] | Subscribed topic list |
| `enable_trace` | bool | false | Whether to enable trace |

## Usage

### Producer Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/mq/rocketmq"
)

func main() {
    // Get RocketMQ client from plugin manager
    client := pluginManager.GetPlugin("rocketmq").(rocketmq.ClientInterface)
    
    // Send message
    err := client.SendMessage(context.Background(), "test-topic", []byte("Hello RocketMQ"))
    if err != nil {
        log.Fatal(err)
    }
    
    // Send message with specific producer
    err = client.SendMessageWith(context.Background(), "default-producer", "test-topic", []byte("Hello RocketMQ"))
    if err != nil {
        log.Fatal(err)
    }
}
```

### Consumer Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/mq/rocketmq"
    "github.com/apache/rocketmq-client-go/v2/primitive"
)

func main() {
    // Get RocketMQ client from plugin manager
    client := pluginManager.GetPlugin("rocketmq").(rocketmq.ClientInterface)
    
    // Define message handler
    handler := func(ctx context.Context, msg *primitive.MessageExt) error {
        log.Printf("Received message: %s", string(msg.Body))
        return nil
    }
    
    // Subscribe to topics
    err := client.Subscribe(context.Background(), []string{"test-topic"}, handler)
    if err != nil {
        log.Fatal(err)
    }
    
    // Subscribe with specific consumer
    err = client.SubscribeWith(context.Background(), "default-consumer", []string{"test-topic"}, handler)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Monitoring

The plugin provides comprehensive metrics that can be accessed through the `GetMetrics()` method:

```go
metrics := client.GetMetrics()
stats := metrics.GetStats()

// Access producer metrics
producerStats := stats["producer"].(map[string]interface{})
messagesSent := producerStats["messages_sent"].(int64)

// Access consumer metrics
consumerStats := stats["consumer"].(map[string]interface{})
messagesReceived := consumerStats["messages_received"].(int64)

// Access health metrics
healthStats := stats["health"].(map[string]interface{})
isHealthy := healthStats["is_healthy"].(bool)
```

## Health Checks

The plugin includes built-in health checks that monitor:

- Connection status
- Producer/Consumer readiness
- Message processing health
- Error rates and latencies

Health status can be checked programmatically:

```go
// Check if producer is ready
if client.IsProducerReady("default-producer") {
    // Producer is ready
}

// Check if consumer is ready
if client.IsConsumerReady("default-consumer") {
    // Consumer is ready
}
```

## Error Handling

The plugin provides comprehensive error handling with context-aware error wrapping:

```go
if err := client.SendMessage(ctx, topic, body); err != nil {
    if rocketmq.IsError(err, rocketmq.ErrProducerNotReady) {
        // Handle producer not ready error
    } else if rocketmq.IsError(err, rocketmq.ErrSendMessageFailed) {
        // Handle send message failed error
    }
}
```

## Dependencies

- `github.com/apache/rocketmq-client-go/v2` - Official RocketMQ Go client
- `github.com/go-lynx/lynx` - Lynx framework core

## License

This plugin is part of the Lynx framework and follows the same license terms.
