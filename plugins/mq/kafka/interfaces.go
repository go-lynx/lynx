package kafka

import (
	"context"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer Kafka producer interface
type Producer interface {
	// Produce sends a single message to the specified topic
	Produce(ctx context.Context, topic string, key, value []byte) error

	// ProduceBatch sends messages in batch to the specified topic
	ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error

	// ProduceWith sends a single message by producer instance name
	ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error

	// ProduceBatchWith sends messages in batch by producer instance name
	ProduceBatchWith(ctx context.Context, producerName string, topic string, records []*kgo.Record) error

	// GetProducer gets the underlying producer client
	GetProducer() *kgo.Client

	// IsProducerReady checks if the producer is ready
	IsProducerReady() bool
}

// Consumer Kafka consumer interface
type Consumer interface {
	// Subscribe subscribes to topics and sets message handler
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error

	// SubscribeWith subscribes by consumer instance name
	SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error

	// GetConsumer gets the underlying consumer client
	GetConsumer() *kgo.Client

	// IsConsumerReady checks if the consumer is ready
	IsConsumerReady() bool
}

// ClientInterface Kafka client interface
type ClientInterface interface {
	Producer
	Consumer

	// InitializeResources initializes resources
	InitializeResources(rt plugins.Runtime) error

	// StartupTasks startup tasks
	StartupTasks() error

	// ShutdownTasks shutdown tasks
	ShutdownTasks() error

	// GetMetrics gets monitoring metrics
	GetMetrics() *Metrics
}

// MetricsProvider monitoring metrics provider interface
type MetricsProvider interface {
	// GetStats gets statistics
	GetStats() map[string]interface{}

	// Reset resets metrics
	Reset()
}

// HealthCheckerInterface health checker interface
type HealthCheckerInterface interface {
	// Start starts health check
	Start()

	// Stop stops health check
	Stop()

	// IsHealthy checks if healthy
	IsHealthy() bool

	// GetLastCheck gets last check time
	GetLastCheck() time.Time

	// GetErrorCount gets error count
	GetErrorCount() int
}

// ConnectionManagerInterface connection manager interface
type ConnectionManagerInterface interface {
	// Start starts connection manager
	Start()

	// Stop stops connection manager
	Stop()

	// IsConnected checks if connected
	IsConnected() bool

	// GetHealthChecker gets health checker
	GetHealthChecker() HealthCheckerInterface

	// ForceReconnect forces reconnection
	ForceReconnect()
}

// BatchProcessorInterface batch processor interface
type BatchProcessorInterface interface {
	// AddRecord adds record
	AddRecord(ctx context.Context, record *kgo.Record) error

	// Flush forces processing
	Flush(ctx context.Context) error

	// Close closes processor
	Close()
}

// RetryHandlerInterface retry handler interface
type RetryHandlerInterface interface {
	// DoWithRetry executes operation with retry
	DoWithRetry(ctx context.Context, operation func() error) error
}

// GoroutinePoolInterface goroutine pool interface
type GoroutinePoolInterface interface {
	// Submit submits task
	Submit(task func())

	// Wait waits for completion
	Wait()
}
