package pulsar

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/mq/pulsar/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata constants
const (
	pluginName        = "pulsar.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "Apache Pulsar client plugin for lynx framework"
	confPrefix        = "lynx.pulsar"
)

// PulsarClient represents the main Pulsar client plugin instance
type PulsarClient struct {
	*plugins.BasePlugin
	config            *conf.Pulsar
	client            pulsar.Client
	producers         map[string]pulsar.Producer
	consumers         map[string]pulsar.Consumer
	producerMutex     sync.RWMutex
	consumerMutex     sync.RWMutex
	closeChan         chan struct{}
	closeOnce         sync.Once // Protect against multiple close operations
	closed            bool
	metrics           *Metrics
	healthStatus      *HealthStatus
	healthChecker     *HealthChecker
	connectionManager *ConnectionManager
	retryManager      *RetryManager
}

// NewPulsarClient creates a new Pulsar client plugin instance
func NewPulsarClient() *PulsarClient {
	pulsarConf := &conf.Pulsar{
		ServiceUrl: "pulsar://localhost:6650",
		Connection: &conf.Connection{
			ConnectionTimeout:       durationpb.New(30 * time.Second),
			OperationTimeout:        durationpb.New(30 * time.Second),
			KeepAliveInterval:       durationpb.New(30 * time.Second),
			MaxConnectionsPerHost:   1,
			EnableConnectionPooling: true,
		},
		Retry: &conf.Retry{
			Enable:               true,
			MaxAttempts:          3,
			InitialDelay:         durationpb.New(100 * time.Millisecond),
			MaxDelay:             durationpb.New(30 * time.Second),
			RetryDelayMultiplier: 2.0,
			JitterFactor:         0.1,
		},
		Monitoring: &conf.Monitoring{
			EnableMetrics:       true,
			MetricsNamespace:    "lynx_pulsar",
			EnableHealthCheck:   true,
			HealthCheckInterval: durationpb.New(30 * time.Second),
		},
		Producers: []*conf.Producer{
			{
				Name:    "default-producer",
				Enabled: true,
				Topic:   "default-topic",
				Options: &conf.ProducerOptions{
					SendTimeout:             durationpb.New(30 * time.Second),
					MaxPendingMessages:      1000,
					BatchingEnabled:         true,
					BatchingMaxPublishDelay: durationpb.New(10 * time.Millisecond),
					BatchingMaxMessages:     1000,
					CompressionType:         "none",
					HashingScheme:           "java_string_hash",
					MessageRoutingMode:      "round_robin",
				},
			},
		},
		Consumers: []*conf.Consumer{
			{
				Name:             "default-consumer",
				Enabled:          true,
				Topics:           []string{"default-topic"},
				SubscriptionName: "default-subscription",
				Options: &conf.ConsumerOptions{
					SubscriptionType:            "exclusive",
					SubscriptionInitialPosition: "latest",
					SubscriptionMode:            "durable",
					ReceiverQueueSize:           1000,
					EnableRetryOnMessageFailure: true,
					RetryEnable:                 true,
					NegativeAckDelay:            durationpb.New(1 * time.Minute),
					CryptoFailureAction:         "fail",
				},
			},
		},
	}

	c := &PulsarClient{
		config:       pulsarConf,
		producers:    make(map[string]pulsar.Producer),
		consumers:    make(map[string]pulsar.Consumer),
		closeChan:    make(chan struct{}),
		closed:       false,
		metrics:      &Metrics{},
		healthStatus: &HealthStatus{},
	}

	c.BasePlugin = plugins.NewBasePlugin(
		plugins.GeneratePluginID("", pluginName, pluginVersion),
		pluginName,
		pluginDescription,
		pluginVersion,
		confPrefix,
		103, // Weight for Pulsar
	)

	return c
}

// Configure updates Pulsar configuration
func (p *PulsarClient) Configure(c any) error {
	if pulsarConf, ok := c.(*conf.Pulsar); ok {
		p.config = pulsarConf
		return nil
	}
	return plugins.ErrInvalidConfiguration
}

// InitializeResources initializes the plugin with configuration
func (p *PulsarClient) InitializeResources(rt plugins.Runtime) error {
	// Initialize base plugin
	if err := p.BasePlugin.InitializeResources(rt); err != nil {
		return err
	}

	// Initialize managers
	if p.config.Monitoring != nil {
		p.healthChecker = NewHealthChecker(p.config.Monitoring.HealthCheckInterval.AsDuration())
	}
	if p.config.Connection != nil {
		p.connectionManager = NewConnectionManager(p.config.Connection)
	}
	if p.config.Retry != nil {
		p.retryManager = NewRetryManager(p.config.Retry)
	}

	return nil
}

