package conf

import (
	"time"
)

// Pulsar represents the main Pulsar configuration
type Pulsar struct {
	// Service URL for Pulsar cluster
	ServiceURL string `json:"service_url" yaml:"service_url"`

	// Authentication configuration
	Auth *Auth `json:"auth" yaml:"auth"`

	// TLS configuration
	TLS *TLS `json:"tls" yaml:"tls"`

	// Connection configuration
	Connection *Connection `json:"connection" yaml:"connection"`

	// Producer configurations
	Producers []*Producer `json:"producers" yaml:"producers"`

	// Consumer configurations
	Consumers []*Consumer `json:"consumers" yaml:"consumers"`

	// Retry configuration
	Retry *Retry `json:"retry" yaml:"retry"`

	// Monitoring configuration
	Monitoring *Monitoring `json:"monitoring" yaml:"monitoring"`
}

// Auth represents authentication configuration
type Auth struct {
	// Authentication type (token, oauth2, tls, etc.)
	Type string `json:"type" yaml:"type"`

	// Token for token-based authentication
	Token string `json:"token" yaml:"token"`

	// OAuth2 configuration
	OAuth2 *OAuth2 `json:"oauth2" yaml:"oauth2"`

	// TLS authentication
	TLSAuth *TLSAuth `json:"tls_auth" yaml:"tls_auth"`
}

// OAuth2 represents OAuth2 authentication configuration
type OAuth2 struct {
	// OAuth2 issuer URL
	IssuerURL string `json:"issuer_url" yaml:"issuer_url"`

	// OAuth2 client ID
	ClientID string `json:"client_id" yaml:"client_id"`

	// OAuth2 client secret
	ClientSecret string `json:"client_secret" yaml:"client_secret"`

	// OAuth2 audience
	Audience string `json:"audience" yaml:"audience"`

	// OAuth2 scope
	Scope string `json:"scope" yaml:"scope"`
}

// TLSAuth represents TLS authentication configuration
type TLSAuth struct {
	// Certificate file path
	CertFile string `json:"cert_file" yaml:"cert_file"`

	// Private key file path
	KeyFile string `json:"key_file" yaml:"key_file"`

	// CA certificate file path
	CAFile string `json:"ca_file" yaml:"ca_file"`
}

// TLS represents TLS configuration
type TLS struct {
	// Enable TLS
	Enable bool `json:"enable" yaml:"enable"`

	// Allow insecure connection
	AllowInsecureConnection bool `json:"allow_insecure_connection" yaml:"allow_insecure_connection"`

	// Trust certificate file path
	TrustCertsFile string `json:"trust_certs_file" yaml:"trust_certs_file"`

	// Verify hostname
	VerifyHostname bool `json:"verify_hostname" yaml:"verify_hostname"`
}

