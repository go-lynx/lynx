package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
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

// NewConsumerGroup 创建新的消费者组（按实例配置）
func (k *Client) NewConsumerGroup(client *kgo.Client, c *conf.Consumer, topics []string, handler MessageHandler) *ConsumerGroup {
	ctx, cancel := context.WithCancel(k.ctx)
	maxConc := 10
	if c != nil && c.MaxConcurrency > 0 {
		maxConc = int(c.MaxConcurrency)
	}
	autoCommit := false
	if c != nil {
		autoCommit = c.AutoCommit
	}
	return &ConsumerGroup{
		client:        client,
		groupID:       c.GetGroupId(),
		topics:        topics,
		handler:       handler,
		pool:          NewGoroutinePool(maxConc),
		metrics:       k.metrics,
		ctx:           ctx,
		cancel:        cancel,
		errorChan:     make(chan error, 100),
		rebalanceChan: make(chan RebalanceEvent, 16),
		autoCommit:    autoCommit,
		partChans:     make(map[string]chan []*kgo.Record),
	}
}

// initConsumerInstance 初始化指定名称的消费者实例
func (k *Client) initConsumerInstance(name string, cconf *conf.Consumer) (*kgo.Client, error) {
	if cconf == nil {
		return nil, fmt.Errorf("consumer config is nil for %s", name)
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.conf.Brokers...),
		kgo.ConsumerGroup(cconf.GroupId),
		kgo.ConsumeResetOffset(k.getStartOffset(cconf)),
		kgo.DialTimeout(k.conf.DialTimeout.AsDuration()),
	}
	if cconf.RebalanceTimeout != nil && cconf.RebalanceTimeout.AsDuration() > 0 {
		opts = append(opts, kgo.RebalanceTimeout(cconf.RebalanceTimeout.AsDuration()))
	}

	if cconf.AutoCommit {
		opts = append(opts,
			kgo.AutoCommitInterval(cconf.AutoCommitInterval.AsDuration()),
			kgo.AutoCommitMarks(),
		)
	}

	// 重平衡事件分派到对应实例的活跃组
	opts = append(opts,
		kgo.OnPartitionsAssigned(func(ctx context.Context, c *kgo.Client, assigned map[string][]int32) {
			var target *ConsumerGroup
			k.mu.RLock()
			// 通过客户端指针反查实例名
			for in, cli := range k.consumers {
				if cli == c {
					target = k.activeGroups[in]
					break
				}
			}
			k.mu.RUnlock()
			if target != nil {
				select {
				case <-target.ctx.Done():
				case target.rebalanceChan <- RebalanceEvent{Assigned: assigned}:
				default:
					log.WarnfCtx(ctx, "rebalance channel full, dropping assigned event")
				}
			} else {
				log.InfofCtx(ctx, "Partitions assigned: %+v", assigned)
			}
		}),
		kgo.OnPartitionsRevoked(func(ctx context.Context, c *kgo.Client, revoked map[string][]int32) {
			var target *ConsumerGroup
			k.mu.RLock()
			for in, cli := range k.consumers {
				if cli == c {
					target = k.activeGroups[in]
					break
				}
			}
			k.mu.RUnlock()
			if target != nil {
				select {
				case <-target.ctx.Done():
				case target.rebalanceChan <- RebalanceEvent{Revoked: revoked}:
				default:
					log.WarnfCtx(ctx, "rebalance channel full, dropping revoked event")
				}
			} else {
				log.InfofCtx(ctx, "Partitions revoked: %+v", revoked)
			}
		}),
	)

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

	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	return consumer, nil
}

// getStartOffset 获取起始偏移量（按实例配置）
func (k *Client) getStartOffset(c *conf.Consumer) kgo.Offset {
	if c == nil {
		return kgo.NewOffset().AtEnd()
	}
	switch c.StartOffset {
	case StartOffsetEarliest:
		return kgo.NewOffset().AtStart()
	case StartOffsetLatest:
		fallthrough
	default:
		return kgo.NewOffset().AtEnd()
	}
}

// Subscribe 订阅主题（路由到第一个启用的消费者实例）
func (k *Client) Subscribe(ctx context.Context, topics []string, handler MessageHandler) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics provided")
	}
	// 选择第一个启用的消费者作为默认
	var chosen *conf.Consumer
	var name string
	for _, c := range k.conf.Consumers {
		if c != nil && c.Enabled {
			chosen = c
			name = c.GetName()
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("no enabled consumer configured")
	}
	return k.SubscribeWith(ctx, name, topics, handler)
}

// SubscribeWith 按消费者实例名订阅
func (k *Client) SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics provided")
	}
	// 找到对应配置
	var cconf *conf.Consumer
	for _, c := range k.conf.Consumers {
		if c != nil && c.Enabled && c.GetName() == consumerName {
			cconf = c
			break
		}
	}
	if cconf == nil {
		return fmt.Errorf("consumer instance %s not found or disabled", consumerName)
	}

	// 获取或初始化客户端
	k.mu.RLock()
	consumer := k.consumers[consumerName]
	k.mu.RUnlock()
	if consumer == nil {
		cli, err := k.initConsumerInstance(consumerName, cconf)
		if err != nil {
			return err
		}
		k.mu.Lock()
		k.consumers[consumerName] = cli
		consumer = cli
		// 启动消费者连接管理器
		if _, ok := k.consConnMgrs[consumerName]; !ok {
			cm := NewConnectionManager(cli, k.conf.GetBrokers())
			k.consConnMgrs[consumerName] = cm
			cm.Start()
			log.Infof("Kafka consumer[%s] connection manager started", consumerName)
		}
		k.mu.Unlock()
	}

	// 若已有活跃组，先停止该实例对应的旧组
	k.mu.Lock()
	if old := k.activeGroups[consumerName]; old != nil {
		old.Stop()
		delete(k.activeGroups, consumerName)
	}
	cg := k.NewConsumerGroup(consumer, cconf, topics, handler)
	k.activeGroups[consumerName] = cg
	// 兼容旧字段：若 legacy 未设置，则设置为当前实例
	if k.consumer == nil {
		k.consumer = consumer
		k.activeConsumerGroup = cg
	}
	k.mu.Unlock()

	if err := cg.Start(); err != nil {
		return fmt.Errorf("consumer group start failed: %w", err)
	}
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
	if k.consumer != nil {
		return k.consumer
	}
	// 回退：返回任一已初始化消费者
	for _, c := range k.consumers {
		if c != nil {
			return c
		}
	}
	return nil
}

// IsConsumerReady 检查消费者是否就绪
func (k *Client) IsConsumerReady() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.consumer != nil {
		return true
	}
	for _, c := range k.consumers {
		if c != nil {
			return true
		}
	}
	return false
}
