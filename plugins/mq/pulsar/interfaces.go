package pulsar

import (
	"context"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/go-lynx/lynx/plugins"
)

// Producer represents a Pulsar producer interface
type Producer interface {
	// Produce sends a single message to the specified topic
	Produce(ctx context.Context, topic string, key, value []byte) error

	// ProduceWithProperties sends a message with properties
	ProduceWithProperties(ctx context.Context, topic string, key, value []byte, properties map[string]string) error

	// ProduceAsync sends a message asynchronously
	ProduceAsync(ctx context.Context, topic string, key, value []byte, callback func(pulsar.MessageID, *pulsar.ProducerMessage, error)) error

	// ProduceBatch sends messages in batch
	ProduceBatch(ctx context.Context, topic string, messages []*pulsar.ProducerMessage) error

	// ProduceWith sends a message by producer instance name
	ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error

	// GetProducer gets the underlying producer client
	GetProducer(name string) pulsar.Producer

	// IsProducerReady checks if the producer is ready
	IsProducerReady(name string) bool

	// Close closes the producer
	Close(name string) error
}

// Consumer represents a Pulsar consumer interface
type Consumer interface {
	// Subscribe subscribes to topics and sets message handler
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error

	// SubscribeWith subscribes by consumer instance name
	SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error

	// SubscribeWithRegex subscribes to topics matching a regex pattern
	SubscribeWithRegex(ctx context.Context, topicsPattern string, handler MessageHandler) error

	// GetConsumer gets the underlying consumer client
	GetConsumer(name string) pulsar.Consumer

	// IsConsumerReady checks if the consumer is ready
	IsConsumerReady(name string) bool

	// Close closes the consumer
	Close(name string) error

	// Unsubscribe unsubscribes from topics
	Unsubscribe(name string) error
}

// MessageHandler represents a message handler function
type MessageHandler func(ctx context.Context, msg pulsar.Message) error

// ClientInterface represents the main Pulsar client interface
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

	// GetHealth gets health status
	GetHealth() *HealthStatus
}

// MetricsProvider represents monitoring metrics provider interface
type MetricsProvider interface {
	// GetStats gets statistics
	GetStats() map[string]interface{}

	// Reset resets metrics
	Reset()

	// RecordMessageSent records a sent message
	RecordMessageSent(topic string, size int, duration time.Duration)

	// RecordMessageReceived records a received message
	RecordMessageReceived(topic string, size int)

	// RecordError records an error
	RecordError(topic string, errorType string)
}

// HealthCheckerInterface represents health checker interface
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

	// GetLastError gets last error
	GetLastError() error
}

// ConnectionManagerInterface represents connection manager interface
type ConnectionManagerInterface interface {
	// Start starts connection manager
	Start()

	// Stop stops connection manager
	Stop()

	// IsConnected checks if connected
	IsConnected() bool

	// GetConnectionStats gets connection statistics
	GetConnectionStats() map[string]interface{}

	// Reconnect attempts to reconnect
	Reconnect() error
}

// RetryManagerInterface represents retry manager interface
type RetryManagerInterface interface {
	// ShouldRetry determines if operation should be retried
	ShouldRetry(attempt int, err error) bool

	// GetRetryDelay gets retry delay for attempt
	GetRetryDelay(attempt int) time.Duration

	// RecordRetry records a retry attempt
	RecordRetry(operation string, attempt int, err error)

	// GetRetryStats gets retry statistics
	GetRetryStats() map[string]interface{}
}

// DeadLetterQueueInterface represents dead letter queue interface
type DeadLetterQueueInterface interface {
	// SendToDLQ sends message to dead letter queue
	SendToDLQ(topic string, message pulsar.Message, reason string) error

	// GetDLQStats gets dead letter queue statistics
	GetDLQStats() map[string]interface{}

	// ProcessDLQ processes dead letter queue messages
	ProcessDLQ(handler MessageHandler) error
}

// SchemaRegistryInterface represents schema registry interface
type SchemaRegistryInterface interface {
	// GetSchema gets schema for topic
	GetSchema(topic string) (pulsar.Schema, error)

	// RegisterSchema registers schema for topic
	RegisterSchema(topic string, schema pulsar.Schema) error

	// CheckCompatibility checks schema compatibility
	CheckCompatibility(topic string, schema pulsar.Schema) (bool, error)
}

// TopicManagerInterface represents topic management interface
type TopicManagerInterface interface {
	// CreateTopic creates a new topic
	CreateTopic(topic string, partitions int) error

	// DeleteTopic deletes a topic
	DeleteTopic(topic string) error

	// GetTopicInfo gets topic information
	GetTopicInfo(topic string) (map[string]interface{}, error)

	// ListTopics lists all topics
	ListTopics() ([]string, error)
}

// SubscriptionManagerInterface represents subscription management interface
type SubscriptionManagerInterface interface {
	// CreateSubscription creates a new subscription
	CreateSubscription(topic, subscription string, subscriptionType pulsar.SubscriptionType) error

	// DeleteSubscription deletes a subscription
	DeleteSubscription(topic, subscription string) error

	// GetSubscriptionInfo gets subscription information
	GetSubscriptionInfo(topic, subscription string) (map[string]interface{}, error)

	// ListSubscriptions lists all subscriptions for a topic
	ListSubscriptions(topic string) ([]string, error)
}

// Metrics represents Pulsar metrics
type Metrics struct {
	// Message counts
	MessagesSent     int64
	MessagesReceived int64
	MessagesFailed   int64

	// Message sizes
	TotalBytesSent     int64
	TotalBytesReceived int64

	// Latency
	AverageSendLatency    time.Duration
	AverageReceiveLatency time.Duration

	// Error counts
	SendErrors       int64
	ReceiveErrors    int64
	ConnectionErrors int64

	// Connection stats
	ActiveConnections int
	TotalConnections  int

	// Retry stats
	TotalRetries int64
	RetryErrors  int64
}

// HealthStatus represents health status
type HealthStatus struct {
	// Overall health
	Healthy bool

	// Last check time
	LastCheck time.Time

	// Error count
	ErrorCount int

	// Last error
	LastError error

	// Component status
	ProducerStatus   map[string]bool
	ConsumerStatus   map[string]bool
	ConnectionStatus bool

	// Metrics
	Metrics *Metrics
}
