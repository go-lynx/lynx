package kafka

import (
	"context"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer Kafka 生产者接口
type Producer interface {
	// Produce 发送单条消息到指定主题
	Produce(ctx context.Context, topic string, key, value []byte) error

	// ProduceBatch 批量发送消息到指定主题
	ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error

	// ProduceWith 按生产者实例名发送单条消息
	ProduceWith(ctx context.Context, producerName, topic string, key, value []byte) error

	// ProduceBatchWith 按生产者实例名批量发送
	ProduceBatchWith(ctx context.Context, producerName string, topic string, records []*kgo.Record) error

	// GetProducer 获取底层生产者客户端
	GetProducer() *kgo.Client

	// IsProducerReady 检查生产者是否就绪
	IsProducerReady() bool
}

// Consumer Kafka 消费者接口
type Consumer interface {
	// Subscribe 订阅主题并设置消息处理器
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error

	// SubscribeWith 按消费者实例名订阅
	SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error

	// GetConsumer 获取底层消费者客户端
	GetConsumer() *kgo.Client

	// IsConsumerReady 检查消费者是否就绪
	IsConsumerReady() bool
}

// ClientInterface Kafka 客户端接口
type ClientInterface interface {
	Producer
	Consumer

	// InitializeResources 初始化资源
	InitializeResources(rt plugins.Runtime) error

	// StartupTasks 启动任务
	StartupTasks() error

	// ShutdownTasks 关闭任务
	ShutdownTasks() error

	// GetMetrics 获取监控指标
	GetMetrics() *Metrics
}

// MetricsProvider 监控指标提供者接口
type MetricsProvider interface {
	// GetStats 获取统计信息
	GetStats() map[string]interface{}

	// Reset 重置指标
	Reset()
}

// HealthCheckerInterface 健康检查器接口
type HealthCheckerInterface interface {
	// Start 启动健康检查
	Start()

	// Stop 停止健康检查
	Stop()

	// IsHealthy 检查是否健康
	IsHealthy() bool

	// GetLastCheck 获取最后检查时间
	GetLastCheck() time.Time

	// GetErrorCount 获取错误计数
	GetErrorCount() int
}

// ConnectionManagerInterface 连接管理器接口
type ConnectionManagerInterface interface {
	// Start 启动连接管理器
	Start()

	// Stop 停止连接管理器
	Stop()

	// IsConnected 检查是否已连接
	IsConnected() bool

	// GetHealthChecker 获取健康检查器
	GetHealthChecker() HealthCheckerInterface

	// ForceReconnect 强制重连
	ForceReconnect()
}

// BatchProcessorInterface 批量处理器接口
type BatchProcessorInterface interface {
	// AddRecord 添加记录
	AddRecord(ctx context.Context, record *kgo.Record) error

	// Flush 强制处理
	Flush(ctx context.Context) error

	// Close 关闭处理器
	Close()
}

// RetryHandlerInterface 重试处理器接口
type RetryHandlerInterface interface {
	// DoWithRetry 执行带重试的操作
	DoWithRetry(ctx context.Context, operation func() error) error
}

// GoroutinePoolInterface 协程池接口
type GoroutinePoolInterface interface {
	// Submit 提交任务
	Submit(task func())

	// Wait 等待完成
	Wait()
}
