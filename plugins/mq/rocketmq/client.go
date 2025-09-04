package rocketmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/mq/rocketmq/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Client RocketMQ client plugin
type Client struct {
	*plugins.BasePlugin
	conf *conf.RocketMQ
	// Multi-instance producers/consumers
	producers       map[string]primitive.Producer
	consumers       map[string]primitive.PushConsumer
	defaultProducer string
	defaultConsumer string
	// Connection managers
	prodConnMgrs map[string]*ConnectionManager
	consConnMgrs map[string]*ConnectionManager
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	metrics      *Metrics
	retryHandler *RetryHandler
}

// Ensure Client implements all interfaces
var _ ClientInterface = (*Client)(nil)
var _ Producer = (*Client)(nil)
var _ Consumer = (*Client)(nil)

// NewRocketMQClient creates a new RocketMQ client plugin instance
func NewRocketMQClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
			confPrefix,
			100,
		),
		ctx:          ctx,
		cancel:       cancel,
		metrics:      NewMetrics(),
		retryHandler: NewRetryHandler(RetryConfig{MaxRetries: 3, BackoffTime: time.Second, MaxBackoff: 30 * time.Second}),
		producers:    make(map[string]primitive.Producer),
		consumers:    make(map[string]primitive.PushConsumer),
		prodConnMgrs: make(map[string]*ConnectionManager),
		consConnMgrs: make(map[string]*ConnectionManager),
	}
}

// InitializeResources initializes RocketMQ resources
func (r *Client) InitializeResources(rt plugins.Runtime) error {
	r.conf = &conf.RocketMQ{}

	// Load configuration
	err := rt.GetConfig().Value(confPrefix).Scan(r.conf)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	// Validate configuration
	if err := r.validateConfiguration(); err != nil {
		return err
	}

	// Set default values
	r.setDefaultValues()

	return nil
}

// StartupTasks startup tasks
func (r *Client) StartupTasks() error {
	// Initialize all enabled producer instances
	var firstProducerName string
	for _, p := range r.conf.Producers {
		if p == nil || !p.Enabled {
			continue
		}
		name := p.Name
		if name == "" {
			name = "default-producer"
		}

		producer, err := r.createProducer(name, p)
		if err != nil {
			return WrapError(err, "failed to create producer: "+name)
		}

		r.producers[name] = producer
		if firstProducerName == "" {
			firstProducerName = name
			r.defaultProducer = name
		}

		// Start connection manager for producer
		connMgr := NewConnectionManager(r.metrics)
		r.prodConnMgrs[name] = connMgr
		connMgr.Start()
	}

	// Initialize all enabled consumer instances
	var firstConsumerName string
	for _, c := range r.conf.Consumers {
		if c == nil || !c.Enabled {
			continue
		}
		name := c.Name
		if name == "" {
			name = "default-consumer"
		}

		consumer, err := r.createConsumer(name, c)
		if err != nil {
			return WrapError(err, "failed to create consumer: "+name)
		}

		r.consumers[name] = consumer
		if firstConsumerName == "" {
			firstConsumerName = name
			r.defaultConsumer = name
		}

		// Start connection manager for consumer
		connMgr := NewConnectionManager(r.metrics)
		r.consConnMgrs[name] = connMgr
		connMgr.Start()
	}

	log.Info("RocketMQ plugin started successfully")
	return nil
}

// ShutdownTasks shutdown tasks
func (r *Client) ShutdownTasks() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop all connection managers
	for name, connMgr := range r.prodConnMgrs {
		connMgr.Stop()
		log.Info("Stopped producer connection manager", "name", name)
	}

	for name, connMgr := range r.consConnMgrs {
		connMgr.Stop()
		log.Info("Stopped consumer connection manager", "name", name)
	}

	// Shutdown all producers
	for name, producer := range r.producers {
		if err := producer.Shutdown(); err != nil {
			log.Error("Failed to shutdown producer", "name", name, "error", err)
		} else {
			log.Info("Shutdown producer", "name", name)
		}
	}

	// Shutdown all consumers
	for name, consumer := range r.consumers {
		if err := consumer.Shutdown(); err != nil {
			log.Error("Failed to shutdown consumer", "name", name, "error", err)
		} else {
			log.Info("Shutdown consumer", "name", name)
		}
	}

	r.cancel()
	log.Info("RocketMQ plugin shutdown completed")
	return nil
}

// GetMetrics gets monitoring metrics
func (r *Client) GetMetrics() *Metrics {
	return r.metrics
}

// validateConfiguration validates the configuration
func (r *Client) validateConfiguration() error {
	if len(r.conf.NameServer) == 0 {
		return ErrMissingNameServer
	}

	// Validate producers
	for _, p := range r.conf.Producers {
		if p == nil || !p.Enabled {
			continue
		}
		if err := r.validateProducerConfig(p); err != nil {
			return err
		}
	}

	// Validate consumers
	for _, c := range r.conf.Consumers {
		if c == nil || !c.Enabled {
			continue
		}
		if err := r.validateConsumerConfig(c); err != nil {
			return err
		}
	}

	return nil
}

