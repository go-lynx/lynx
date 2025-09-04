package rocketmq

import (
	"context"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-lynx/lynx/plugins"
)

// MessageHandler defines the message processing function
type MessageHandler func(ctx context.Context, msg *primitive.MessageExt) error

// Producer RocketMQ producer interface
type Producer interface {
	// SendMessage sends a single message to the specified topic
	SendMessage(ctx context.Context, topic string, body []byte) error

	// SendMessageSync sends a message synchronously
	SendMessageSync(ctx context.Context, topic string, body []byte) (*primitive.SendResult, error)

	// SendMessageAsync sends a message asynchronously
	SendMessageAsync(ctx context.Context, topic string, body []byte) error

	// SendMessageWith sends a message by producer instance name
	SendMessageWith(ctx context.Context, producerName, topic string, body []byte) error

	// GetProducer gets the underlying producer client
	GetProducer(name string) (rocketmq.Producer, error)

	// IsProducerReady checks if the producer is ready
	IsProducerReady(name string) bool
}

// Consumer RocketMQ consumer interface
type Consumer interface {
	// Subscribe subscribes to topics and sets message handler
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error

	// SubscribeWith subscribes by consumer instance name
	SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error

	// GetConsumer gets the underlying consumer client
	GetConsumer(name string) (rocketmq.PushConsumer, error)

	// IsConsumerReady checks if the consumer is ready
	IsConsumerReady(name string) bool
}

// ClientInterface RocketMQ client interface
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