// Connection represents connection configuration
type Connection struct {
	// Connection timeout
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`

	// Operation timeout
	OperationTimeout time.Duration `json:"operation_timeout" yaml:"operation_timeout"`

	// Keep alive interval
	KeepAliveInterval time.Duration `json:"keep_alive_interval" yaml:"keep_alive_interval"`

	// Max number of connections per host
	MaxConnectionsPerHost int32 `json:"max_connections_per_host" yaml:"max_connections_per_host"`

	// Connection max lifetime
	ConnectionMaxLifetime time.Duration `json:"connection_max_lifetime" yaml:"connection_max_lifetime"`

	// Enable connection pooling
	EnableConnectionPooling bool `json:"enable_connection_pooling" yaml:"enable_connection_pooling"`
}

// Producer represents producer configuration
type Producer struct {
	// Producer name
	Name string `json:"name" yaml:"name"`

	// Enable producer
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Topic name
	Topic string `json:"topic" yaml:"topic"`

	// Producer options
	Options *ProducerOptions `json:"options" yaml:"options"`
}

// ProducerOptions represents producer options
type ProducerOptions struct {
	// Producer name
	ProducerName string `json:"producer_name" yaml:"producer_name"`

	// Send timeout
	SendTimeout time.Duration `json:"send_timeout" yaml:"send_timeout"`

	// Max pending messages
	MaxPendingMessages int32 `json:"max_pending_messages" yaml:"max_pending_messages"`

	// Max pending messages across partitions
	MaxPendingMessagesAcrossPartitions int32 `json:"max_pending_messages_across_partitions" yaml:"max_pending_messages_across_partitions"`

	// Block if queue full
	BlockIfQueueFull bool `json:"block_if_queue_full" yaml:"block_if_queue_full"`

	// Batching enabled
	BatchingEnabled bool `json:"batching_enabled" yaml:"batching_enabled"`

	// Batching max publish delay
	BatchingMaxPublishDelay time.Duration `json:"batching_max_publish_delay" yaml:"batching_max_publish_delay"`

	// Batching max messages
	BatchingMaxMessages int32 `json:"batching_max_messages" yaml:"batching_max_messages"`

	// Batching max size
	BatchingMaxSize int32 `json:"batching_max_size" yaml:"batching_max_size"`

	// Compression type (none, lz4, zlib, zstd, snappy)
	CompressionType string `json:"compression_type" yaml:"compression_type"`

	// Hashing scheme (java_string_hash, murmur3_32hash, consistent_hashing)
	HashingScheme string `json:"hashing_scheme" yaml:"hashing_scheme"`

	// Message routing mode (round_robin, single_partition, custom_partition)
	MessageRoutingMode string `json:"message_routing_mode" yaml:"message_routing_mode"`

	// Enable chunking
	EnableChunking bool `json:"enable_chunking" yaml:"enable_chunking"`

	// Chunk max size
	ChunkMaxSize int32 `json:"chunk_max_size" yaml:"chunk_max_size"`
}

// Consumer represents consumer configuration
type Consumer struct {
	// Consumer name
	Name string `json:"name" yaml:"name"`

	// Enable consumer
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Topic names
	Topics []string `json:"topics" yaml:"topics"`

	// Subscription name
	SubscriptionName string `json:"subscription_name" yaml:"subscription_name"`

	// Consumer options
	Options *ConsumerOptions `json:"options" yaml:"options"`
}

// ConsumerOptions represents consumer options
type ConsumerOptions struct {
	// Consumer name
	ConsumerName string `json:"consumer_name" yaml:"consumer_name"`

	// Subscription type (exclusive, shared, failover, key_shared)
	SubscriptionType string `json:"subscription_type" yaml:"subscription_type"`

	// Subscription initial position (latest, earliest)
	SubscriptionInitialPosition string `json:"subscription_initial_position" yaml:"subscription_initial_position"`

	// Subscription mode (durable, non_durable)
	SubscriptionMode string `json:"subscription_mode" yaml:"subscription_mode"`

	// Receiver queue size
	ReceiverQueueSize int32 `json:"receiver_queue_size" yaml:"receiver_queue_size"`

	// Max total receiver queue size across partitions
	MaxTotalReceiverQueueSizeAcrossPartitions int32 `json:"max_total_receiver_queue_size_across_partitions" yaml:"max_total_receiver_queue_size_across_partitions"`

	// Consumer name prefix
	ConsumerNamePrefix string `json:"consumer_name_prefix" yaml:"consumer_name_prefix"`

	// Read compacted
	ReadCompacted bool `json:"read_compacted" yaml:"read_compacted"`

	// Enable retry on message failure
	EnableRetryOnMessageFailure bool `json:"enable_retry_on_message_failure" yaml:"enable_retry_on_message_failure"`

	// Dead letter policy
	DeadLetterPolicy *DeadLetterPolicy `json:"dead_letter_policy" yaml:"dead_letter_policy"`

	// Retry enable
	RetryEnable bool `json:"retry_enable" yaml:"retry_enable"`

	// Ack timeout
	AckTimeout time.Duration `json:"ack_timeout" yaml:"ack_timeout"`

	// Negative ack delay
	NegativeAckDelay time.Duration `json:"negative_ack_delay" yaml:"negative_ack_delay"`

	// Priority level
	PriorityLevel int32 `json:"priority_level" yaml:"priority_level"`

	// Crypto failure action (fail, discard, consume)
	CryptoFailureAction string `json:"crypto_failure_action" yaml:"crypto_failure_action"`

	// Properties
	Properties map[string]string `json:"properties" yaml:"properties"`
}

// DeadLetterPolicy represents dead letter policy configuration
type DeadLetterPolicy struct {
	// Max redeliver count
	MaxRedeliverCount int32 `json:"max_redeliver_count" yaml:"max_redeliver_count"`

	// Dead letter topic
	DeadLetterTopic string `json:"dead_letter_topic" yaml:"dead_letter_topic"`

	// Initial subscription name
	InitialSubscriptionName string `json:"initial_subscription_name" yaml:"initial_subscription_name"`
}

// Retry represents retry configuration
type Retry struct {
	// Enable retry
	Enable bool `json:"enable" yaml:"enable"`

	// Max retry attempts
	MaxAttempts int32 `json:"max_attempts" yaml:"max_attempts"`

	// Initial retry delay
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`

	// Max retry delay
	MaxDelay time.Duration `json:"max_delay" yaml:"max_delay"`

	// Retry delay multiplier
	RetryDelayMultiplier float32 `json:"retry_delay_multiplier" yaml:"retry_delay_multiplier"`

	// Jitter factor
	JitterFactor float32 `json:"jitter_factor" yaml:"jitter_factor"`
}

// Monitoring represents monitoring configuration
type Monitoring struct {
	// Enable metrics
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`

	// Metrics namespace
	MetricsNamespace string `json:"metrics_namespace" yaml:"metrics_namespace"`

	// Enable health check
	EnableHealthCheck bool `json:"enable_health_check" yaml:"enable_health_check"`

	// Health check interval
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`

	// Enable tracing
	EnableTracing bool `json:"enable_tracing" yaml:"enable_tracing"`
}

