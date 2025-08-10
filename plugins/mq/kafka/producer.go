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
		// 默认采用较小的 linger，允许 franz-go 进行轻量批处理；重试策略统一交给外层 RetryHandler
		kgo.ProducerLinger(5 * time.Millisecond),
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

	// 映射 RequiredAcks：true 等待所有 ISR；false 仅等待 leader
	if k.conf.Producer != nil {
		if k.conf.Producer.RequiredAcks {
			opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
		} else {
			opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
		}
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

	// 更新指标（使用线程安全的封装方法）
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

	// 规范 topic 语义：若入参 topic 非空，则将所有 record 的 Topic 统一设置为该 topic；
	// 若入参 topic 为空，则要求每条 record 自带合法 Topic。
	if topic != "" {
		if err := k.validateTopic(topic); err != nil {
			return fmt.Errorf("invalid topic %s: %w", topic, err)
		}
	}

	// 过滤 nil 记录，避免后续 ProduceSync/统计时出现空指针
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

	// 若全部为 nil，直接返回
	if len(nonNil) == 0 {
		return nil
	}

	start := time.Now()
	err := k.retryHandler.DoWithRetry(ctx, func() error {
		return producer.ProduceSync(ctx, nonNil...).FirstErr()
	})

	if err != nil {
		k.metrics.IncrementProducerErrors()
		log.ErrorfCtx(ctx, "Failed to produce batch messages to topic %s: %v", topic, err)
		return fmt.Errorf("failed to produce batch messages: %w", err)
	}

	// 更新指标（基于过滤后的数组）
	totalBytes := int64(0)
	for _, record := range nonNil {
		totalBytes += int64(len(record.Value))
	}
	k.metrics.IncrementProducedMessages(int64(len(nonNil)))
	k.metrics.IncrementProducedBytes(totalBytes)
	k.metrics.SetProducerLatency(time.Since(start))

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
