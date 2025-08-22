# Apache Pulsar Plugin

The Apache Pulsar Plugin is a comprehensive message queue integration plugin for the Lynx framework. It provides full support for Apache Pulsar, a high-performance, distributed messaging and streaming platform that combines the best features of traditional message queues and streaming systems.

## Features

### Core Messaging Support
- **Producer/Consumer Pattern**: Full support for Pulsar's producer and consumer APIs
- **Multiple Subscription Types**: Exclusive, Shared, Failover, and Key-Shared subscriptions
- **Topic Management**: Create, delete, and manage topics and subscriptions
- **Schema Registry**: Built-in support for Pulsar's schema registry
- **Multi-tenancy**: Support for Pulsar's multi-tenant architecture

### Advanced Messaging Features
- **Message Batching**: Configurable message batching for high throughput
- **Compression**: Support for multiple compression algorithms (LZ4, Zlib, Zstd, Snappy)
- **Message Routing**: Configurable message routing strategies
- **Chunking**: Support for large message chunking
- **Dead Letter Queue**: Built-in dead letter queue support
- **Retry Policies**: Configurable retry mechanisms with exponential backoff

### Security & Authentication
- **Token Authentication**: Simple token-based authentication
- **OAuth2 Integration**: Full OAuth2 authentication support
- **TLS Authentication**: Certificate-based authentication
- **TLS Encryption**: End-to-end encryption support
- **Multi-tenant Security**: Tenant and namespace isolation

### Performance & Scalability
- **Connection Pooling**: Efficient connection management
- **Async Operations**: Asynchronous message production and consumption
- **High Throughput**: Optimized for high-performance scenarios
- **Low Latency**: Minimal latency for real-time applications
- **Horizontal Scaling**: Support for Pulsar's distributed architecture

### Monitoring & Observability
- **Health Checks**: Comprehensive health monitoring
- **Metrics Collection**: Detailed performance metrics
- **Error Tracking**: Comprehensive error handling and reporting
- **Connection Monitoring**: Real-time connection status tracking
- **Performance Analytics**: Throughput and latency measurements

## Architecture

The plugin follows the Lynx framework's layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                    Pulsar Plugin Layer                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Client    │  │   Metrics   │  │   Configuration    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Core Pulsar Layer                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Producer   │  │  Consumer   │  │   Connection       │ │
│  │  Manager    │  │  Manager    │  │   Manager          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Pulsar Client Layer                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Pulsar    │  │   Schema    │  │   Authentication   │ │
│  │   Client    │  │   Registry  │  │     System         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
lynx:
  pulsar:
    service_url: "pulsar://localhost:6650"
    
    producers:
      - name: "default-producer"
        enabled: true
        topic: "default-topic"
    
    consumers:
      - name: "default-consumer"
        enabled: true
        topics:
          - "default-topic"
        subscription_name: "default-subscription"
```

### Advanced Configuration

```yaml
lynx:
  pulsar:
    service_url: "pulsar://pulsar-cluster:6650"
    
    auth:
      type: "token"
      token: "your-auth-token"
    
    tls:
      enable: true
      trust_certs_file: "/path/to/certs.pem"
    
    connection:
      connection_timeout: 60s
      operation_timeout: 60s
      max_connections_per_host: 5
    
    producers:
      - name: "high-throughput-producer"
        enabled: true
        topic: "high-throughput-topic"
        options:
          batching_enabled: true
          compression_type: "lz4"
          max_pending_messages: 10000
    
    consumers:
      - name: "batch-consumer"
        enabled: true
        topics:
          - "batch-topic"
        subscription_name: "batch-subscription"
        options:
          subscription_type: "shared"
          receiver_queue_size: 5000
          read_compacted: true
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/mq/pulsar"
)

func main() {
    // Get the Pulsar client instance
    pulsarClient := pulsar.GetPulsarClient()
    
    // Send a message
    ctx := context.Background()
    err := pulsarClient.Produce(ctx, "my-topic", []byte("key"), []byte("value"))
    if err != nil {
        panic(err)
    }
    
    // Subscribe to messages
    err = pulsarClient.Subscribe(ctx, []string{"my-topic"}, func(ctx context.Context, msg pulsar.Message) error {
        fmt.Printf("Received message: %s\n", string(msg.Payload()))
        return nil
    })
    if err != nil {
        panic(err)
    }
}
```

### Advanced Usage

```go
// Send message with properties
properties := map[string]string{
    "source": "lynx-framework",
    "version": "2.0.0",
}
err := pulsarClient.ProduceWithProperties(ctx, "my-topic", []byte("key"), []byte("value"), properties)

