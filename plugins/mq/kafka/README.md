# Kafka Plugin for Lynx Framework

The Kafka Plugin provides comprehensive Apache Kafka integration for the Lynx framework, supporting high-performance message production and consumption with advanced features like batch processing, retry mechanisms, and monitoring.

## Features

### Core Messaging Support
- **Producer/Consumer Pattern**: Full support for Kafka's producer and consumer APIs
- **Batch Processing**: Configurable message batching for high throughput
- **Retry Mechanisms**: Intelligent retry logic with exponential backoff
- **Connection Pooling**: Efficient connection and resource management
- **Graceful Shutdown**: Proper cleanup and resource management

### Advanced Features
- **SASL Authentication**: Support for SASL/PLAIN, SASL/SCRAM, and SASL/GSSAPI
- **TLS Encryption**: End-to-end encryption support
- **Compression**: Support for gzip, snappy, lz4, and zstd compression
- **Message Routing**: Configurable message routing strategies
- **Dead Letter Queue**: Built-in dead letter queue support
- **Schema Registry**: Integration with Confluent Schema Registry

### Performance & Monitoring
- **Prometheus Metrics**: Comprehensive monitoring and alerting
- **Health Checks**: Real-time health monitoring
- **Performance Analytics**: Throughput and latency measurements
- **Error Tracking**: Detailed error categorization and reporting
- **Connection Monitoring**: Real-time connection status tracking

## Architecture

The plugin follows the Lynx framework's layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                    Kafka Plugin Layer                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Client    │  │   Metrics   │  │   Configuration    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Core Kafka Layer                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Producer   │  │  Consumer   │  │   Connection       │ │
│  │  Manager    │  │  Manager    │  │   Manager          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Kafka Client Layer                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Kafka     │  │   Schema    │  │   Authentication   │ │
│  │   Client    │  │   Registry  │  │     System         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
lynx:
  kafka:
    brokers:
      - "localhost:9092"
      - "localhost:9093"
    client_id: "lynx-kafka-client"
    group_id: "lynx-consumer-group"
    
    producers:
      - name: "default-producer"
        enabled: true
        topic: "default-topic"
        max_retries: 3
        retry_backoff: "100ms"
        batch_size: 16384
        batch_timeout: "10ms"
        compression: "gzip"
    
    consumers:
      - name: "default-consumer"
        enabled: true
        topics:
          - "default-topic"
        group_id: "lynx-consumer-group"
        auto_offset_reset: "earliest"
        enable_auto_commit: true
        max_poll_records: 500
```

### Advanced Configuration

```yaml
lynx:
  kafka:
    brokers:
      - "kafka1:9092"
      - "kafka2:9092"
      - "kafka3:9092"
    
    # Security configuration
    security:
      sasl:
        enabled: true
        mechanism: "PLAIN"
        username: "kafka-user"
        password: "kafka-password"
      tls:
        enabled: true
        ca_file: "/path/to/ca-cert.pem"
        cert_file: "/path/to/client-cert.pem"
        key_file: "/path/to/client-key.pem"
        insecure_skip_verify: false
    
    # Connection configuration
    connection:
      timeout: 30s
      keep_alive: 30s
      max_connections: 100
    
    # Producer configuration
    producers:
      - name: "high-throughput-producer"
        enabled: true
        topic: "high-throughput-topic"
        options:
          batch_size: 65536
          batch_timeout: "5ms"
          compression: "lz4"
          max_retries: 5
          retry_backoff: "200ms"
          acks: "all"
          idempotent: true
    
    # Consumer configuration
    consumers:
      - name: "batch-consumer"
        enabled: true
        topics:
          - "batch-topic"
        group_id: "batch-consumer-group"
        options:
          auto_offset_reset: "earliest"
          enable_auto_commit: false
          max_poll_records: 1000
          session_timeout: 30s
          heartbeat_interval: 3s
          fetch_min_bytes: 1
          fetch_max_wait: "500ms"
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/mq/kafka"
)

