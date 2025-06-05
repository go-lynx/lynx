package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugins/mq/kafka/v2/conf"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"google.golang.org/protobuf/types/known/durationpb"
)

// 插件元数据
const (
	pluginName        = "kafka.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "kafka client plugin for Lynx framework"
	confPrefix        = "lynx.kafka"
)

// KafkaClient Kafka 客户端插件
type KafkaClient struct {
	*plugins.BasePlugin
	conf     *conf.Kafka
	producer *kgo.Client
	consumer *kgo.Client
	mu       sync.RWMutex
}

// NewKafkaClient 创建一个新的 Kafka 客户端插件实例
func NewKafkaClient() *KafkaClient {
	return &KafkaClient{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
			confPrefix,
			100,
		),
	}
}

// InitializeResources 初始化 Kafka 资源
func (k *KafkaClient) InitializeResources(rt plugins.Runtime) error {
	k.conf = &conf.Kafka{}

	// 加载配置
	err := rt.GetConfig().Value(confPrefix).Scan(k.conf)
	if err != nil {
		return err
	}

	// 设置默认值
	defaultConf := &conf.Kafka{
		DialTimeout: &durationpb.Duration{Seconds: 10},
		Producer: &conf.Producer{
			MaxRetries:   3,
			RetryBackoff: &durationpb.Duration{Seconds: 1},
			BatchSize:    1000,
			BatchTimeout: &durationpb.Duration{Seconds: 1},
			Compression:  "snappy",
			RequiredAcks: true,
		},
		Consumer: &conf.Consumer{
			AutoCommitInterval: &durationpb.Duration{Seconds: 5},
			AutoCommit:         true,
			StartOffset:        "latest",
			MaxConcurrency:     10,
			MinBatchSize:       1,
			MaxBatchSize:       1000,
			MaxWaitTime:        &durationpb.Duration{Seconds: 1},
			RebalanceTimeout:   &durationpb.Duration{Seconds: 30},
		},
	}

	// 应用默认值
	if k.conf.DialTimeout == nil {
		k.conf.DialTimeout = defaultConf.DialTimeout
	}
	if k.conf.Producer != nil {
		if k.conf.Producer.MaxRetries == 0 {
			k.conf.Producer.MaxRetries = defaultConf.Producer.MaxRetries
		}
		if k.conf.Producer.RetryBackoff == nil {
			k.conf.Producer.RetryBackoff = defaultConf.Producer.RetryBackoff
		}
		if k.conf.Producer.BatchSize == 0 {
			k.conf.Producer.BatchSize = defaultConf.Producer.BatchSize
		}
		if k.conf.Producer.BatchTimeout == nil {
			k.conf.Producer.BatchTimeout = defaultConf.Producer.BatchTimeout
		}
		if k.conf.Producer.Compression == "" {
			k.conf.Producer.Compression = defaultConf.Producer.Compression
		}
	}
	if k.conf.Consumer != nil {
		if k.conf.Consumer.AutoCommitInterval == nil {
			k.conf.Consumer.AutoCommitInterval = defaultConf.Consumer.AutoCommitInterval
		}
		if k.conf.Consumer.StartOffset == "" {
			k.conf.Consumer.StartOffset = defaultConf.Consumer.StartOffset
		}
		if k.conf.Consumer.MaxConcurrency == 0 {
			k.conf.Consumer.MaxConcurrency = defaultConf.Consumer.MaxConcurrency
		}
		if k.conf.Consumer.MinBatchSize == 0 {
			k.conf.Consumer.MinBatchSize = defaultConf.Consumer.MinBatchSize
		}
		if k.conf.Consumer.MaxBatchSize == 0 {
			k.conf.Consumer.MaxBatchSize = defaultConf.Consumer.MaxBatchSize
		}
		if k.conf.Consumer.MaxWaitTime == nil {
			k.conf.Consumer.MaxWaitTime = defaultConf.Consumer.MaxWaitTime
		}
		if k.conf.Consumer.RebalanceTimeout == nil {
			k.conf.Consumer.RebalanceTimeout = defaultConf.Consumer.RebalanceTimeout
		}
	}

	return nil
}

// StartupTasks 启动任务
func (k *KafkaClient) StartupTasks() error {
	// 验证配置
	if len(k.conf.Brokers) == 0 {
		return errors.New("kafka brokers not configured")
	}

	// 初始化生产者
	if k.conf.Producer != nil && k.conf.Producer.Enabled {
		if err := k.initProducer(); err != nil {
			return fmt.Errorf("failed to initialize kafka producer: %w", err)
		}
	}

	// 消费者在 Subscribe 时初始化
	return nil
}

// ShutdownTasks 关闭任务
func (k *KafkaClient) ShutdownTasks() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.producer != nil {
		k.producer.Close()
	}
	if k.consumer != nil {
		k.consumer.Close()
	}

	return nil
}

// initProducer 初始化生产者
func (k *KafkaClient) initProducer() error {
	if k.producer != nil {
		return nil
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ProducerLinger(time.Duration(0)),
		kgo.RetryBackoffFn(func(attempts int) time.Duration {
			return time.Duration(attempts) * time.Second
		}),
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
		return err
	}

	k.producer = producer
	return nil
}

