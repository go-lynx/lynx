# RabbitMQ Plugin for Lynx Framework

## Overview

The RabbitMQ plugin provides integration with RabbitMQ message broker for the Lynx framework. It supports both producer and consumer functionality with comprehensive monitoring, health checks, and retry mechanisms.

## Features

- **Multi-instance Support**: Support for multiple producer and consumer instances
- **Health Monitoring**: Built-in health checks and connection management
- **Metrics Collection**: Comprehensive metrics for monitoring and observability
- **Retry Mechanism**: Configurable retry logic with exponential backoff
- **Connection Pooling**: Efficient connection and channel management
- **Graceful Shutdown**: Proper cleanup and resource management
- **Exchange and Queue Management**: Automatic exchange and queue declaration

## Configuration

### Basic Configuration

```yaml
rabbitmq:
  urls:
    - "amqp://guest:guest@localhost:5672/"
    - "amqp://guest:guest@localhost:5673/"
  username: "guest"
  password: "guest"
  virtual_host: "/"
  dial_timeout: "3s"
  heartbeat: "30s"
  channel_pool_size: 10
  
  producers:
    - name: "default-producer"
      enabled: true
      exchange: "lynx.exchange"
      exchange_type: "direct"
      routing_key: "lynx.routing.key"
      max_retries: 3
      retry_backoff: "100ms"
      publish_timeout: "3s"
      exchange_durable: true
      exchange_auto_delete: false
      message_persistent: true
      
  consumers:
    - name: "default-consumer"
      enabled: true
      queue: "lynx.queue"
      exchange: "lynx.exchange"
      routing_key: "lynx.routing.key"
      consumer_tag: "lynx.consumer"
      max_concurrency: 1
      prefetch_count: 1
      queue_durable: true
      queue_auto_delete: false
      queue_exclusive: false
      auto_ack: false
```

### Producer Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable the producer |
| `exchange` | string | "lynx.exchange" | Exchange name |
| `exchange_type` | string | "direct" | Exchange type (direct/fanout/topic/headers) |
| `routing_key` | string | "" | Routing key |
| `max_retries` | int | 3 | Maximum number of retries |
| `retry_backoff` | duration | "100ms" | Retry interval |
| `publish_timeout` | duration | "3s" | Publish timeout |
| `name` | string | "" | Producer instance name |
| `exchange_durable` | bool | true | Whether exchange is durable |
| `exchange_auto_delete` | bool | false | Whether exchange is auto-deleted |
| `message_persistent` | bool | true | Whether message is persistent |

### Consumer Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Whether to enable the consumer |
| `queue` | string | "lynx.queue" | Queue name |
| `exchange` | string | "" | Exchange name to bind |
| `routing_key` | string | "" | Routing key for binding |
| `consumer_tag` | string | "lynx.consumer" | Consumer tag |
| `max_concurrency` | int | 1 | Maximum processing concurrency |
| `prefetch_count` | int | 1 | Prefetch count |
| `name` | string | "" | Consumer instance name |
| `queue_durable` | bool | true | Whether queue is durable |
| `queue_auto_delete` | bool | false | Whether queue is auto-deleted |
| `queue_exclusive` | bool | false | Whether queue is exclusive |
| `auto_ack` | bool | false | Whether to auto-ack messages |

## Usage

### Producer Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/mq/rabbitmq"
    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    // Get RabbitMQ client from plugin manager
    client := pluginManager.GetPlugin("rabbitmq").(rabbitmq.ClientInterface)
    
    // Publish message
    err := client.PublishMessage(context.Background(), "test.exchange", "test.routing.key", []byte("Hello RabbitMQ"))
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish message with specific producer
    err = client.PublishMessageWith(context.Background(), "default-producer", "test.exchange", "test.routing.key", []byte("Hello RabbitMQ"))
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish with custom options
    err = client.PublishMessage(context.Background(), "test.exchange", "test.routing.key", []byte("Hello RabbitMQ"),
        amqp.Publishing{
            ContentType: "text/plain",
            Priority:    0,
            Expiration:  "60000", // 60 seconds
        })
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
    "github.com/go-lynx/lynx/plugins/mq/rabbitmq"
    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    // Get RabbitMQ client from plugin manager
    client := pluginManager.GetPlugin("rabbitmq").(rabbitmq.ClientInterface)
    
    // Define message handler
    handler := func(ctx context.Context, msg amqp.Delivery) error {
        log.Printf("Received message: %s", string(msg.Body))
        
        // Acknowledge message
        msg.Ack(false)
        return nil
    }
    
    // Subscribe to queue
    err := client.Subscribe(context.Background(), "test.queue", handler)
    if err != nil {
        log.Fatal(err)
    }
    
    // Subscribe with specific consumer
    err = client.SubscribeWith(context.Background(), "default-consumer", "test.queue", handler)
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
- Channel health
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

## Exchange Types

The plugin supports all RabbitMQ exchange types:

- **Direct**: Routes messages based on exact routing key match
- **Fanout**: Broadcasts messages to all bound queues
- **Topic**: Routes messages based on wildcard routing key patterns
- **Headers**: Routes messages based on message headers

Example configuration for different exchange types:

```yaml
# Direct exchange
producers:
  - name: "direct-producer"
    exchange: "direct.exchange"
    exchange_type: "direct"
    routing_key: "user.created"

# Fanout exchange
producers:
  - name: "fanout-producer"
    exchange: "fanout.exchange"
    exchange_type: "fanout"
    routing_key: "" # Not used for fanout

# Topic exchange
producers:
  - name: "topic-producer"
    exchange: "topic.exchange"
    exchange_type: "topic"
    routing_key: "user.*.created"
```

## Error Handling

The plugin provides comprehensive error handling with context-aware error wrapping:

```go
if err := client.PublishMessage(ctx, exchange, routingKey, body); err != nil {
    if rabbitmq.IsError(err, rabbitmq.ErrProducerNotReady) {
        // Handle producer not ready error
    } else if rabbitmq.IsError(err, rabbitmq.ErrPublishMessageFailed) {
        // Handle publish message failed error
    }
}
```

## Dependencies

- `github.com/rabbitmq/amqp091-go` - Official RabbitMQ Go client
- `github.com/go-lynx/lynx` - Lynx framework core

## License

This plugin is part of the Lynx framework and follows the same license terms.