// Validate validates the configuration and sets default values
func (p *Pulsar) Validate() error {
	// Set default service URL if not provided
	if p.ServiceURL == "" {
		p.ServiceURL = "pulsar://localhost:6650"
	}

	// Set default connection configuration if not provided
	if p.Connection == nil {
		p.Connection = &Connection{}
	}
	if p.Connection.ConnectionTimeout == 0 {
		p.Connection.ConnectionTimeout = 30 * time.Second
	}
	if p.Connection.OperationTimeout == 0 {
		p.Connection.OperationTimeout = 30 * time.Second
	}
	if p.Connection.KeepAliveInterval == 0 {
		p.Connection.KeepAliveInterval = 30 * time.Second
	}
	if p.Connection.MaxConnectionsPerHost == 0 {
		p.Connection.MaxConnectionsPerHost = 1
	}
	if p.Connection.ConnectionMaxLifetime == 0 {
		p.Connection.ConnectionMaxLifetime = 0 // No limit
	}

	// Set default retry configuration if not provided
	if p.Retry == nil {
		p.Retry = &Retry{}
	}
	if p.Retry.MaxAttempts == 0 {
		p.Retry.MaxAttempts = 3
	}
	if p.Retry.InitialDelay == 0 {
		p.Retry.InitialDelay = 100 * time.Millisecond
	}
	if p.Retry.MaxDelay == 0 {
		p.Retry.MaxDelay = 30 * time.Second
	}
	if p.Retry.RetryDelayMultiplier == 0 {
		p.Retry.RetryDelayMultiplier = 2.0
	}
	if p.Retry.JitterFactor == 0 {
		p.Retry.JitterFactor = 0.1
	}

	// Set default monitoring configuration if not provided
	if p.Monitoring == nil {
		p.Monitoring = &Monitoring{}
	}
	if p.Monitoring.MetricsNamespace == "" {
		p.Monitoring.MetricsNamespace = "lynx_pulsar"
	}
	if p.Monitoring.HealthCheckInterval == 0 {
		p.Monitoring.HealthCheckInterval = 30 * time.Second
	}

	// Validate producers
	for _, producer := range p.Producers {
		if producer.Options == nil {
			producer.Options = &ProducerOptions{}
		}
		if producer.Options.SendTimeout == 0 {
			producer.Options.SendTimeout = 30 * time.Second
		}
		if producer.Options.MaxPendingMessages == 0 {
			producer.Options.MaxPendingMessages = 1000
		}
		if producer.Options.CompressionType == "" {
			producer.Options.CompressionType = "none"
		}
		if producer.Options.HashingScheme == "" {
			producer.Options.HashingScheme = "java_string_hash"
		}
		if producer.Options.MessageRoutingMode == "" {
			producer.Options.MessageRoutingMode = "round_robin"
		}
	}

	// Validate consumers
	for _, consumer := range p.Consumers {
		if consumer.Options == nil {
			consumer.Options = &ConsumerOptions{}
		}
		if consumer.Options.SubscriptionType == "" {
			consumer.Options.SubscriptionType = "exclusive"
		}
		if consumer.Options.SubscriptionInitialPosition == "" {
			consumer.Options.SubscriptionInitialPosition = "latest"
		}
		if consumer.Options.SubscriptionMode == "" {
			consumer.Options.SubscriptionMode = "durable"
		}
		if consumer.Options.ReceiverQueueSize == 0 {
			consumer.Options.ReceiverQueueSize = 1000
		}
		if consumer.Options.AckTimeout == 0 {
			consumer.Options.AckTimeout = 0 // No timeout
		}
		if consumer.Options.NegativeAckDelay == 0 {
			consumer.Options.NegativeAckDelay = 1 * time.Minute
		}
		if consumer.Options.CryptoFailureAction == "" {
			consumer.Options.CryptoFailureAction = "fail"
		}
	}

	return nil
}

// GetProducerByName returns a producer configuration by name
func (p *Pulsar) GetProducerByName(name string) *Producer {
	for _, producer := range p.Producers {
		if producer.Name == name {
			return producer
		}
	}
	return nil
}

// GetConsumerByName returns a consumer configuration by name
func (p *Pulsar) GetConsumerByName(name string) *Consumer {
	for _, consumer := range p.Consumers {
		if consumer.Name == name {
			return consumer
		}
	}
	return nil
}

// GetEnabledProducers returns all enabled producers
func (p *Pulsar) GetEnabledProducers() []*Producer {
	var enabled []*Producer
	for _, producer := range p.Producers {
		if producer.Enabled {
			enabled = append(enabled, producer)
		}
	}
	return enabled
}

// GetEnabledConsumers returns all enabled consumers
func (p *Pulsar) GetEnabledConsumers() []*Consumer {
	var enabled []*Consumer
	for _, consumer := range p.Consumers {
		if consumer.Enabled {
			enabled = append(enabled, consumer)
		}
	}
	return enabled
}