// StartupTasks initializes Pulsar client and performs health check
func (p *PulsarClient) StartupTasks() error {
	log.Infof("initializing Apache Pulsar client")

	// Create Pulsar client
	clientOptions := p.buildClientOptions()
	client, err := pulsar.NewClient(clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create Pulsar client: %w", err)
	}
	p.client = client

	// Initialize producers
	if err := p.initializeProducers(); err != nil {
		return fmt.Errorf("failed to initialize producers: %w", err)
	}

	// Initialize consumers
	if err := p.initializeConsumers(); err != nil {
		return fmt.Errorf("failed to initialize consumers: %w", err)
	}

	// Start health checker
	if p.config.Monitoring.EnableHealthCheck {
		p.healthChecker.Start()
	}

	// Start connection manager
	p.connectionManager.Start()

	log.Infof("Apache Pulsar client successfully initialized")
	return nil
}

// CleanupTasks gracefully shuts down the plugin
func (p *PulsarClient) CleanupTasks() error {
	log.Infof("shutting down Apache Pulsar client plugin")

	// Signal background tasks to stop (protected against multiple calls)
	p.closeOnce.Do(func() {
		close(p.closeChan)
	})
	p.closed = true

	// Stop health checker
	if p.healthChecker != nil {
		p.healthChecker.Stop()
	}

	// Stop connection manager
	if p.connectionManager != nil {
		p.connectionManager.Stop()
	}

	// Close consumers
	p.consumerMutex.Lock()
	for name, consumer := range p.consumers {
		consumer.Close()
		log.Infof("consumer %s closed", name)
	}
	p.consumers = make(map[string]pulsar.Consumer)
	p.consumerMutex.Unlock()

	// Close producers
	p.producerMutex.Lock()
	for name, producer := range p.producers {
		producer.Close()
		log.Infof("producer %s closed", name)
	}
	p.producers = make(map[string]pulsar.Producer)
	p.producerMutex.Unlock()

	// Close client
	if p.client != nil {
		p.client.Close()
	}

	log.Infof("Apache Pulsar client plugin successfully shut down")
	return nil
}

// CheckHealth performs health check on Pulsar client
func (p *PulsarClient) CheckHealth() error {
	if p.client == nil {
		return fmt.Errorf("pulsar client not initialized")
	}

	// Check connection status
	if !p.connectionManager.IsConnected() {
		return fmt.Errorf("pulsar client not connected")
	}

	// Check producer status
	p.producerMutex.RLock()
	for name, producer := range p.producers {
		if producer == nil {
			log.Warnf("producer %s is nil", name)
		}
	}
	p.producerMutex.RUnlock()

	// Check consumer status
	p.consumerMutex.RLock()
	for name, consumer := range p.consumers {
		if consumer == nil {
			log.Warnf("consumer %s is nil", name)
		}
	}
	p.consumerMutex.RUnlock()

	return nil
}

// buildClientOptions builds Pulsar client options from configuration
func (p *PulsarClient) buildClientOptions() pulsar.ClientOptions {
	options := pulsar.ClientOptions{
		URL: p.config.ServiceUrl,
	}

	// Connection options
	if p.config.Connection != nil {
		options.ConnectionTimeout = p.config.Connection.ConnectionTimeout.AsDuration()
		options.OperationTimeout = p.config.Connection.OperationTimeout.AsDuration()
		options.KeepAliveInterval = p.config.Connection.KeepAliveInterval.AsDuration()
		options.MaxConnectionsPerBroker = int(p.config.Connection.MaxConnectionsPerHost)
	}

	// TLS options
	if p.config.Tls != nil && p.config.Tls.Enable {
		options.TLSAllowInsecureConnection = p.config.Tls.AllowInsecureConnection
		if p.config.Tls.TrustCertsFile != "" {
			options.TLSTrustCertsFilePath = p.config.Tls.TrustCertsFile
		}
		options.TLSValidateHostname = p.config.Tls.VerifyHostname
	}

	// Authentication options
	if p.config.Auth != nil {
		switch p.config.Auth.Type {
		case "token":
			if p.config.Auth.Token != "" {
				options.Authentication = pulsar.NewAuthenticationToken(p.config.Auth.Token)
			}
		case "oauth2":
			if p.config.Auth.Oauth2 != nil {
				oauth2 := p.config.Auth.Oauth2
				authParams := map[string]string{
					"issuerEndpoint": oauth2.IssuerUrl,
					"clientId":       oauth2.ClientId,
					"clientSecret":   oauth2.ClientSecret,
					"audience":       oauth2.Audience,
					"scope":          oauth2.Scope,
				}
				options.Authentication = pulsar.NewAuthenticationOAuth2(authParams)
			}
		case "tls":
			if p.config.Auth.TlsAuth != nil {
				tlsAuth := p.config.Auth.TlsAuth
				options.Authentication = pulsar.NewAuthenticationTLS(
					tlsAuth.CertFile,
					tlsAuth.KeyFile,
				)
			}
		}
	}

	return options
}

// initializeProducers initializes all configured producers
func (p *PulsarClient) initializeProducers() error {
	for _, producerConfig := range p.GetEnabledProducers() {
		if err := p.createProducer(producerConfig); err != nil {
			return fmt.Errorf("failed to create producer %s: %w", producerConfig.Name, err)
		}
	}
	return nil
}

