package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/twmb/franz-go/pkg/kgo"
)

// initProducer 初始化生产者
func (k *Client) initProducer() error {
	if k.producer != nil {
		return nil
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ProducerLinger(time.Duration(0)),
		kgo.RetryBackoffFn(func(attempts int) time.Duration {
			return time.Duration(attempts) * time.Second
		}),
		kgo.DialTimeout(k.conf.DialTimeout.AsDuration()),
	}

	if k.conf.Tls {
		opts = append(opts, kgo.DialTLS())
	}

	if saslMech := k.getSASLMechanism(); saslMech != nil {
		opts = append(opts, kgo.SASL(saslMech))
	}

	if comp := k.getCompression(); comp != kgo.NoCompression() {
		opts = append(opts, kgo.ProducerBatchCompression(comp))
	}

	producer, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	k.producer = producer
	return nil
}

// Produce 发送消息到指定主题
func (k *Client) Produce(ctx context.Context, topic string, key, value []byte) error {
	// 验证参数
	if err := k.validateTopic(topic); err != nil {
		return fmt.Errorf("invalid topic %s: %w", topic, err)
	}

	k.mu.RLock()
	producer := k.producer
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

	// 更新指标
	k.metrics.IncrementProducedMessages(1)
	k.metrics.IncrementProducedBytes(int64(len(value)))
	k.metrics.SetProducerLatency(time.Since(start))

	return nil
}

// ProduceBatch 批量发送消息
func (k *Client) ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error {
	k.mu.RLock()
	producer := k.producer
	k.mu.RUnlock()

	if producer == nil {
		return ErrProducerNotInitialized
	}

	start := time.Now()
	err := k.retryHandler.DoWithRetry(ctx, func() error {
		return producer.ProduceSync(ctx, records...).FirstErr()
	})

	if err != nil {
		k.metrics.IncrementProducerErrors()
		log.ErrorfCtx(ctx, "Failed to produce batch messages to topic %s: %v", topic, err)
		return fmt.Errorf("failed to produce batch messages: %w", err)
	}

	// 更新指标
	totalBytes := int64(0)
	for _, record := range records {
		totalBytes += int64(len(record.Value))
	}
	k.metrics.IncrementProducedMessages(int64(len(records)))
	k.metrics.IncrementProducedBytes(totalBytes)
	k.metrics.ProducerLatency = time.Since(start)

	return nil
}

// getCompression 获取压缩算法
func (k *Client) getCompression() kgo.CompressionCodec {
	switch k.conf.Producer.Compression {
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
		return kgo.SnappyCompression() // 默认使用 snappy 压缩
	}
}

// GetProducer 获取生产者客户端（用于高级操作）
func (k *Client) GetProducer() *kgo.Client {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.producer
}

// IsProducerReady 检查生产者是否就绪
func (k *Client) IsProducerReady() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.producer != nil
}
