package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/twmb/franz-go/pkg/kerr"
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
	rebalanceChan chan RebalanceEvent
	autoCommit    bool
	// per-partition 串行处理通道，确保同一分区严格顺序
	partChans map[string]chan []*kgo.Record
	partMu    sync.Mutex
}

// RebalanceEvent 重平衡事件
type RebalanceEvent struct {
	Assigned map[string][]int32
	Revoked  map[string][]int32
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
		rebalanceChan: make(chan RebalanceEvent, 16),
		autoCommit:    k.conf.Consumer.AutoCommit,
		partChans:     make(map[string]chan []*kgo.Record),
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
	}

	// 根据配置启用自动提交相关选项；手动提交模式下不启用 AutoCommitMarks
	if k.conf.Consumer.AutoCommit {
		opts = append(opts,
			kgo.AutoCommitInterval(k.conf.Consumer.AutoCommitInterval.AsDuration()),
			kgo.AutoCommitMarks(),
		)
	}

	// 基础的重平衡回调：仅发送事件到当前活跃的 ConsumerGroup，由 handleRebalances 统一处理
	opts = append(opts,
		kgo.OnPartitionsAssigned(func(ctx context.Context, c *kgo.Client, assigned map[string][]int32) {
			var cg *ConsumerGroup
			if k != nil {
				k.mu.RLock()
				cg = k.activeConsumerGroup
				k.mu.RUnlock()
			}
			if cg != nil {
				select {
				case <-cg.ctx.Done():
					// 组已关闭，忽略
				case cg.rebalanceChan <- RebalanceEvent{Assigned: assigned}:
				default:
					log.WarnfCtx(ctx, "rebalance channel full, dropping assigned event")
				}
			} else {
				log.InfofCtx(ctx, "Partitions assigned: %+v", assigned)
			}
		}),
		kgo.OnPartitionsRevoked(func(ctx context.Context, c *kgo.Client, revoked map[string][]int32) {
			var cg *ConsumerGroup
			if k != nil {
				k.mu.RLock()
				cg = k.activeConsumerGroup
				k.mu.RUnlock()
			}
			if cg != nil {
				select {
				case <-cg.ctx.Done():
					// 组已关闭，忽略
				case cg.rebalanceChan <- RebalanceEvent{Revoked: revoked}:
				default:
					log.WarnfCtx(ctx, "rebalance channel full, dropping revoked event")
				}
			} else {
				log.InfofCtx(ctx, "Partitions revoked: %+v", revoked)
			}
		}),
	)

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

	// 单组约束：若已有活跃消费者组且尚未结束，则拒绝新的订阅
	k.mu.RLock()
	if existing := k.activeConsumerGroup; existing != nil {
		select {
		case <-existing.ctx.Done():
			// 旧组已结束，允许继续
		default:
			k.mu.RUnlock()
			return fmt.Errorf("a consumer group is already active; stop it before subscribing a new one")
		}
	}
	k.mu.RUnlock()

	if len(topics) == 0 {
		return ErrNoTopicsSpecified
	}

	consumerGroup := k.NewConsumerGroup(k.conf.Consumer.GroupId, topics, handler)
	// 设置当前活跃的 ConsumerGroup，用于 rebalance 回调路由（Plan B）
	k.mu.Lock()
	k.activeConsumerGroup = consumerGroup
	k.mu.Unlock()

	// 启动消费者组
	go func() {
		if err := consumerGroup.Start(); err != nil {
			if err == context.Canceled {
				log.InfofCtx(ctx, "Consumer group stopped: %v", err)
			} else {
				log.ErrorfCtx(ctx, "Consumer group failed: %v", err)
			}
		}
		// 清理活跃组引用（仅当仍指向本组）
		k.mu.Lock()
		if k.activeConsumerGroup == consumerGroup {
			k.activeConsumerGroup = nil
		}
		k.mu.Unlock()
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
			// 正常关停不视为错误，减少日志噪声
			return nil
		default:
		}

		fetches := cg.client.PollFetches(cg.ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				cg.metrics.IncrementConsumerErrors()
				e := fe.Err
				// 分类：上下文取消/超时视为可预期；Kafka 可重试错误为 Warn；不可重试为 Error，并上报错误通道
				switch {
				case errors.Is(e, context.Canceled):
					log.InfofCtx(cg.ctx, "Consumer fetch canceled: %s[%d]: %v", fe.Topic, fe.Partition, e)
				case errors.Is(e, context.DeadlineExceeded):
					log.WarnfCtx(cg.ctx, "Consumer fetch timeout: %s[%d]: %v", fe.Topic, fe.Partition, e)
				case kerr.IsRetriable(e):
					log.WarnfCtx(cg.ctx, "Consumer fetch retriable error: %s[%d]: %v", fe.Topic, fe.Partition, e)
				default:
					log.ErrorfCtx(cg.ctx, "Consumer fetch non-retriable error: %s[%d]: %v", fe.Topic, fe.Partition, e)
					select {
					case cg.errorChan <- e:
					default:
						log.WarnfCtx(cg.ctx, "Error channel full, dropping non-retriable fetch error: %v", e)
					}
				}
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

				// 将记录送入该分区的串行通道，保证同分区顺序处理
				ch := cg.getPartitionChan(topic.Topic, partition.Partition)
				select {
				case <-cg.ctx.Done():
					return
				case ch <- partition.Records:
				}
			}
		}
	}
}