func main() {
    // Get Kafka client from plugin manager
    client := pluginManager.GetPlugin("kafka").(kafka.ClientInterface)
    
    // Send message
    err := client.Produce(ctx, "test-topic", []byte("key"), []byte("Hello Kafka"))
    if err != nil {
        log.Fatal(err)
    }
    
    // Subscribe to messages
    err = client.Subscribe(ctx, []string{"test-topic"}, func(ctx context.Context, msg *kgo.Record) error {
        log.Printf("Received message: %s", string(msg.Value))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### Advanced Usage

```go
// Send message with specific producer
err := client.ProduceWith(ctx, "high-throughput-producer", "test-topic", []byte("key"), []byte("value"))

// Send batch messages
records := []*kgo.Record{
    {Topic: "test-topic", Key: []byte("key1"), Value: []byte("value1")},
    {Topic: "test-topic", Key: []byte("key2"), Value: []byte("value2")},
}
err = client.ProduceBatch(ctx, "test-topic", records)

// Subscribe with specific consumer
err = client.SubscribeWith(ctx, "batch-consumer", []string{"test-topic"}, messageHandler)

// Get producer/consumer instances
producer := client.GetProducer("default-producer")
consumer := client.GetConsumer("default-consumer")
```

### Message Handlers

```go
// Define message handler
messageHandler := func(ctx context.Context, msg *kgo.Record) error {
    log.Printf("Received message from topic %s: %s", msg.Topic, string(msg.Value))
    
    // Process message
    err := processMessage(msg)
    if err != nil {
        log.Printf("Failed to process message: %v", err)
        return err
    }
    
    return nil
}

// Subscribe with handler
err := client.Subscribe(ctx, []string{"test-topic"}, messageHandler)
```

## API Reference

### KafkaClientInterface

The main client interface providing access to all Kafka functionality.

#### Core Methods

- `Produce(ctx context.Context, topic string, key, value []byte) error` - Send a message
- `ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error` - Send with specific producer
- `ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error` - Send batch messages
- `Subscribe(ctx context.Context, topics []string, handler MessageHandler) error` - Subscribe to topics
- `SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error` - Subscribe with specific consumer

#### Management Methods

- `GetProducer(name string) *kgo.Client` - Get producer instance
- `GetConsumer(name string) *kgo.Client` - Get consumer instance
- `IsProducerReady(name string) bool` - Check producer status
- `IsConsumerReady(name string) bool` - Check consumer status
- `Close(name string) error` - Close producer/consumer

#### Monitoring Methods

- `GetMetrics() *Metrics` - Get performance metrics
- `CheckHealth() error` - Perform health check
- `GetHealthStatus() *HealthStatus` - Get health status

## Monitoring and Metrics

### Health Checks

```go
// Check overall health
err := client.CheckHealth()
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Get detailed health status
health := client.GetHealthStatus()
if health.Healthy {
    log.Printf("All components healthy")
} else {
    log.Printf("Health issues detected: %v", health.LastError)
}
```

### Prometheus Metrics

The plugin exposes comprehensive Prometheus metrics:

#### Producer Metrics
- `lynx_kafka_producer_messages_total` - Total messages sent
- `lynx_kafka_producer_bytes_total` - Total bytes sent
- `lynx_kafka_producer_errors_total` - Total producer errors
- `lynx_kafka_producer_duration_seconds` - Message send duration

#### Consumer Metrics
- `lynx_kafka_consumer_messages_total` - Total messages received
- `lynx_kafka_consumer_bytes_total` - Total bytes received
- `lynx_kafka_consumer_errors_total` - Total consumer errors
- `lynx_kafka_consumer_lag` - Consumer lag

#### Connection Metrics
- `lynx_kafka_connections_active` - Active connections
- `lynx_kafka_connections_total` - Total connections
- `lynx_kafka_connection_errors_total` - Connection errors

## Performance Tuning

### Producer Optimization

```yaml
producers:
  - name: "optimized-producer"
    options:
      batch_size: 65536          # Increase batch size
      batch_timeout: "5ms"       # Reduce batch timeout
      compression: "lz4"         # Use efficient compression
      acks: "1"                  # Reduce acknowledgment overhead
      retries: 3                 # Configure retries
      retry_backoff: "100ms"     # Retry backoff
```

### Consumer Optimization

```yaml
consumers:
  - name: "optimized-consumer"
    options:
      max_poll_records: 1000     # Increase poll size
      fetch_min_bytes: 1024      # Minimum fetch size
      fetch_max_wait: "500ms"    # Maximum fetch wait
      session_timeout: "30s"     # Session timeout
      heartbeat_interval: "3s"   # Heartbeat interval
```

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Check broker addresses and ports
   - Verify network connectivity
   - Check firewall settings

2. **Authentication Errors**
   - Verify SASL credentials
   - Check TLS certificates
   - Validate security configuration

3. **Performance Issues**
   - Monitor batch processing settings
   - Check compression configuration
   - Review retry settings

4. **Consumer Lag**
   - Increase consumer instances
   - Optimize processing logic
   - Check consumer group configuration

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
lynx:
  kafka:
    logging:
      level: "DEBUG"
      enable_sarama_logger: true
```

## Best Practices

### Message Design
- Use appropriate message sizes
- Implement message versioning
- Design for idempotency
- Use meaningful message keys

### Error Handling
- Implement proper retry logic
- Handle dead letter queues
- Monitor error rates
- Implement circuit breakers

### Performance
- Use batch processing effectively
- Configure appropriate timeouts
- Monitor resource usage
- Implement backpressure handling

### Monitoring
- Set up comprehensive monitoring
- Monitor consumer lag
- Track error rates
- Use metrics for capacity planning

## Dependencies

- `github.com/twmb/franz-go` - High-performance Kafka client
- `github.com/go-lynx/lynx` - Lynx framework core
- `github.com/prometheus/client_golang` - Prometheus metrics

## License

This plugin is part of the Lynx framework and follows the same license terms.

## Contributing

Contributions are welcome! Please see the main Lynx framework contribution guidelines.

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Documentation](https://lynx.go-lynx.com)
- Community: [Lynx Community](https://community.go-lynx.com)