// Send message asynchronously
err = pulsarClient.ProduceAsync(ctx, "my-topic", []byte("key"), []byte("value"), func(id pulsar.MessageID, msg *pulsar.ProducerMessage, err error) {
    if err != nil {
        log.Printf("Failed to send message: %v", err)
    } else {
        log.Printf("Message sent successfully: %v", id)
    }
})

// Subscribe with regex pattern
err = pulsarClient.SubscribeWithRegex(ctx, "tenant-.*/namespace-.*/topic-.*", messageHandler)

// Get specific producer/consumer
producer := pulsarClient.GetProducer("my-producer")
consumer := pulsarClient.GetConsumer("my-consumer")

// Check health
err = pulsarClient.CheckHealth()
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Get metrics
metrics := pulsarClient.GetMetrics()
log.Printf("Messages sent: %d, received: %d", metrics.MessagesSent, metrics.MessagesReceived)
```

### Producer Configuration

```go
// Configure producer with specific options
producerConfig := &conf.Producer{
    Name:    "custom-producer",
    Enabled: true,
    Topic:   "custom-topic",
    Options: &conf.ProducerOptions{
        ProducerName:        "lynx-custom-producer",
        SendTimeout:         30 * time.Second,
        MaxPendingMessages:  5000,
        BatchingEnabled:     true,
        CompressionType:     "lz4",
        MessageRoutingMode:  "single_partition",
    },
}

// Update configuration
err := pulsarClient.Configure(&conf.Pulsar{
    Producers: []*conf.Producer{producerConfig},
})
```

### Consumer Configuration

```go
// Configure consumer with specific options
consumerConfig := &conf.Consumer{
    Name:             "custom-consumer",
    Enabled:          true,
    Topics:           []string{"custom-topic"},
    SubscriptionName: "custom-subscription",
    Options: &conf.ConsumerOptions{
        ConsumerName:                "lynx-custom-consumer",
        SubscriptionType:            "shared",
        SubscriptionInitialPosition: "earliest",
        ReceiverQueueSize:           2000,
        EnableRetryOnMessageFailure: true,
        DeadLetterPolicy: &conf.DeadLetterPolicy{
            MaxRedeliverCount: 5,
            DeadLetterTopic:   "dlq-topic",
        },
    },
}
```

## API Reference

### PulsarClient

The main client interface providing access to all Pulsar functionality.

#### Core Methods

- `Produce(ctx, topic, key, value) error` - Send a message
- `ProduceWithProperties(ctx, topic, key, value, properties) error` - Send message with properties
- `ProduceAsync(ctx, topic, key, value, callback) error` - Send message asynchronously
- `ProduceBatch(ctx, topic, messages) error` - Send messages in batch
- `Subscribe(ctx, topics, handler) error` - Subscribe to topics
- `SubscribeWithRegex(ctx, pattern, handler) error` - Subscribe with regex pattern

#### Management Methods

- `GetProducer(name) pulsar.Producer` - Get producer instance
- `GetConsumer(name) pulsar.Consumer` - Get consumer instance
- `IsProducerReady(name) bool` - Check producer status
- `IsConsumerReady(name) bool` - Check consumer status
- `Close(name) error` - Close producer/consumer

#### Configuration Methods

- `Configure(config) error` - Update configuration
- `GetPulsarConfig() *conf.Pulsar` - Get current configuration
- `GetClient() pulsar.Client` - Get underlying Pulsar client

#### Monitoring Methods

- `CheckHealth() error` - Perform health check
- `GetMetrics() *Metrics` - Get performance metrics
- `GetHealth() *HealthStatus` - Get health status
- `IsConnected() bool` - Check connection status

### Configuration Structures

See `conf/pulsar.go` for detailed configuration structure definitions.

## Message Patterns

### 1. Simple Producer/Consumer

```go
// Producer
err := pulsarClient.Produce(ctx, "simple-topic", []byte("key"), []byte("Hello Pulsar!"))

// Consumer
err = pulsarClient.Subscribe(ctx, []string{"simple-topic"}, func(ctx context.Context, msg pulsar.Message) error {
    fmt.Printf("Received: %s\n", string(msg.Payload()))
    return nil
})
```

### 2. Batch Processing

```go
// Producer with batching
producerConfig := &conf.Producer{
    Name: "batch-producer",
    Topic: "batch-topic",
    Options: &conf.ProducerOptions{
        BatchingEnabled:               true,
        BatchingMaxPublishDelay:      100 * time.Millisecond,
        BatchingMaxMessages:           1000,
        BatchingMaxSize:               131072, // 128KB
    },
}