// getPartitionChan 获取或创建指定分区的串行通道，并启动对应 worker
func (cg *ConsumerGroup) getPartitionChan(topic string, partition int32) chan []*kgo.Record {
	key := cg.partitionKey(topic, partition)
	cg.partMu.Lock()
	ch, ok := cg.partChans[key]
	if !ok {
		ch = make(chan []*kgo.Record, 1)
		cg.partChans[key] = ch
		// 启动该分区的串行 worker
		go func(t string, p int32, c chan []*kgo.Record) {
			for {
				select {
				case <-cg.ctx.Done():
					return
				case recs, ok := <-c:
					if !ok {
						return
					}
					// 串行处理该分区的一批记录
					cg.processRecordsSerial(t, p, recs)
				}
			}
		}(topic, partition, ch)
	}
	cg.partMu.Unlock()
	return ch
}

// closePartitionChan 关闭并移除指定分区通道
func (cg *ConsumerGroup) closePartitionChan(topic string, partition int32) {
	key := cg.partitionKey(topic, partition)
	cg.partMu.Lock()
	if ch, ok := cg.partChans[key]; ok {
		delete(cg.partChans, key)
		close(ch)
	}
	cg.partMu.Unlock()
}

func (cg *ConsumerGroup) partitionKey(topic string, partition int32) string {
	return fmt.Sprintf("%s:%d", topic, partition)
}

// processRecordsSerial 同步串行处理指定分区的一批记录，遇错即停，仅提交连续成功的最后一条
func (cg *ConsumerGroup) processRecordsSerial(topic string, partition int32, records []*kgo.Record) {
	var lastSuccess *kgo.Record
	for _, record := range records {
		start := time.Now()
		ctx, cancel := context.WithTimeout(cg.ctx, 30*time.Second)
		err := cg.handler.Handle(ctx, topic, partition, record.Offset, record.Key, record.Value)
		cancel()

		if err != nil {
			cg.metrics.IncrementConsumerErrors()
			log.ErrorfCtx(cg.ctx, "Failed to process message from topic %s partition %d: %v", topic, partition, err)
			select {
			case cg.errorChan <- err:
			default:
				log.ErrorfCtx(cg.ctx, "Error channel is full, dropping error: %v", err)
			}
			break // 遇错即停，保证不推进偏移
		}

		cg.metrics.IncrementConsumedMessages(1)
		cg.metrics.IncrementConsumedBytes(int64(len(record.Value)))
		cg.metrics.SetConsumerLatency(time.Since(start))
		lastSuccess = record
	}

	// 根据配置提交偏移量
	if lastSuccess != nil && cg.client != nil {
		if cg.autoCommit {
			cg.client.MarkCommitRecords(lastSuccess)
		} else {
			if err := cg.client.CommitRecords(cg.ctx, lastSuccess); err != nil {
				cg.metrics.IncrementOffsetCommitErrors()
				log.ErrorfCtx(cg.ctx, "CommitRecords failed for %s[%d]@%d: %v", topic, partition, lastSuccess.Offset, err)
			} else {
				cg.metrics.IncrementOffsetCommits()
			}
		}
	}
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
		case ev := <-cg.rebalanceChan:
			if len(ev.Revoked) > 0 {
				log.InfofCtx(cg.ctx, "Partitions revoked: %+v", ev.Revoked)
				// 在撤销前尝试提交未提交的偏移：仅在自动提交模式下才需要
				if cg.autoCommit {
					if err := cg.client.CommitUncommittedOffsets(cg.ctx); err != nil {
						cg.metrics.IncrementOffsetCommitErrors()
						log.WarnfCtx(cg.ctx, "CommitUncommittedOffsets warn: %v", err)
					} else {
						cg.metrics.IncrementOffsetCommits()
					}
				}
				// 关闭被撤销分区的 worker 通道，避免资源泄漏
				for topic, parts := range ev.Revoked {
					for _, p := range parts {
						cg.closePartitionChan(topic, p)
					}
				}
				// 可在此处添加本地资源清理/状态迁移
			}
			if len(ev.Assigned) > 0 {
				log.InfofCtx(cg.ctx, "Partitions assigned: %+v", ev.Assigned)
				// 可在此处执行预热/恢复
			}
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
