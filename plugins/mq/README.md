# Message Queue Plugins for Lynx Framework

This directory contains message queue plugins for the Lynx framework, providing integration with popular message queue systems.

## Available Plugins

### 1. Kafka Plugin (`kafka/`)
- **Status**: Production Ready
- **Client**: `github.com/twmb/franz-go`
- **Features**: Producer/Consumer, Batch Processing, SASL/TLS, Health Monitoring
- **Documentation**: [Kafka Plugin README](kafka/README.md)

### 2. RocketMQ Plugin (`rocketmq/`)
- **Status**: Development Ready
- **Client**: `github.com/apache/rocketmq-client-go/v2`
- **Features**: Producer/Consumer, Clustering/Broadcasting, Health Monitoring
- **Documentation**: [RocketMQ Plugin README](rocketmq/README.md)

### 3. RabbitMQ Plugin (`rabbitmq/`)
- **Status**: Development Ready
- **Client**: `github.com/rabbitmq/amqp091-go`
- **Features**: Producer/Consumer, Exchange Types, Queue Management, Health Monitoring
- **Documentation**: [RabbitMQ Plugin README](rabbitmq/README.md)

## Common Features

All MQ plugins in this directory share the following common features:

### Core Functionality
- **Multi-instance Support**: Support for multiple producer and consumer instances
- **Health Monitoring**: Built-in health checks and connection management
- **Metrics Collection**: Comprehensive metrics for monitoring and observability
- **Retry Mechanism**: Configurable retry logic with exponential backoff
- **Connection Pooling**: Efficient connection and resource management
- **Graceful Shutdown**: Proper cleanup and resource management

### Plugin Architecture
- **Interface-based Design**: Clear interface contracts for extensibility
- **Configuration Management**: Protobuf-based configuration with YAML support
- **Error Handling**: Context-aware error wrapping and classification
- **Lifecycle Management**: Proper initialization, startup, and shutdown procedures

### Monitoring & Observability
- **Metrics**: Message counts, latencies, error rates
- **Health Checks**: Connection status, service readiness
- **Error Tracking**: Detailed error categorization and context
- **Performance Monitoring**: Throughput and latency measurements

## Quick Start

### 1. Choose Your Plugin

Select the appropriate plugin based on your message queue system:

```go
import (
    "github.com/go-lynx/lynx/plugins/mq/kafka"      // For Kafka
    "github.com/go-lynx/lynx/plugins/mq/rocketmq"   // For RocketMQ
    "github.com/go-lynx/lynx/plugins/mq/rabbitmq"   // For RabbitMQ
)
```

### 2. Configure Your Plugin

Add configuration to your application config:

```yaml
# For Kafka
kafka:
  brokers:
    - "localhost:9092"
  producers:
    - name: "default-producer"
      enabled: true
  consumers:
    - name: "default-consumer"
      enabled: true

# For RocketMQ
rocketmq:
  name_server:
    - "localhost:9876"
  producers:
    - name: "default-producer"
      enabled: true
  consumers:
    - name: "default-consumer"
      enabled: true

# For RabbitMQ
rabbitmq:
  urls:
    - "amqp://guest:guest@localhost:5672/"
  producers:
    - name: "default-producer"
      enabled: true
  consumers:
    - name: "default-consumer"
      enabled: true
```

### 3. Use the Plugin

```go
// Get the plugin from the plugin manager
client := pluginManager.GetPlugin("kafka").(kafka.ClientInterface)

// Send a message
err := client.SendMessage(context.Background(), "test-topic", []byte("Hello World"))

// Subscribe to messages
handler := func(ctx context.Context, msg *kgo.Record) error {
    log.Printf("Received: %s", string(msg.Value))
    return nil
}
err = client.Subscribe(context.Background(), []string{"test-topic"}, handler)
```

## Plugin Comparison

| Feature | Kafka | RocketMQ | RabbitMQ |
|---------|-------|----------|----------|
| **Message Model** | Topic-based | Topic-based | Exchange/Queue |
| **Message Ordering** | Per-partition | Per-queue | Per-queue |
| **Message Persistence** | Yes | Yes | Configurable |
| **Message Routing** | Partition-based | Tag-based | Exchange types |
| **Consumer Groups** | Yes | Yes | No (manual) |
| **Message Acknowledgment** | Auto/Manual | Auto/Manual | Auto/Manual |
| **Message Priority** | No | Yes | Yes |
| **Message TTL** | No | Yes | Yes |
| **Dead Letter Queue** | No | Yes | Yes |
| **Message Compression** | Yes | Yes | No |
| **Message Batching** | Yes | Yes | No |

## Development Status

### Production Ready
- **Kafka Plugin**: Fully implemented with comprehensive features

### Development Ready
- **RocketMQ Plugin**: Core structure implemented, needs client integration
- **RabbitMQ Plugin**: Core structure implemented, needs client integration

## Contributing

When contributing to MQ plugins:

1. **Follow the existing pattern**: Use the Kafka plugin as a reference
2. **Implement interfaces**: Ensure all plugins implement the common interfaces
3. **Add comprehensive tests**: Include unit tests and integration tests
4. **Update documentation**: Keep README files and examples up to date
5. **Follow error handling patterns**: Use the established error wrapping approach

## Testing

Each plugin includes:
- Unit tests for core functionality
- Integration tests with actual message queue systems
- Performance benchmarks
- Error handling tests

Run tests for a specific plugin:

```bash
# Test Kafka plugin
cd plugins/mq/kafka && go test ./...

# Test RocketMQ plugin
cd plugins/mq/rocketmq && go test ./...

# Test RabbitMQ plugin
cd plugins/mq/rabbitmq && go test ./...
```

## License

All plugins are part of the Lynx framework and follow the same license terms.
