package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Client Kafka client plugin
type Client struct {
	*plugins.BasePlugin
	conf *conf.Kafka
	// Multi-instance producers/consumers
	producers       map[string]*kgo.Client
	batchProcessors map[string]*BatchProcessor
	defaultProducer string
	consumers       map[string]*kgo.Client
	activeGroups    map[string]*ConsumerGroup // Maintain active groups by consumer instance name
	// Connection managers
	prodConnMgrs map[string]*ConnectionManager
	consConnMgrs map[string]*ConnectionManager
	// Compatible with old fields, to be removed after consumer refactoring is complete
	producer            *kgo.Client
	consumer            *kgo.Client
	activeConsumerGroup *ConsumerGroup
	batchProcessor      *BatchProcessor
	mu                  sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
	metrics             *Metrics
	retryHandler        *RetryHandler
}

// Ensure Client implements all interfaces
var _ ClientInterface = (*Client)(nil)
var _ Producer = (*Client)(nil)
var _ Consumer = (*Client)(nil)

// NewKafkaClient creates a new Kafka client plugin instance
func NewKafkaClient() *Client {
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
		ctx:             ctx,
		cancel:          cancel,
		metrics:         NewMetrics(),
		retryHandler:    NewRetryHandler(RetryConfig{MaxRetries: 3, BackoffTime: time.Second, MaxBackoff: 30 * time.Second}),
		producers:       make(map[string]*kgo.Client),
		batchProcessors: make(map[string]*BatchProcessor),
		consumers:       make(map[string]*kgo.Client),
		activeGroups:    make(map[string]*ConsumerGroup),
		prodConnMgrs:    make(map[string]*ConnectionManager),
		consConnMgrs:    make(map[string]*ConnectionManager),
	}
}

// InitializeResources initializes Kafka resources
func (k *Client) InitializeResources(rt plugins.Runtime) error {
	k.conf = &conf.Kafka{}

	// Load configuration
	err := rt.GetConfig().Value(confPrefix).Scan(k.conf)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	// Validate configuration
	if err := k.validateConfiguration(); err != nil {
		return err
	}

	// Set default values
	k.setDefaultValues()

	return nil
}

// StartupTasks startup tasks
func (k *Client) StartupTasks() error {
	// Initialize all enabled producer instances
	var firstProducerName string
	for _, p := range k.conf.Producers {
		if p == nil || !p.Enabled {
			continue
		}
		name := p.Name
		if name == "" {
			// If unnamed, use topic combination or sequence number; here simply use first-available incremental name
			name = fmt.Sprintf("producer-%p", p)
		}
		if _, exists := k.producers[name]; exists {
			log.Warnf("duplicate producer name: %s, skip", name)
			continue
		}
		client, err := k.initProducerInstance(name, p)
		if err != nil {
			return fmt.Errorf("failed to initialize kafka producer %s: %w", name, err)
		}
		k.mu.Lock()
		k.producers[name] = client
		// Start producer connection manager
		if _, ok := k.prodConnMgrs[name]; !ok {
			cm := NewConnectionManager(client, k.conf.GetBrokers())
			k.prodConnMgrs[name] = cm
			cm.Start()
			log.Infof("Kafka producer[%s] connection manager started", name)
			// Register health metrics
			k.registerHealthForProducer(name)
		}
		if firstProducerName == "" {
			firstProducerName = name
		}
		// Each producer decides whether to enable async batch processing based on configuration
		batchSize := int(p.BatchSize)
		batchTimeout := p.BatchTimeout.AsDuration()
		if batchSize > 1 && batchTimeout > 0 {
			bp := NewBatchProcessor(batchSize, batchTimeout, func(ctx context.Context, recs []*kgo.Record) error {
				return k.ProduceBatchWith(ctx, name, "", recs)
			})
			k.batchProcessors[name] = bp
			log.Infof("Kafka producer[%s] batch processor started: size=%d, timeout=%s", name, batchSize, batchTimeout)
		} else {
			log.Infof("Kafka producer[%s] batch processor disabled (batch_size=%d, batch_timeout=%s)", name, batchSize, batchTimeout)
		}
		k.mu.Unlock()
		log.Infof("Kafka producer[%s] initialized successfully", name)
	}
	if firstProducerName != "" {
		k.defaultProducer = firstProducerName
	}

	// Consumers are initialized during Subscribe/SubscribeWith
	return nil
}

// ShutdownTasks shutdown tasks
func (k *Client) ShutdownTasks() error {
	k.cancel() // Cancel all contexts

	k.mu.Lock()
	defer k.mu.Unlock()

	// Stop connection managers (before closing clients)
	for name, cm := range k.prodConnMgrs {
		if cm != nil {
			cm.Stop()
			log.Infof("Kafka producer connection manager[%s] stopped", name)
		}
		delete(k.prodConnMgrs, name)
	}
	for name, cm := range k.consConnMgrs {
		if cm != nil {
			cm.Stop()
			log.Infof("Kafka consumer connection manager[%s] stopped", name)
		}
		delete(k.consConnMgrs, name)
	}

	// First gracefully flush all batch processors
	for name, bp := range k.batchProcessors {
		if bp == nil {
			continue
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = bp.Flush(flushCtx)
		cancel()
		bp.Close()
		delete(k.batchProcessors, name)
		log.Infof("Kafka producer batch processor[%s] closed", name)
	}

	for name, p := range k.producers {
		if p != nil {
			p.Close()
			log.Infof("Kafka producer[%s] closed", name)
		}
		delete(k.producers, name)
	}
	for name, c := range k.consumers {
		if c != nil {
			c.Close()
			log.Infof("Kafka consumer[%s] closed", name)
		}
		delete(k.consumers, name)
	}

	return nil
}

// GetMetrics gets monitoring metrics
func (k *Client) GetMetrics() *Metrics {
	return k.metrics
}
