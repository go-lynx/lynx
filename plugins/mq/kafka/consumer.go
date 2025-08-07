package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/twmb/franz-go/pkg/kgo"
)

// MessageHandler 消息处理器接口
type MessageHandler interface {
	Handle(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error
}

// MessageHandlerFunc 消息处理器函数类型
type MessageHandlerFunc func(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error

// Handle 实现 MessageHandler 接口
func (f MessageHandlerFunc) Handle(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error {
	return f(ctx, topic, partition, offset, key, value)
}

// ConsumerGroup 消费者组
type ConsumerGroup struct {
	client        *kgo.Client
	groupID       string
	topics        []string
	handler       MessageHandler
	pool          *GoroutinePool
	metrics       *Metrics
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	isRunning     bool
	errorChan     chan error
	rebalanceChan chan struct{}
}

// NewConsumerGroup 创建新的消费者组
func (k *Client) NewConsumerGroup(groupID string, topics []string, handler MessageHandler) *ConsumerGroup {
	ctx, cancel := context.WithCancel(k.ctx)
	return &ConsumerGroup{
		client:        k.consumer,
		groupID:       groupID,
		topics:        topics,
		handler:       handler,
		pool:          NewGoroutinePool(int(k.conf.Consumer.MaxConcurrency)),
		metrics:       k.metrics,
		ctx:           ctx,
		cancel:        cancel,
		errorChan:     make(chan error, 100),
		rebalanceChan: make(chan struct{}, 10),
	}
}

// initConsumer 初始化消费者
func (k *Client) initConsumer() error {
	if k.consumer != nil {
		return nil
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ConsumerGroup(k.conf.Consumer.GroupId),
		kgo.ConsumeResetOffset(k.getStartOffset()),
		kgo.DialTimeout(k.conf.DialTimeout.AsDuration()),
		kgo.RebalanceTimeout(k.conf.Consumer.RebalanceTimeout.AsDuration()),
		kgo.AutoCommitInterval(k.conf.Consumer.AutoCommitInterval.AsDuration()),
		kgo.AutoCommitMarks(),
	}

	if k.conf.Tls {
		opts = append(opts, kgo.DialTLS())
	}

	if saslMech := k.getSASLMechanism(); saslMech != nil {
		opts = append(opts, kgo.SASL(saslMech))
	}

	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	k.consumer = consumer
	return nil
}

// getStartOffset 获取起始偏移量
func (k *Client) getStartOffset() kgo.Offset {
	switch k.conf.Consumer.StartOffset {
	case StartOffsetEarliest:
		return kgo.NewOffset().AtStart()
	case StartOffsetLatest:
		return kgo.NewOffset().AtEnd()
	default:
		return kgo.NewOffset().AtEnd()
	}
}

// Subscribe 订阅主题
func (k *Client) Subscribe(ctx context.Context, topics []string, handler MessageHandler) error {
	if k.consumer == nil {
		if err := k.initConsumer(); err != nil {
			return fmt.Errorf("failed to initialize consumer: %w", err)
		}
	}

	if len(topics) == 0 {
		return ErrNoTopicsSpecified
	}

	consumerGroup := k.NewConsumerGroup(k.conf.Consumer.GroupId, topics, handler)

	// 启动消费者组
	go func() {
		if err := consumerGroup.Start(); err != nil {
			log.ErrorfCtx(ctx, "Consumer group failed: %v", err)
		}
	}()

	return nil
}

// Start 启动消费者组
func (cg *ConsumerGroup) Start() error {
	cg.mu.Lock()
	if cg.isRunning {
		cg.mu.Unlock()
		return fmt.Errorf("consumer group is already running")
	}
	cg.isRunning = true
	cg.mu.Unlock()

	defer func() {
		cg.mu.Lock()
		cg.isRunning = false
		cg.mu.Unlock()
	}()

	// 启动错误处理协程
	go cg.handleErrors()

	// 启动重平衡处理协程
	go cg.handleRebalances()

	// 开始消费
	for {
		select {
		case <-cg.ctx.Done():
			return cg.ctx.Err()
		default:
		}

		fetches := cg.client.PollFetches(cg.ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				cg.metrics.IncrementConsumerErrors()
				log.ErrorfCtx(cg.ctx, "Consumer error: %v", err)
			}
			continue
		}

		// 处理消息
		cg.processFetches(fetches)
	}
}

// processFetches 处理获取的消息
func (cg *ConsumerGroup) processFetches(fetches kgo.Fetches) {
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				if len(partition.Records) == 0 {
					continue
				}

				// 批量处理记录
				cg.processRecords(topic.Topic, partition.Partition, partition.Records)
			}
		}
	}
}

// processRecords 处理记录
func (cg *ConsumerGroup) processRecords(topic string, partition int32, records []*kgo.Record) {
	// 使用 goroutine 池处理消息
	cg.pool.Submit(func() {
		for _, record := range records {
			start := time.Now()

			// 创建带超时的上下文
			ctx, cancel := context.WithTimeout(cg.ctx, 30*time.Second)

			err := cg.handler.Handle(ctx, topic, partition, record.Offset, record.Key, record.Value)
			cancel()

			if err != nil {
				cg.metrics.IncrementConsumerErrors()
				log.ErrorfCtx(cg.ctx, "Failed to process message from topic %s partition %d: %v", topic, partition, err)
				// 发送错误到错误通道
				select {
				case cg.errorChan <- err:
				default:
					log.ErrorfCtx(cg.ctx, "Error channel is full, dropping error: %v", err)
				}
			} else {
				// 更新指标
				cg.metrics.IncrementConsumedMessages(1)
				cg.metrics.IncrementConsumedBytes(int64(len(record.Value)))
				cg.metrics.ConsumerLatency = time.Since(start)
			}
		}
	})
}

// handleErrors 处理错误
func (cg *ConsumerGroup) handleErrors() {
	for {
		select {
		case <-cg.ctx.Done():
			return
		case err := <-cg.errorChan:
			log.ErrorfCtx(cg.ctx, "Consumer error: %v", err)
			// 这里可以添加错误处理逻辑，比如发送到死信队列
		}
	}
}

// handleRebalances 处理重平衡
func (cg *ConsumerGroup) handleRebalances() {
	for {
		select {
		case <-cg.ctx.Done():
			return
		case <-cg.rebalanceChan:
			log.InfofCtx(cg.ctx, "Consumer group rebalancing...")
			// 这里可以添加重平衡处理逻辑
		}
	}
}

// Stop 停止消费者组
func (cg *ConsumerGroup) Stop() {
	cg.cancel()
	cg.pool.Wait()
}

// IsRunning 检查是否正在运行
func (cg *ConsumerGroup) IsRunning() bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.isRunning
}

// GetConsumer 获取消费者客户端（用于高级操作）
func (k *Client) GetConsumer() *kgo.Client {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.consumer
}

// IsConsumerReady 检查消费者是否就绪
func (k *Client) IsConsumerReady() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.consumer != nil
}
