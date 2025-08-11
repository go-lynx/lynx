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

// Client Kafka 客户端插件
type Client struct {
	*plugins.BasePlugin
	conf *conf.Kafka
	// 多实例生产者/消费者
	producers        map[string]*kgo.Client
	batchProcessors  map[string]*BatchProcessor
	defaultProducer  string
	consumers        map[string]*kgo.Client
	activeGroups     map[string]*ConsumerGroup // 按消费者实例名维护活跃组
	// 连接管理器
	prodConnMgrs     map[string]*ConnectionManager
	consConnMgrs     map[string]*ConnectionManager
	// 兼容旧字段，待 consumer 重构完成后移除
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

// 确保 Client 实现了所有接口
var _ ClientInterface = (*Client)(nil)
var _ Producer = (*Client)(nil)
var _ Consumer = (*Client)(nil)

// NewKafkaClient 创建一个新的 Kafka 客户端插件实例
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
		ctx:          ctx,
		cancel:       cancel,
		metrics:      NewMetrics(),
		retryHandler: NewRetryHandler(RetryConfig{MaxRetries: 3, BackoffTime: time.Second, MaxBackoff: 30 * time.Second}),
		producers:       make(map[string]*kgo.Client),
		batchProcessors: make(map[string]*BatchProcessor),
		consumers:       make(map[string]*kgo.Client),
		activeGroups:    make(map[string]*ConsumerGroup),
		prodConnMgrs:    make(map[string]*ConnectionManager),
		consConnMgrs:    make(map[string]*ConnectionManager),
	}
}

// InitializeResources 初始化 Kafka 资源
func (k *Client) InitializeResources(rt plugins.Runtime) error {
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
func (k *Client) StartupTasks() error {
	// 初始化所有启用的生产者实例
	var firstProducerName string
	for _, p := range k.conf.Producers {
		if p == nil || !p.Enabled {
			continue
		}
		name := p.Name
		if name == "" {
			// 若未命名，用 topic 组合或序号；这里简单使用 first-available 自增名
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
		// 启动生产者连接管理器
		if _, ok := k.prodConnMgrs[name]; !ok {
			cm := NewConnectionManager(client, k.conf.GetBrokers())
			k.prodConnMgrs[name] = cm
			cm.Start()
			log.Infof("Kafka producer[%s] connection manager started", name)
		}
		if firstProducerName == "" {
			firstProducerName = name
		}
		// 每个生产者按配置决定是否启用异步批处理
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

	// 消费者在 Subscribe/SubscribeWith 时初始化
	return nil
}

// ShutdownTasks 关闭任务
func (k *Client) ShutdownTasks() error {
	k.cancel() // 取消所有上下文

	k.mu.Lock()
	defer k.mu.Unlock()

	// 停止连接管理器（先于关闭客户端）
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

	// 先优雅刷盘所有批处理器
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

// GetMetrics 获取监控指标
func (k *Client) GetMetrics() *Metrics {
	return k.metrics
}