// Consumer for batch processing
err = pulsarClient.Subscribe(ctx, []string{"batch-topic"}, func(ctx context.Context, msg pulsar.Message) error {
    // Process message in batch context
    return processBatchMessage(msg)
})
```

### 3. High Availability

```go
// Configure for high availability
config := &conf.Pulsar{
    ServiceURL: "pulsar://cluster1:6650,cluster2:6650,cluster3:6650",
    Connection: &conf.Connection{
        ConnectionTimeout:      60 * time.Second,
        OperationTimeout:       60 * time.Second,
        MaxConnectionsPerHost:  3,
        EnableConnectionPooling: true,
    },
}

// Failover subscription
consumerConfig := &conf.Consumer{
    Name: "ha-consumer",
    Topics: []string{"ha-topic"},
    SubscriptionName: "ha-subscription",
    Options: &conf.ConsumerOptions{
        SubscriptionType: "failover",
    },
}
```

### 4. Schema Evolution

```go
// Producer with schema
producerConfig := &conf.Producer{
    Name: "schema-producer",
    Topic: "schema-topic",
    // Schema will be automatically detected from the topic
}

// Consumer with schema validation
err = pulsarClient.Subscribe(ctx, []string{"schema-topic"}, func(ctx context.Context, msg pulsar.Message) error {
    // Schema validation happens automatically
    return processSchemaMessage(msg)
})
```

## Monitoring and Metrics

### Health Checks

The plugin provides comprehensive health monitoring:

```go
// Check overall health
err := pulsarClient.CheckHealth()
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Get detailed health status
health := pulsarClient.GetHealth()
if health.Healthy {
    log.Printf("All components healthy")
} else {
    log.Printf("Health issues detected: %v", health.LastError)
}
```

### Metrics Collection

```go
// Get performance metrics
metrics := pulsarClient.GetMetrics()
log.Printf("Throughput: %d msg/s", metrics.MessagesSent)
log.Printf("Latency: %v", metrics.AverageSendLatency)
log.Printf("Errors: %d", metrics.SendErrors)
```

### Connection Monitoring

```go
// Check connection status
if pulsarClient.IsConnected() {
    log.Printf("Pulsar client connected")
} else {
    log.Printf("Pulsar client disconnected")
}
```

## Deployment

### Local Development

```bash
cd plugins/mq/pulsar
go mod tidy
go build
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && go build -o pulsar-plugin .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/pulsar-plugin .
CMD ["./pulsar-plugin"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lynx-pulsar-plugin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: lynx-pulsar-plugin
  template:
    metadata:
      labels:
        app: lynx-pulsar-plugin
    spec:
      containers:
      - name: pulsar-plugin
        image: lynx-pulsar-plugin:latest
        ports:
        - containerPort: 8080
        env:
        - name: PULSAR_SERVICE_URL
          value: "pulsar://pulsar-service:6650"
        - name: PULSAR_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: pulsar-secret
              key: token
```

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Check service URL and port
   - Verify network connectivity
   - Check authentication credentials

2. **Authentication Errors**
   - Verify token/credentials
   - Check OAuth2 configuration
   - Verify TLS certificates

3. **Performance Issues**
   - Monitor connection pool usage
   - Check batching configuration
   - Review compression settings

4. **Message Loss**
   - Check acknowledgment settings
   - Verify subscription configuration
   - Monitor dead letter queue

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
lynx:
  pulsar:
    monitoring:
      enable_metrics: true
      enable_health_check: true
      health_check_interval: 10s
```

## Best Practices

### Performance Optimization

- Use message batching for high throughput
- Configure appropriate connection pool sizes
- Enable compression for large messages
- Use async operations for non-blocking calls

### Reliability

- Implement proper error handling
- Use dead letter queues for failed messages
- Configure appropriate retry policies
- Monitor health status regularly

### Security

- Use TLS encryption in production
- Implement proper authentication
- Use OAuth2 for enterprise deployments
- Regularly rotate authentication tokens

### Monitoring

- Set up health check alerts
- Monitor message throughput and latency
- Track error rates and connection status
- Use metrics for capacity planning

## Contributing

Contributions are welcome! Please see the main Lynx framework contribution guidelines.

## License

This plugin is part of the Lynx framework and follows the same license terms.

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Documentation](https://lynx.go-lynx.com)
- Community: [Lynx Community](https://community.go-lynx.com)
- Pulsar Documentation: [Apache Pulsar](https://pulsar.apache.org/docs/)