// initConsumer 初始化消费者
func (k *KafkaClient) initConsumer(topics ...string) error {
	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ConsumerGroup(k.conf.Consumer.GroupId),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(k.getStartOffset()),
		kgo.RebalanceTimeout(k.conf.Consumer.RebalanceTimeout.AsDuration()),
	}

	if k.conf.Consumer.AutoCommit {
		opts = append(opts, kgo.AutoCommitInterval(k.conf.Consumer.AutoCommitInterval.AsDuration()))
	} else {
		opts = append(opts, kgo.DisableAutoCommit())
	}

	// 配置 SASL
	if k.conf.Sasl != nil && k.conf.Sasl.Enabled {
		opts = append(opts, kgo.SASL(k.getSASLMechanism()))
	}

	// 配置 TLS
	if k.conf.Tls {
		opts = append(opts, kgo.DialTLS())
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}

	k.mu.Lock()
	k.consumer = client
	k.mu.Unlock()

	return nil
}

// Produce 发送消息到指定主题
func (k *KafkaClient) Produce(ctx context.Context, topic string, key, value []byte) error {
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

	return producer.ProduceSync(ctx, record).FirstErr()
}

// Subscribe 订阅主题并消费消息
func (k *KafkaClient) Subscribe(ctx context.Context, topics []string, handler func(context.Context, *kgo.Record) error) error {
	if len(topics) == 0 {
		return errors.New("no topics specified")
	}

	if k.conf.Consumer == nil || !k.conf.Consumer.Enabled {
		return errors.New("consumer is not enabled in configuration")
	}

	if k.conf.Consumer.GroupId == "" {
		return errors.New("consumer group ID is required")
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	// 如果已经有消费者在运行，先关闭它
	if k.consumer != nil {
		k.consumer.Close()
		k.consumer = nil
	}

	// 初始化新的消费者
	if err := k.initConsumer(topics...); err != nil {
		return fmt.Errorf("failed to initialize consumer: %w", err)
	}

	// 启动消费循环
	go func() {
		for {
			select {
			case <-ctx.Done():
				k.mu.Lock()
				if k.consumer != nil {
					k.consumer.Close()
					k.consumer = nil
				}
				k.mu.Unlock()
				return
			default:
				k.mu.RLock()
				consumer := k.consumer
				k.mu.RUnlock()

				if consumer == nil {
					return
				}

				fetches := consumer.PollFetches(ctx)
				if errs := fetches.Errors(); len(errs) > 0 {
					// TODO: 考虑重试或通知错误处理器
					continue
				}

				// 批量处理消息
				iter := fetches.RecordIter()
				for !iter.Done() {
					record := iter.Next()
					if err := handler(ctx, record); err != nil {
						// TODO: 实现错误处理策略，可能的选项：
						// 1. 记录错误日志
						// 2. 发送到死信队列
						// 3. 重试处理
						log.ErrorfCtx(ctx, "Failed to process message: %v", err)
					}
				}

				// 手动提交 offset
				if !k.conf.Consumer.AutoCommit {
					if err := consumer.CommitUncommittedOffsets(ctx); err != nil {
						log.ErrorfCtx(ctx, "Failed to commit offsets: %v", err)
					}
				}
			}
		}
	}()

	return nil
}

// getCompression 获取压缩算法
func (k *KafkaClient) getCompression() kgo.CompressionCodec {
	switch k.conf.Producer.Compression {
	case "gzip":
		return kgo.GzipCompression()
	case "snappy":
		return kgo.SnappyCompression()
	case "lz4":
		return kgo.Lz4Compression()
	case "zstd":
		return kgo.ZstdCompression()
	default:
		return kgo.GzipCompression() // 默认使用 gzip 压缩
	}
}

// getSASLMechanism 获取 SASL 认证机制
func (k *KafkaClient) getSASLMechanism() sasl.Mechanism {
	if k.conf.Sasl == nil || !k.conf.Sasl.Enabled {
		return nil
	}

	switch k.conf.Sasl.Mechanism {
	case "PLAIN":
		return plain.Plain(func(ctx context.Context) (plain.Auth, error) {
			return plain.Auth{
				User: k.conf.Sasl.Username,
				Pass: k.conf.Sasl.Password,
			}, nil
		})
	case "SCRAM-SHA-256":
		return scram.Sha256(func(ctx context.Context) (scram.Auth, error) {
			return scram.Auth{
				User: k.conf.Sasl.Username,
				Pass: k.conf.Sasl.Password,
			}, nil
		})
	case "SCRAM-SHA-512":
		return scram.Sha512(func(ctx context.Context) (scram.Auth, error) {
			return scram.Auth{
				User: k.conf.Sasl.Username,
				Pass: k.conf.Sasl.Password,
			}, nil
		})
	default:
		return nil
	}
}

// getStartOffset 获取消费起始位置
func (k *KafkaClient) getStartOffset() kgo.Offset {
	switch k.conf.Consumer.StartOffset {
	case "earliest":
		return kgo.NewOffset().AtStart()
	default:
		return kgo.NewOffset().AtEnd()
	}
}
