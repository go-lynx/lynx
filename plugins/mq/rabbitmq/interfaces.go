package rabbitmq

import (
	"context"
	"time"

	"github.com/go-lynx/lynx/plugins"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageHandler defines the message processing function
type MessageHandler func(ctx context.Context, msg amqp.Delivery) error

// Producer RabbitMQ producer interface
type Producer interface {
	// PublishMessage publishes a message to the specified exchange
	PublishMessage(ctx context.Context, exchange, routingKey string, body []byte, opts ...amqp.Publishing) error

	// PublishMessageWith publishes a message by producer instance name
	PublishMessageWith(ctx context.Context, producerName, exchange, routingKey string, body []byte, opts ...amqp.Publishing) error

	// GetProducer gets the underlying producer channel
	GetProducer(name string) (*amqp.Channel, error)

	// IsProducerReady checks if the producer is ready
	IsProducerReady(name string) bool
}

// Consumer RabbitMQ consumer interface
type Consumer interface {
	// Subscribe subscribes to a queue and sets message handler
	Subscribe(ctx context.Context, queue string, handler MessageHandler) error

	// SubscribeWith subscribes by consumer instance name
	SubscribeWith(ctx context.Context, consumerName, queue string, handler MessageHandler) error

	// GetConsumer gets the underlying consumer channel
	GetConsumer(name string) (*amqp.Channel, error)

	// IsConsumerReady checks if the consumer is ready
	IsConsumerReady(name string) bool
}

// ClientInterface RabbitMQ client interface
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