// validateProducerConfig validates producer configuration
func (r *Client) validateProducerConfig(p *conf.Producer) error {
	if p.GroupName == "" {
		p.GroupName = defaultProducerGroup
	}

	if err := validateGroupName(p.GroupName); err != nil {
		return err
	}

	for _, topic := range p.Topics {
		if err := validateTopic(topic); err != nil {
			return err
		}
	}

	return nil
}

// validateConsumerConfig validates consumer configuration
func (r *Client) validateConsumerConfig(c *conf.Consumer) error {
	if c.GroupName == "" {
		c.GroupName = defaultConsumerGroup
	}

	if err := validateGroupName(c.GroupName); err != nil {
		return err
	}

	if err := validateConsumeModel(c.ConsumeModel); err != nil {
		return err
	}

	if err := validateConsumeOrder(c.ConsumeOrder); err != nil {
		return err
	}

	for _, topic := range c.Topics {
		if err := validateTopic(topic); err != nil {
			return err
		}
	}

	return nil
}

// setDefaultValues sets default values for configuration
func (r *Client) setDefaultValues() {
	// Set default timeouts if not specified
	if r.conf.DialTimeout == nil {
		r.conf.DialTimeout = &durationpb.Duration{}
		r.conf.DialTimeout.FromDuration(parseDuration(defaultDialTimeout, 3*time.Second))
	}

	if r.conf.RequestTimeout == nil {
		r.conf.RequestTimeout = &durationpb.Duration{}
		r.conf.RequestTimeout.FromDuration(parseDuration(defaultRequestTimeout, 30*time.Second))
	}

	// Set producer defaults
	for _, p := range r.conf.Producers {
		if p == nil {
			continue
		}
		if p.MaxRetries == 0 {
			p.MaxRetries = defaultMaxRetries
		}
		if p.RetryBackoff == nil {
			p.RetryBackoff = &durationpb.Duration{}
			p.RetryBackoff.FromDuration(parseDuration(defaultRetryBackoff, 100*time.Millisecond))
		}
		if p.SendTimeout == nil {
			p.SendTimeout = &durationpb.Duration{}
			p.SendTimeout.FromDuration(parseDuration(defaultSendTimeout, 3*time.Second))
		}
	}

	// Set consumer defaults
	for _, c := range r.conf.Consumers {
		if c == nil {
			continue
		}
		if c.MaxConcurrency == 0 {
			c.MaxConcurrency = defaultMaxConcurrency
		}
		if c.PullBatchSize == 0 {
			c.PullBatchSize = defaultPullBatchSize
		}
		if c.PullInterval == nil {
			c.PullInterval = &durationpb.Duration{}
			c.PullInterval.FromDuration(parseDuration(defaultPullInterval, 100*time.Millisecond))
		}
	}
}

// createProducer creates a RocketMQ producer
func (r *Client) createProducer(name string, config *conf.Producer) (primitive.Producer, error) {
	// Create producer options
	opts := []primitive.ProducerOption{
		primitive.WithNameServer(r.conf.NameServer),
		primitive.WithGroupName(config.GroupName),
		primitive.WithRetry(config.MaxRetries),
		primitive.WithSendMsgTimeout(config.SendTimeout.AsDuration()),
	}

	// Add authentication if provided
	if r.conf.AccessKey != "" && r.conf.SecretKey != "" {
		opts = append(opts, primitive.WithCredentials(primitive.Credentials{
			AccessKey: r.conf.AccessKey,
			SecretKey: r.conf.SecretKey,
		}))
	}

	// Create producer
	producer, err := rocketmq.NewProducer(opts...)
	if err != nil {
		return nil, WrapError(err, "failed to create producer")
	}

	// Start producer
	if err := producer.Start(); err != nil {
		return nil, WrapError(err, "failed to start producer")
	}

	log.Info("Created RocketMQ producer", "name", name, "group", config.GroupName)
	return producer, nil
}

// createConsumer creates a RocketMQ consumer
func (r *Client) createConsumer(name string, config *conf.Consumer) (primitive.PushConsumer, error) {
	// Create consumer options
	opts := []primitive.ConsumerOption{
		primitive.WithNameServer(r.conf.NameServer),
		primitive.WithGroupName(config.GroupName),
		primitive.WithConsumeFromWhere(primitive.ConsumeFromLastOffset),
		primitive.WithConsumerModel(primitive.Clustering),
		primitive.WithConsumeOrderly(config.ConsumeOrder == ConsumeOrderOrderly),
	}

	// Add authentication if provided
	if r.conf.AccessKey != "" && r.conf.SecretKey != "" {
		opts = append(opts, primitive.WithCredentials(primitive.Credentials{
			AccessKey: r.conf.AccessKey,
			SecretKey: r.conf.SecretKey,
		}))
	}

	// Create consumer
	consumer, err := rocketmq.NewPushConsumer(opts...)
	if err != nil {
		return nil, WrapError(err, "failed to create consumer")
	}

	log.Info("Created RocketMQ consumer", "name", name, "group", config.GroupName)
	return consumer, nil
}
