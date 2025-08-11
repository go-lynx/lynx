package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/twmb/franz-go/pkg/kgo"
)

// initProducerInstance 初始化指定名称的生产者实例
func (k *Client) initProducerInstance(name string, p *conf.Producer) (*kgo.Client, error) {
	if p == nil {
		return nil, fmt.Errorf("producer config is nil for %s", name)
	}

	// 将 linger 与配置联动：若配置了 BatchTimeout 则用作 linger，以便更好批处理
	linger := 5 * time.Millisecond
	if d := p.BatchTimeout.AsDuration(); d > 0 {
		linger = d
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ProducerLinger(linger),
		kgo.DialTimeout(k.conf.DialTimeout.AsDuration()),
	}

	// TLS 对象配置
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

	// RequiredAcks 数值映射：-1=AllISRAcks，1=LeaderAck（默认），0=NoAck
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

// Produce 发送消息到指定主题
func (k *Client) Produce(ctx context.Context, topic string, key, value []byte) error {
	// 路由到默认生产者
	k.mu.RLock()
	name := k.defaultProducer
	k.mu.RUnlock()
	if name == "" {
		return ErrProducerNotInitialized
	}
	return k.ProduceWith(ctx, name, topic, key, value)
}

// ProduceWith 按生产者实例名发送
func (k *Client) ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error {
	// 验证参数
	if err := k.validateTopic(topic); err != nil {
		return fmt.Errorf("invalid topic %s: %w", topic, err)
	}

	// 若启用了异步批处理器，则优先入队，由后台统一批量发送与计量
	k.mu.RLock()
	bp := k.batchProcessors[producerName]
	k.mu.RUnlock()
	if bp != nil {
		record := &kgo.Record{Topic: topic, Key: key, Value: value}
		if err := bp.AddRecord(ctx, record); err != nil {
			// 入队失败则回退到同步发送路径
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

	// 更新指标（使用线程安全的封装方法）
	k.metrics.IncrementProducedMessages(1)
	k.metrics.IncrementProducedBytes(int64(len(value)))
	k.metrics.SetProducerLatency(time.Since(start))

	return nil
}

// ProduceBatch 批量发送消息
func (k *Client) ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error {
	// 路由到默认生产者
	k.mu.RLock()
	name := k.defaultProducer
	k.mu.RUnlock()
	if name == "" {
		return ErrProducerNotInitialized
	}
	return k.ProduceBatchWith(ctx, name, topic, records)
}

// ProduceBatchWith 按生产者实例名批量发送
func (k *Client) ProduceBatchWith(ctx context.Context, producerName string, topic string, records []*kgo.Record) error {
	k.mu.RLock()
	producer := k.producers[producerName]
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
		// 当传入 topic 为空时，统计本批实际涉及的 topic 以便排障
		if topic == "" {
			topicSet := make(map[string]struct{})
			for _, r := range nonNil {
				topicSet[r.Topic] = struct{}{}
			}
			// 构造稳定的 topic 列表展示（最多展示前 5 个）
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

// getCompression 获取压缩算法（基于实例配置）
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
		return kgo.SnappyCompression() // 默认使用 snappy 压缩
	}
}

// GetProducer 获取默认生产者客户端
func (k *Client) GetProducer() *kgo.Client {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.defaultProducer == "" {
		return nil
	}
	return k.producers[k.defaultProducer]
}

// IsProducerReady 检查默认生产者是否就绪
func (k *Client) IsProducerReady() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.defaultProducer == "" {
		return false
	}
	return k.producers[k.defaultProducer] != nil
}
