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

// KafkaClient Kafka 客户端插件
type KafkaClient struct {
	*plugins.BasePlugin
	conf           *conf.Kafka
	producer       *kgo.Client
	consumer       *kgo.Client
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	metrics        *Metrics
	batchProcessor *BatchProcessor
	retryHandler   *RetryHandler
}

// 确保 KafkaClient 实现了所有接口
var _ KafkaClientInterface = (*KafkaClient)(nil)
var _ KafkaProducer = (*KafkaClient)(nil)
var _ KafkaConsumer = (*KafkaClient)(nil)

// NewKafkaClient 创建一个新的 Kafka 客户端插件实例
func NewKafkaClient() *KafkaClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &KafkaClient{
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
	}
}

// InitializeResources 初始化 Kafka 资源
func (k *KafkaClient) InitializeResources(rt plugins.Runtime) error {
	k.conf = &conf.Kafka{}

	// 加载配置
	err := rt.GetConfig().Value(confPrefix).Scan(k.conf)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	// 验证配置
	if err := k.validateConfiguration(); err != nil {
		return err
	}

	// 设置默认值
	k.setDefaultValues()

	return nil
}

// StartupTasks 启动任务
func (k *KafkaClient) StartupTasks() error {
	// 初始化生产者
	if k.conf.Producer != nil && k.conf.Producer.Enabled {
		if err := k.initProducer(); err != nil {
			return fmt.Errorf("failed to initialize kafka producer: %w", err)
		}
		log.Infof("Kafka producer initialized successfully")
	}

	// 消费者在 Subscribe 时初始化
	return nil
}

// ShutdownTasks 关闭任务
func (k *KafkaClient) ShutdownTasks() error {
	k.cancel() // 取消所有上下文

	k.mu.Lock()
	defer k.mu.Unlock()

	if k.producer != nil {
		k.producer.Close()
		log.Infof("Kafka producer closed")
	}
	if k.consumer != nil {
		k.consumer.Close()
		log.Infof("Kafka consumer closed")
	}

	return nil
}

// GetMetrics 获取监控指标
func (k *KafkaClient) GetMetrics() *Metrics {
	return k.metrics
}