// initializeConsumers initializes all configured consumers
func (p *PulsarClient) initializeConsumers() error {
	for _, consumerConfig := range p.GetEnabledConsumers() {
		if err := p.createConsumer(consumerConfig); err != nil {
			return fmt.Errorf("failed to create consumer %s: %w", consumerConfig.Name, err)
		}
	}
	return nil
}

// createProducer creates a Pulsar producer
func (p *PulsarClient) createProducer(config *conf.Producer) error {
	options := pulsar.ProducerOptions{
		Topic: config.Topic,
	}

	if config.Options != nil {
		if config.Options.ProducerName != "" {
			options.Name = config.Options.ProducerName
		}
		if config.Options.SendTimeout != nil {
			options.SendTimeout = config.Options.SendTimeout.AsDuration()
		}
		if config.Options.MaxPendingMessages > 0 {
			options.MaxPendingMessages = int(config.Options.MaxPendingMessages)
		}
		if config.Options.BatchingEnabled {
			if config.Options.BatchingMaxPublishDelay != nil {
				options.BatchingMaxPublishDelay = config.Options.BatchingMaxPublishDelay.AsDuration()
			}
			options.BatchingMaxMessages = uint(config.Options.BatchingMaxMessages)
			options.BatchingMaxSize = uint(config.Options.BatchingMaxSize)
		}
		if config.Options.EnableChunking {
			options.EnableChunking = true
			options.ChunkMaxMessageSize = uint(config.Options.ChunkMaxSize)
		}
	}

	producer, err := p.client.CreateProducer(options)
	if err != nil {
		return err
	}

	p.producerMutex.Lock()
	p.producers[config.Name] = producer
	p.producerMutex.Unlock()

	log.Infof("producer %s created for topic %s", config.Name, config.Topic)
	return nil
}

// createConsumer creates a Pulsar consumer
func (p *PulsarClient) createConsumer(config *conf.Consumer) error {
	options := pulsar.ConsumerOptions{
		Topics:           config.Topics,
		SubscriptionName: config.SubscriptionName,
	}

	if config.Options != nil {
		if config.Options.ConsumerName != "" {
			options.Name = config.Options.ConsumerName
		}
		if config.Options.SubscriptionType != "" {
			options.Type = p.parseSubscriptionType(config.Options.SubscriptionType)
		}
		if config.Options.SubscriptionInitialPosition != "" {
			options.SubscriptionInitialPosition = p.parseSubscriptionInitialPosition(config.Options.SubscriptionInitialPosition)
		}
		if config.Options.ReceiverQueueSize > 0 {
			options.ReceiverQueueSize = int(config.Options.ReceiverQueueSize)
		}
		if config.Options.NegativeAckDelay != nil {
			options.NackRedeliveryDelay = config.Options.NegativeAckDelay.AsDuration()
		}
		if config.Options.Properties != nil {
			options.Properties = config.Options.Properties
		}
	}

	consumer, err := p.client.Subscribe(options)
	if err != nil {
		return err
	}

	p.consumerMutex.Lock()
	p.consumers[config.Name] = consumer
	p.consumerMutex.Unlock()

	log.Infof("consumer %s created for topics %v with subscription %s",
		config.Name, config.Topics, config.SubscriptionName)
	return nil
}

// parseSubscriptionType parses subscription type string to Pulsar type
func (p *PulsarClient) parseSubscriptionType(subType string) pulsar.SubscriptionType {
	switch subType {
	case "exclusive":
		return pulsar.Exclusive
	case "shared":
		return pulsar.Shared
	case "failover":
		return pulsar.Failover
	case "key_shared":
		return pulsar.KeyShared
	default:
		return pulsar.Exclusive
	}
}

// parseSubscriptionInitialPosition parses subscription initial position
func (p *PulsarClient) parseSubscriptionInitialPosition(pos string) pulsar.SubscriptionInitialPosition {
	switch pos {
	case "earliest":
		return pulsar.SubscriptionPositionEarliest
	case "latest":
		return pulsar.SubscriptionPositionLatest
	default:
		return pulsar.SubscriptionPositionLatest
	}
}

// GetPulsarConfig returns the current Pulsar configuration
func (p *PulsarClient) GetPulsarConfig() *conf.Pulsar {
	return p.config
}

// GetClient returns the underlying Pulsar client
func (p *PulsarClient) GetClient() pulsar.Client {
	return p.client
}

// IsConnected checks if the Pulsar client is connected
func (p *PulsarClient) IsConnected() bool {
	return !p.closed && p.client != nil && p.connectionManager.IsConnected()
}

// GetEnabledProducers returns all enabled producers
func (p *PulsarClient) GetEnabledProducers() []*conf.Producer {
	var enabled []*conf.Producer
	for _, producer := range p.config.Producers {
		if producer.Enabled {
			enabled = append(enabled, producer)
		}
	}
	return enabled
}

// GetEnabledConsumers returns all enabled consumers
func (p *PulsarClient) GetEnabledConsumers() []*conf.Consumer {
	var enabled []*conf.Consumer
	for _, consumer := range p.config.Consumers {
		if consumer.Enabled {
			enabled = append(enabled, consumer)
		}
	}
	return enabled
}
