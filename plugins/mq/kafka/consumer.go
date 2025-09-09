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

// MessageHandler message handler interface
type MessageHandler interface {
	Handle(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error
}

// MessageHandlerFunc message handler function type
type MessageHandlerFunc func(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error

// Handle implements MessageHandler interface
func (f MessageHandlerFunc) Handle(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error {
	return f(ctx, topic, partition, offset, key, value)
}

// ConsumerGroup consumer group
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
	// per-partition serial processing channels, ensuring strict order for the same partition
	partChans map[string]chan []*kgo.Record
	partMu    sync.Mutex

	// internal wait group for background goroutines (errors/rebalances)
	wg sync.WaitGroup
}

// ConsumerGroupOptions optional parameters for creating consumer group
type ConsumerGroupOptions struct {
	// MaxConcurrency concurrency limit. Priority: Options.MaxConcurrency > conf.Consumer.MaxConcurrency > DefaultPoolConfig().Size
	MaxConcurrency int
}

// RebalanceEvent represents partition rebalance events
type RebalanceEvent struct {
	Assigned map[string][]int32
	Revoked  map[string][]int32
}

// NewConsumerGroup creates a new consumer group (based on instance configuration)
func (k *Client) NewConsumerGroup(client *kgo.Client, c *conf.Consumer, topics []string, handler MessageHandler) *ConsumerGroup {
	return k.NewConsumerGroupWithOptions(client, c, topics, handler, nil)
}

// NewConsumerGroupWithOptions creates a new consumer group with optional overrides (e.g., concurrency)
func (k *Client) NewConsumerGroupWithOptions(client *kgo.Client, c *conf.Consumer, topics []string, handler MessageHandler, opts *ConsumerGroupOptions) *ConsumerGroup {
	ctx, cancel := context.WithCancel(k.ctx)
	// Default concurrency: from DefaultPoolConfig
	maxConc := DefaultPoolConfig().Size
	// Override by configuration
	if c != nil && c.MaxConcurrency > 0 {
		maxConc = int(c.MaxConcurrency)
	}
	// Override by runtime options
	if opts != nil && opts.MaxConcurrency > 0 {
		maxConc = opts.MaxConcurrency
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

// initConsumerInstance initializes a consumer instance with the given name
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

	// Dispatch rebalance events to the active group of the corresponding instance
	opts = append(opts,
		kgo.OnPartitionsAssigned(func(ctx context.Context, c *kgo.Client, assigned map[string][]int32) {
			var target *ConsumerGroup
			k.mu.RLock()
			// Reverse lookup instance name via client pointer
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

// getStartOffset gets the start offset (based on instance configuration)
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

// Subscribe subscribes topics (route to the first enabled consumer instance)
func (k *Client) Subscribe(ctx context.Context, topics []string, handler MessageHandler) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics provided")
	}
	// Select the first enabled consumer as default
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

// SubscribeWith subscribes by consumer instance name
func (k *Client) SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error {
	return k.SubscribeWithOptions(ctx, consumerName, topics, handler, nil)
}

// SubscribeWithOptions subscribes by consumer instance name (with optional overrides)
func (k *Client) SubscribeWithOptions(ctx context.Context, consumerName string, topics []string, handler MessageHandler, opts *ConsumerGroupOptions) error {
	if len(topics) == 0 {
		return fmt.Errorf("no topics provided")
	}
	// Find matching configuration
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

	// Get or initialize client
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
		// Start the consumer connection manager
		if _, ok := k.consConnMgrs[consumerName]; !ok {
			cm := NewConnectionManager(cli, k.conf.GetBrokers())
			k.consConnMgrs[consumerName] = cm
			cm.Start()
			log.Infof("Kafka consumer[%s] connection manager started", consumerName)
			// Register health metrics
			k.registerHealthForConsumer(consumerName)
		}
		k.mu.Unlock()
	}

	// If there is an active group, stop the old group for this instance first
	k.mu.Lock()
	if old := k.activeGroups[consumerName]; old != nil {
		old.Stop()
		delete(k.activeGroups, consumerName)
	}
	cg := k.NewConsumerGroupWithOptions(consumer, cconf, topics, handler, opts)
	k.activeGroups[consumerName] = cg
	// Backward compatibility: if legacy fields are not set, set to current instance
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

// Start starts the consumer group
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

	// Start error handling goroutine
	cg.wg.Add(1)
	go func() {
		defer cg.wg.Done()
		cg.handleErrors()
	}()

	// Start rebalance handling goroutine
	cg.wg.Add(1)
	go func() {
		defer cg.wg.Done()
		cg.handleRebalances()
	}()

	// Begin consuming
	for {
		select {
		case <-cg.ctx.Done():
			// Graceful shutdown is not treated as an error to reduce log noise
			return nil
		default:
		}

		fetches := cg.client.PollFetches(cg.ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				cg.metrics.IncrementConsumerErrors()
				e := fe.Err
				// Categorization: context canceled/deadline exceeded are expected; retriable Kafka errors -> Warn; non-retriable -> Error and report via error channel
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

		// Process messages
		cg.processFetches(fetches)
	}
}

// processFetches processes fetched messages
func (cg *ConsumerGroup) processFetches(fetches kgo.Fetches) {
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				if len(partition.Records) == 0 {
					continue
				}

				// Send records to the serial channel of this partition to ensure in-partition ordering
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

// getPartitionChan gets or creates the serial channel for the given partition and starts its worker
func (cg *ConsumerGroup) getPartitionChan(topic string, partition int32) chan []*kgo.Record {
	key := cg.partitionKey(topic, partition)
	cg.partMu.Lock()
	ch, ok := cg.partChans[key]
	if !ok {
		ch = make(chan []*kgo.Record, 1)
		cg.partChans[key] = ch
		// Start the serial worker for this partition
		go func(t string, p int32, c chan []*kgo.Record) {
			for {
				select {
				case <-cg.ctx.Done():
					return
				case recs, ok := <-c:
					if !ok {
						return
					}
					// Use the goroutine pool to process a batch for this partition with strict in-partition order:
					// submit to the pool, then synchronously wait within this worker before fetching the next batch
					done := make(chan struct{})
					cg.pool.Submit(func() {
						cg.processRecordsSerial(t, p, recs)
						close(done)
					})
					select {
					case <-cg.ctx.Done():
						return
					case <-done:
					}
				}
			}
		}(topic, partition, ch)
	}
	cg.partMu.Unlock()
	return ch
}

// closePartitionChan closes and removes the channel for the specified partition
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

// processRecordsSerial synchronously processes a batch of records for the given partition; stops on first error and only commits the last consecutive success
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
			break // Stop on first error to avoid advancing offset
		}

		cg.metrics.IncrementConsumedMessages(1)
		cg.metrics.IncrementConsumedBytes(int64(len(record.Value)))
		cg.metrics.SetConsumerLatency(time.Since(start))
		lastSuccess = record
	}

	// Commit offset according to configuration
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

// handleErrors handles errors
func (cg *ConsumerGroup) handleErrors() {
	for {
		select {
		case <-cg.ctx.Done():
			return
		case err := <-cg.errorChan:
			log.ErrorfCtx(cg.ctx, "Consumer error: %v", err)
			// You can add error handling here, e.g., send to a dead-letter queue
		}
	}
}

// handleRebalances handles partition rebalances
func (cg *ConsumerGroup) handleRebalances() {
	for {
		select {
		case <-cg.ctx.Done():
			return
		case ev := <-cg.rebalanceChan:
			if len(ev.Revoked) > 0 {
				log.InfofCtx(cg.ctx, "Partitions revoked: %+v", ev.Revoked)
				// Try to commit uncommitted offsets before revocation: only needed in auto-commit mode
				if cg.autoCommit {
					if err := cg.client.CommitUncommittedOffsets(cg.ctx); err != nil {
						cg.metrics.IncrementOffsetCommitErrors()
						log.WarnfCtx(cg.ctx, "CommitUncommittedOffsets warn: %v", err)
					} else {
						cg.metrics.IncrementOffsetCommits()
					}
				}
				// Close worker channels for revoked partitions to avoid resource leaks
				for topic, parts := range ev.Revoked {
					for _, p := range parts {
						cg.closePartitionChan(topic, p)
					}
				}
				// Optionally add local resource cleanup/state migration here
			}
			if len(ev.Assigned) > 0 {
				log.InfofCtx(cg.ctx, "Partitions assigned: %+v", ev.Assigned)
				// Optionally perform warm-up/recovery here
			}
		}
	}
}

// Stop stops the consumer group
func (cg *ConsumerGroup) Stop() {
	cg.cancel()
	cg.pool.Wait()
	// Close and purge all partition channels to release workers promptly
	cg.partMu.Lock()
	for key, ch := range cg.partChans {
		delete(cg.partChans, key)
		close(ch)
	}
	cg.partMu.Unlock()
	// Wait background goroutines to finish
	cg.wg.Wait()
}

// IsRunning checks whether it is running
func (cg *ConsumerGroup) IsRunning() bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.isRunning
}

// GetConsumer returns the consumer client (for advanced operations)
func (k *Client) GetConsumer() *kgo.Client {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.consumer != nil {
		return k.consumer
	}
	// Fallback: return any initialized consumer
	for _, c := range k.consumers {
		if c != nil {
			return c
		}
	}
	return nil
}

// IsConsumerReady checks whether the consumer is ready
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
