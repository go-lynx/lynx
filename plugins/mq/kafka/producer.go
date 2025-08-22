package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/twmb/franz-go/pkg/kgo"
)

// initProducerInstance initializes a producer instance with the specified name
func (k *Client) initProducerInstance(name string, p *conf.Producer) (*kgo.Client, error) {
	if p == nil {
		return nil, fmt.Errorf("producer config is nil for %s", name)
	}

	// Link linger with configuration: if BatchTimeout is configured, use it as linger for better batching
	linger := 5 * time.Millisecond
	if d := p.BatchTimeout.AsDuration(); d > 0 {
		linger = d
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ProducerLinger(linger),
		kgo.DialTimeout(k.conf.DialTimeout.AsDuration()),
	}

    // TLS configuration
    if k.conf.Tls != nil && k.conf.Tls.Enabled {
        tlsCfg, err := buildTLSConfig(k.conf.Tls)
        if err != nil {
            return nil, fmt.Errorf("buildTLSConfig failed: %w", err)
        }
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	if saslMech := k.getSASLMechanism(); saslMech != nil {
		opts = append(opts, kgo.SASL(saslMech))
	}

	if comp := k.getCompression(p); comp != kgo.NoCompression() {
		opts = append(opts, kgo.ProducerBatchCompression(comp))
	}

	// RequiredAcks value mapping: -1=AllISRAcks, 1=LeaderAck (default), 0=NoAck
	switch p.RequiredAcks {
	case -1:
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	case 0:
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()))
	case 1:
		fallthrough
	default:
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
	}

	producer, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	return producer, nil
}

// Produce sends a message to the specified topic
func (k *Client) Produce(ctx context.Context, topic string, key, value []byte) error {
	// Route to default producer
	k.mu.RLock()
	name := k.defaultProducer
	k.mu.RUnlock()
	if name == "" {
		return ErrProducerNotInitialized
	}
	return k.ProduceWith(ctx, name, topic, key, value)
}

// ProduceWith sends by producer instance name
func (k *Client) ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error {
	// Validate parameters
	if err := k.validateTopic(topic); err != nil {
		return fmt.Errorf("invalid topic %s: %w", topic, err)
	}

	// If async batch processor is enabled, prioritize enqueueing, with background unified batch sending and metrics
	k.mu.RLock()
	bp := k.batchProcessors[producerName]
	k.mu.RUnlock()
	if bp != nil {
		record := &kgo.Record{Topic: topic, Key: key, Value: value}
		if err := bp.AddRecord(ctx, record); err != nil {
			// If enqueue fails, fallback to sync sending path
			log.WarnfCtx(ctx, "Batch enqueue failed, fallback to sync produce: %v", err)
		} else {
			return nil
		}
	}

	k.mu.RLock()
	producer := k.producers[producerName]
	k.mu.RUnlock()

	if producer == nil {
		return ErrProducerNotInitialized
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}

	start := time.Now()
	err := k.retryHandler.DoWithRetry(ctx, func() error {
		return producer.ProduceSync(ctx, record).FirstErr()
	})

	if err != nil {
		k.metrics.IncrementProducerErrors()
		log.ErrorfCtx(ctx, "Failed to produce message to topic %s: %v", topic, err)
		return fmt.Errorf("failed to produce message: %w", err)
	}

	// Update metrics (using thread-safe wrapper methods)
	k.metrics.IncrementProducedMessages(1)
	k.metrics.IncrementProducedBytes(int64(len(value)))
	k.metrics.SetProducerLatency(time.Since(start))

	return nil
}

// ProduceBatch sends messages in batch
func (k *Client) ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error {
	// Route to default producer
	k.mu.RLock()
	name := k.defaultProducer
	k.mu.RUnlock()
	if name == "" {
		return ErrProducerNotInitialized
	}
	return k.ProduceBatchWith(ctx, name, topic, records)
}

// ProduceBatchWith sends batch by producer instance name
func (k *Client) ProduceBatchWith(ctx context.Context, producerName string, topic string, records []*kgo.Record) error {
	k.mu.RLock()
	producer := k.producers[producerName]
	k.mu.RUnlock()

	if producer == nil {
		return ErrProducerNotInitialized
	}

	// Standardize topic semantics: if input topic is not empty, set all records' Topic to this topic;
	// if input topic is empty, require each record to have its own valid Topic.
	if topic != "" {
		if err := k.validateTopic(topic); err != nil {
			return fmt.Errorf("invalid topic %s: %w", topic, err)
		}
	}

	// Filter nil records to avoid null pointer in subsequent ProduceSync/statistics
	nonNil := make([]*kgo.Record, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		if topic != "" {
			r.Topic = topic
		} else {
			if err := k.validateTopic(r.Topic); err != nil {
				return fmt.Errorf("invalid topic %s: %w", r.Topic, err)
			}
		}
		nonNil = append(nonNil, r)
	}

	// If all are nil, return directly
	if len(nonNil) == 0 {
		return nil
	}

	start := time.Now()
	err := k.retryHandler.DoWithRetry(ctx, func() error {
		return producer.ProduceSync(ctx, nonNil...).FirstErr()
	})

	if err != nil {
		k.metrics.IncrementProducerErrors()
		// When input topic is empty, count the actual topics involved in this batch for troubleshooting
		if topic == "" {
			topicSet := make(map[string]struct{})
			for _, r := range nonNil {
				topicSet[r.Topic] = struct{}{}
			}
			// Construct stable topic list display (show at most first 5)
			topics := make([]string, 0, len(topicSet))
			for tp := range topicSet {
				topics = append(topics, tp)
			}
			if len(topics) > 5 {
				topics = topics[:5]
				topics = append(topics, "...")
			}
			log.ErrorfCtx(ctx, "Failed to produce batch messages to topics %v: %v", topics, err)
		} else {
			log.ErrorfCtx(ctx, "Failed to produce batch messages to topic %s: %v", topic, err)
		}
		return fmt.Errorf("failed to produce batch messages: %w", err)
	}

	// Update metrics (based on filtered array)
	totalBytes := int64(0)
	for _, record := range nonNil {
		totalBytes += int64(len(record.Value))
	}
	k.metrics.IncrementProducedMessages(int64(len(nonNil)))
	k.metrics.IncrementProducedBytes(totalBytes)
	k.metrics.SetProducerLatency(time.Since(start))

	return nil
}

// getCompression gets compression algorithm (based on instance configuration)
func (k *Client) getCompression(p *conf.Producer) kgo.CompressionCodec {
	if p == nil {
		return kgo.SnappyCompression()
	}
	switch p.Compression {
	case CompressionGzip:
		return kgo.GzipCompression()
	case CompressionSnappy:
		return kgo.SnappyCompression()
	case CompressionLz4:
		return kgo.Lz4Compression()
	case CompressionZstd:
		return kgo.ZstdCompression()
	case CompressionNone:
		return kgo.NoCompression()
	default:
		return kgo.SnappyCompression() // Default to snappy compression
	}
}

// GetProducer gets the default producer client
func (k *Client) GetProducer() *kgo.Client {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.defaultProducer == "" {
		return nil
	}
	return k.producers[k.defaultProducer]
}

// IsProducerReady checks if the default producer is ready
func (k *Client) IsProducerReady() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.defaultProducer == "" {
		return false
	}
	return k.producers[k.defaultProducer] != nil
}
