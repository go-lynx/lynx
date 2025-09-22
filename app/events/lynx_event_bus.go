package events

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kelindarEvent "github.com/kelindar/event"
	ants "github.com/panjf2000/ants/v2"
)

// SimpleThrottler implements a basic rate limiter
type SimpleThrottler struct {
	rate     int       // events per second
	burst    int       // burst size
	tokens   int       // current tokens
	lastTime time.Time // last token refill time
	mu       sync.Mutex
}

// wrapHandler wraps user handler with retry & panic recovery
func (b *LynxEventBus) wrapHandler(handler func(LynxEvent)) func(LynxEvent) {
	return func(ev LynxEvent) {
		b.handleWithRetry(ev, handler, 1)
	}
}

// Subscribe subscribes to events on this bus
func (b *LynxEventBus) Subscribe(handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(handler)
	cancel := kelindarEvent.Subscribe(b.dispatcher, wrapped)
	b.subscriberCount.Add(1)
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
	}
}

// SubscribeTo subscribes to a specific event type on this bus
func (b *LynxEventBus) SubscribeTo(eventType EventType, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(handler)
	cancel := kelindarEvent.SubscribeTo(b.dispatcher, uint32(eventType), wrapped)
	b.subscriberCount.Add(1)
	b.mu.Lock()
	b.typeSubs[eventType]++
	b.mu.Unlock()
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
		b.mu.Lock()
		if b.typeSubs[eventType] > 0 {
			b.typeSubs[eventType]--
		}
		b.mu.Unlock()
	}
}

// SubscribeWithFilter subscribes with a predicate filter
func (b *LynxEventBus) SubscribeWithFilter(filter func(LynxEvent) bool, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(func(ev LynxEvent) {
		if filter == nil || filter(ev) {
			handler(ev)
		}
	})
	cancel := kelindarEvent.Subscribe(b.dispatcher, wrapped)
	b.subscriberCount.Add(1)
	return func() { cancel(); b.subscriberCount.Add(-1) }
}

// SubscribeToWithFilter subscribes to a specific event type with a predicate
func (b *LynxEventBus) SubscribeToWithFilter(eventType EventType, filter func(LynxEvent) bool, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(func(ev LynxEvent) {
		if filter == nil || filter(ev) {
			handler(ev)
		}
	})
	cancel := kelindarEvent.SubscribeTo(b.dispatcher, uint32(eventType), wrapped)
	b.subscriberCount.Add(1)
	b.mu.Lock()
	b.typeSubs[eventType]++
	b.mu.Unlock()
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
		b.mu.Lock()
		if b.typeSubs[eventType] > 0 {
			b.typeSubs[eventType]--
		}
		b.mu.Unlock()
	}
}

// Close closes the event bus
func (b *LynxEventBus) Close() error {
	if b.isClosed.CompareAndSwap(false, true) {
		close(b.done)
		b.wg.Wait()
		if b.workerPool != nil {
			b.workerPool.Release()
		}
		return b.dispatcher.Close()
	}
	return nil
}

// IsClosed returns whether the bus is closed
func (b *LynxEventBus) IsClosed() bool { return b.isClosed.Load() }

// SetLogger sets the logger for this bus
func (b *LynxEventBus) SetLogger(logger log.Logger) { b.logger = logger }

// GetLogger returns the logger for this bus
func (b *LynxEventBus) GetLogger() log.Logger { return b.logger }

// GetBusType returns the bus type
func (b *LynxEventBus) GetBusType() BusType { return b.busType }

// GetConfig returns the bus configuration
func (b *LynxEventBus) GetConfig() BusConfig { return b.config }

// GetHistory returns the event history (if enabled)
func (b *LynxEventBus) GetHistory() *EventHistory { return b.history }

// GetMetrics returns the event metrics (if enabled)
func (b *LynxEventBus) GetMetrics() *EventMetrics { return b.metrics }

// IsHealthy returns whether the bus is healthy
func (b *LynxEventBus) IsHealthy() bool {
	if b.isClosed.Load() || b.paused.Load() || b.isDegraded.Load() {
		return false
	}
	totalCap := b.totalQueueCap()
	if totalCap > 0 {
		if b.totalQueueSize()*100/totalCap >= 80 {
			return false
		}
	}
	return true
}

// IsPaused returns whether the bus is paused
func (b *LynxEventBus) IsPaused() bool { return b.paused.Load() }

// IsDegraded returns whether the bus is currently degraded
func (b *LynxEventBus) IsDegraded() bool { return b.isDegraded.Load() }

// GetQueueSize returns the current queue size (approximate)
func (b *LynxEventBus) GetQueueSize() int { return b.totalQueueSize() }

// GetTotalSubscriberCount returns total subscribers
func (b *LynxEventBus) GetTotalSubscriberCount() int { return int(b.subscriberCount.Load()) }

// GetSubscriberCount returns the number of subscribers for a specific event type
func (b *LynxEventBus) GetSubscriberCount(eventType EventType) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.typeSubs[eventType]
}

// Pause stops consuming events from internal queue (publishing still enqueues)
func (b *LynxEventBus) Pause() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.paused.Load() {
		b.pauseStartTime = time.Now()
		b.paused.Store(true)
		b.pauseCount.Add(1)
		// Emit pause event (best effort)
		pauseEvent := LynxEvent{EventType: EventSystemError, Priority: PriorityHigh, Source: "event-bus", Category: "system", PluginID: "system", Status: "paused", Timestamp: time.Now().Unix(), Metadata: map[string]any{"bus_type": b.busType, "reason": "manual_pause"}}
		if manager := GetGlobalEventBus(); manager != nil {
			_ = manager.PublishEvent(pauseEvent)
		}
	}
}

// Resume resumes consuming events
func (b *LynxEventBus) Resume() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.paused.Load() {
		b.pauseDuration += time.Since(b.pauseStartTime)
		b.paused.Store(false)
		resumeEvent := LynxEvent{EventType: EventSystemError, Priority: PriorityNormal, Source: "event-bus", Category: "system", PluginID: "system", Status: "resumed", Timestamp: time.Now().Unix(), Metadata: map[string]any{"bus_type": b.busType, "pause_duration": b.pauseDuration.String(), "reason": "manual_resume"}}
		if manager := GetGlobalEventBus(); manager != nil {
			_ = manager.PublishEvent(resumeEvent)
		}
	}
}

// GetPauseStats returns pause statistics
func (b *LynxEventBus) GetPauseStats() (time.Duration, time.Time) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.paused.Load() {
		return b.pauseDuration + time.Since(b.pauseStartTime), b.pauseStartTime
	}
	return b.pauseDuration, time.Time{}
}

// GetPauseCount returns how many times the bus has been paused
func (b *LynxEventBus) GetPauseCount() int64 { return b.pauseCount.Load() }

// GetDegradationDuration returns how long the bus has been degraded; zero if not degraded
func (b *LynxEventBus) GetDegradationDuration() time.Duration {
	if !b.isDegraded.Load() {
		return 0
	}
	return time.Since(b.degradationStartTime)
}

// GetWorkerPoolStats exposes current worker pool stats; returns zeros if pool not initialized
func (b *LynxEventBus) GetWorkerPoolStats() (cap int, running int, free int, waiting int) {
	b.mu.RLock()
	pool := b.workerPool
	b.mu.RUnlock()
	if pool == nil {
		return 0, 0, 0, 0
	}
	return pool.Cap(), pool.Running(), pool.Free(), pool.Waiting()
}

// checkDegradation toggles degradation state based on configured thresholds and queue usage
func (b *LynxEventBus) checkDegradation() {
	capTotal := b.totalQueueCap()
	thr := b.config.DegradationThreshold
	if capTotal <= 0 || thr <= 0 {
		return
	}
	usage := b.totalQueueSize() * 100 / capTotal

	if !b.isDegraded.Load() {
		if usage >= thr {
			b.isDegraded.Store(true)
			b.degradationStartTime = time.Now()
			// Optional: auto pause when configured
			if b.config.DegradationMode == DegradationModePause {
				b.Pause()
			}
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf("bus degraded: bus=%d usage=%d%% thr=%d%% mode=%s", b.busType, usage, thr, b.config.DegradationMode)
			}
			GetGlobalMonitor().UpdateHealth(false)
		}
		return
	}

	// degraded -> check recovery
	rec := b.config.DegradationRecoverThreshold
	if rec <= 0 { // derive default recover threshold
		rec = thr - 10
		if rec < 1 {
			rec = 1
		}
	}
	if usage <= rec {
		b.isDegraded.Store(false)
		if b.config.DegradationMode == DegradationModePause {
			b.Resume()
		}
		if b.logger != nil {
			log.NewHelper(b.logger).Infof("bus recovered: bus=%d usage=%d%% rec=%d%%", b.busType, usage, rec)
		}
		GetGlobalMonitor().UpdateHealth(true)
	}
}

// GetEventHistory returns events from history that match the given filter
func (b *LynxEventBus) GetEventHistory(filter *EventFilter) []LynxEvent {
	if b.history == nil {
		return []LynxEvent{}
	}
	if filter == nil {
		return b.history.GetEvents()
	}
	return b.history.GetEventsByFilter(filter)
}

// GetPluginEventHistory returns events from history for a specific plugin
func (b *LynxEventBus) GetPluginEventHistory(pluginID string, filter *EventFilter) []LynxEvent {
	if b.history == nil {
		return []LynxEvent{}
	}
	pluginFilter := &EventFilter{PluginIDs: []string{pluginID}}
	if filter != nil {
		if len(filter.EventTypes) > 0 {
			pluginFilter.EventTypes = filter.EventTypes
		}
		if len(filter.Priorities) > 0 {
			pluginFilter.Priorities = filter.Priorities
		}
		if len(filter.Categories) > 0 {
			pluginFilter.Categories = filter.Categories
		}
		if filter.FromTime > 0 {
			pluginFilter.FromTime = filter.FromTime
		}
		if filter.ToTime > 0 {
			pluginFilter.ToTime = filter.ToTime
		}
		if len(filter.Metadata) > 0 {
			pluginFilter.Metadata = filter.Metadata
		}
		pluginFilter.HasError = filter.HasError
		if len(filter.Statuses) > 0 {
			pluginFilter.Statuses = filter.Statuses
		}
	}
	return b.history.GetEventsByFilter(pluginFilter)
}

// run drains the queue and publishes to the dispatcher
func (b *LynxEventBus) run() {
	ticker := time.NewTicker(b.config.FlushInterval)
	defer ticker.Stop()
	defer b.wg.Done()
	for {
		select {
		case <-b.done:
			drainDeadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(drainDeadline) {
				drained := false
				bs := max(b.config.BatchSize, 1)
				buf := b.bufferPool.GetWithCapacity(bs)
				for len(buf) < bs {
					if ev, ok := b.tryRecvShared(); ok {
						buf = append(buf, ev)
					} else {
						break
					}
				}
				if len(buf) > 0 {
					b.publishBatch(buf)
					drained = true
				}
				b.bufferPool.Put(buf)
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
				if !drained {
					return
				}
			}
			return
		case <-ticker.C:
			GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
		default:
			if b.paused.Load() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			batchSize := max(b.config.BatchSize, 1)
			buf := b.bufferPool.GetWithCapacity(batchSize)
			for len(buf) < batchSize {
				if ev, ok := b.tryRecvShared(); ok {
					buf = append(buf, ev)
				} else {
					break
				}
			}
			if len(buf) > 0 {
				ordered := orderByWeightedPriority(buf)
				b.publishBatch(ordered)
			}
			b.bufferPool.Put(buf)
			// block until any event arrives or ticker/done
			select {
			case <-b.done:
				continue
			case <-ticker.C:
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
			case ev := <-b.queue:
				buf := b.bufferPool.GetWithCapacity(max(b.config.BatchSize, 1))
				buf = append(buf, ev)
				for len(buf) < max(b.config.BatchSize, 1) {
					if ev2, ok := b.tryRecvShared(); ok {
						buf = append(buf, ev2)
					} else {
						break
					}
				}
				ordered := orderByWeightedPriority(buf)
				b.publishBatch(ordered)
				b.bufferPool.Put(buf)
			}
		}
	}
}

// orderByWeightedPriority reorders the batch by weighted fair policy: critical>high>normal>low
func orderByWeightedPriority(in []LynxEvent) []LynxEvent {
	if len(in) <= 1 {
		return in
	}
	var crit, high, norm, low []LynxEvent
	for i := range in {
		switch in[i].Priority {
		case PriorityCritical:
			crit = append(crit, in[i])
		case PriorityHigh:
			high = append(high, in[i])
		case PriorityLow:
			low = append(low, in[i])
		default:
			norm = append(norm, in[i])
		}
	}
	out := make([]LynxEvent, 0, len(in))
	wc, wh, wn, wl := 8, 4, 2, 1
	ic, ih, inx, il := 0, 0, 0, 0
	for ic < len(crit) || ih < len(high) || inx < len(norm) || il < len(low) {
		for k := 0; k < wc && ic < len(crit); k++ {
			out = append(out, crit[ic])
			ic++
		}
		for k := 0; k < wh && ih < len(high); k++ {
			out = append(out, high[ih])
			ih++
		}
		for k := 0; k < wn && inx < len(norm); k++ {
			out = append(out, norm[inx])
			inx++
		}
		for k := 0; k < wl && il < len(low); k++ {
			out = append(out, low[il])
			il++
		}
		if wc > 1 {
			wc--
		}
		if wh > 1 {
			wh--
		}
	}
	return out
}

// (removed) helper: choose queue by priority — switched to single shared queue

// helper: total queue size (single shared queue)
func (b *LynxEventBus) totalQueueSize() int { return len(b.queue) }

// helper: total queue capacity (single shared queue)
func (b *LynxEventBus) totalQueueCap() int { return cap(b.queue) }

// helper: non-blocking receive from shared queue
func (b *LynxEventBus) tryRecvShared() (LynxEvent, bool) {
	select {
	case ev := <-b.queue:
		return ev, true
	default:
		return LynxEvent{}, false
	}
}

// helper: publish to dispatcher and update monitor after dequeue
func (b *LynxEventBus) publish(ev LynxEvent) {
	if b.workerPool != nil {
		// Submit to worker pool; in blocking mode errors are rare (e.g., pool released)
		if err := b.workerPool.Submit(func() { kelindarEvent.Publish(b.dispatcher, ev) }); err != nil {
			// overload or pool closed -> sync fallback and record
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf("worker pool submit failed: %v", err)
			}
			GetGlobalMonitor().SetError(fmt.Errorf("worker_pool_submit_failed: %v", err))
			kelindarEvent.Publish(b.dispatcher, ev)
		}
	} else {
		kelindarEvent.Publish(b.dispatcher, ev)
	}
	GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
}

// helper: publish a batch of events
func (b *LynxEventBus) publishBatch(events []LynxEvent) {
	// Check if throttling is active during degradation
	if b.isDegraded.Load() && b.throttler != nil {
		// Apply throttling to the batch
		throttledEvents := make([]LynxEvent, 0, len(events))
		for _, ev := range events {
			if b.throttler.Allow() {
				throttledEvents = append(throttledEvents, ev)
			} else {
				// Event is throttled, log it and continue
				if b.logger != nil {
					log.NewHelper(b.logger).Debugf("event throttled during degradation: type=%d, plugin=%s", ev.EventType, ev.PluginID)
				}
			}
		}
		// Use throttled events instead of original events
		events = throttledEvents
	}

	// Limit per-batch submissions to reduce burstiness
	limit := len(events)
	if b.config.WorkerCount > 0 {
		if m := b.config.WorkerCount * 2; m < limit {
			limit = m
		}
	}
	if b.workerPool != nil {
		for i := 0; i < limit; i++ {
			ev := events[i]
			if err := b.workerPool.Submit(func() { kelindarEvent.Publish(b.dispatcher, ev) }); err != nil {
				if b.logger != nil {
					log.NewHelper(b.logger).Warnf("worker pool submit failed: %v", err)
				}
				GetGlobalMonitor().SetError(fmt.Errorf("worker_pool_submit_failed: %v", err))
				kelindarEvent.Publish(b.dispatcher, ev)
			}
		}
		// overflow part (if any) publish synchronously to avoid long waits
		for i := limit; i < len(events); i++ {
			kelindarEvent.Publish(b.dispatcher, events[i])
		}
	} else {
		for i := range events {
			kelindarEvent.Publish(b.dispatcher, events[i])
		}
	}
	GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
}

// NewSimpleThrottler creates a new throttler
func NewSimpleThrottler(rate, burst int) *SimpleThrottler {
	return &SimpleThrottler{
		rate:     rate,
		burst:    burst,
		tokens:   burst,
		lastTime: time.Now(),
	}
}

// Allow checks if an event is allowed to proceed
func (t *SimpleThrottler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastTime)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds() * float64(t.rate))
	if tokensToAdd > 0 {
		t.tokens = min(t.tokens+tokensToAdd, t.burst)
		t.lastTime = now
	}

	if t.tokens > 0 {
		t.tokens--
		return true
	}
	return false
}

// LynxEventBus represents a single event bus for a specific bus type
type LynxEventBus struct {
	// kelindar/event dispatcher
	dispatcher *kelindarEvent.Dispatcher

	// Bus configuration
	config  BusConfig
	busType BusType

	// Lynx specific features
	history *EventHistory
	metrics *EventMetrics
	logger  log.Logger

	// State management
	done     chan struct{}
	mu       sync.RWMutex
	isClosed atomic.Bool
	wg       sync.WaitGroup
	paused   atomic.Bool

	// Backpressure queue (single shared queue)
	queue chan LynxEvent
	// Backpressure queue & subscribers tracking
	subscriberCount atomic.Int64
	// per-type subscriber counts for SubscribeTo
	typeSubs map[EventType]int

	// Pause tracking
	pauseStartTime time.Time
	pauseDuration  time.Duration
	pauseCount     atomic.Int64

	// Degradation tracking
	isDegraded           atomic.Bool
	degradationStartTime time.Time

	// Throttling for degradation mode
	throttler *SimpleThrottler

	// Worker pool for parallel dispatching
	workerPool *ants.Pool

	// Retry bounding (limit concurrent scheduled retries)
	retrySem chan struct{}

	// Memory optimization
	bufferPool   *EventBufferPool
	metadataPool *MetadataPool
}

// NewLynxEventBus creates a new LynxEventBus with the given configuration
func NewLynxEventBus(config BusConfig, busType BusType) *LynxEventBus {
	bus := &LynxEventBus{
		dispatcher: kelindarEvent.NewDispatcher(),
		config:     config,
		busType:    busType,
		done:       make(chan struct{}),
		typeSubs:   make(map[EventType]int),
	}

	// Initialize history if enabled
	if config.EnableHistory {
		size := config.HistorySize
		if size <= 0 {
			size = 1000
		}
		bus.history = NewEventHistory(size)
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		bus.metrics = NewEventMetrics()
	}

	// initialize single shared queue (unified capacity pool)
	capTotal := max(config.MaxQueue, 1)
	bus.queue = make(chan LynxEvent, capTotal)

	// Initialize worker pool (for parallel handler execution)
	poolSize := max(config.WorkerCount, 1)
	maxBlock := max(poolSize*4, 64)
	if p, err := ants.NewPool(poolSize, ants.WithNonblocking(false), ants.WithMaxBlockingTasks(maxBlock)); err == nil {
		bus.workerPool = p
	} else {
		// fallback: will publish synchronously if pool not available
		if bus.logger != nil {
			log.NewHelper(bus.logger).Warnf("worker pool init failed, fallback to sync: err=%v", err)
		}
	}

	// Initialize throttler if enabled
	if config.EnableThrottling {
		bus.throttler = NewSimpleThrottler(config.ThrottleRate, config.ThrottleBurst)
	}

	// Initialize retry semaphore if configured
	if config.MaxConcurrentRetries > 0 {
		bus.retrySem = make(chan struct{}, config.MaxConcurrentRetries)
	}

	// Initialize memory pools
	bus.bufferPool = GetGlobalEventBufferPool()
	bus.metadataPool = GetGlobalMetadataPool()

	// Start worker to drain queue and publish to dispatcher
	bus.wg.Add(1)
	go bus.run()

	return bus
}

// Publish publishes an event to this bus
func (b *LynxEventBus) Publish(event LynxEvent) {
	if b.isClosed.Load() {
		return
	}

	// Check throttling first
	if b.throttler != nil && !b.throttler.Allow() {
		// Event throttled - record and call error callback
		if b.metrics != nil {
			b.metrics.IncrementDropped()
		}
		GetGlobalMonitor().IncrementDroppedByReason("throttled")
		throttleErr := fmt.Errorf("event throttled: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		GetGlobalMonitor().SetError(throttleErr)

		// Call error callback if configured
		if b.config.ErrorCallback != nil {
			b.config.ErrorCallback(event, "throttled", throttleErr)
		}

		if b.logger != nil {
			log.NewHelper(b.logger).Warnf("event throttled: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		}
		return
	}

	// Update metrics
	if b.metrics != nil {
		b.metrics.IncrementPublished()
	}
	// Update global monitor (bucketed by priority)
	GetGlobalMonitor().IncrementPublishedByPriority(event.Priority)

	// Add to history if enabled
	if b.history != nil {
		b.history.Add(event)
	}

	// Check degradation before publishing
	b.checkDegradation()

	// Reserve headroom for critical events (if configured)
	if b.config.ReserveForCritical > 0 && event.Priority != PriorityCritical {
		// If used capacity already occupies reserve, treat as overflow for non-critical
		if b.totalQueueSize() >= max(b.totalQueueCap()-b.config.ReserveForCritical, 0) {
			b.handleEnqueueOverflow(event, "reserve_for_critical")
			return
		}
	}

	// Enqueue according to drop policy when full
	select {
	case b.queue <- event:
		GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
	default:
		// queue full
		switch b.config.DropPolicy {
		case DropBlock:
			timeout := b.config.EnqueueBlockTimeout
			if timeout <= 0 {
				timeout = 5 * time.Millisecond
			}
			select {
			case b.queue <- event:
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
			case <-time.After(timeout):
				b.handleEnqueueOverflow(event, "block_timeout")
			}
		case DropOldest:
			// Drop one oldest item to make room, then enqueue
			select {
			case <-b.queue:
				// space made
			default:
				// extremely rare if consumer raced, fallback
			}
			select {
			case b.queue <- event:
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
			default:
				b.handleEnqueueOverflow(event, "drop_oldest_failed")
			}
		default: // DropNewest (or unset)
			b.handleEnqueueOverflow(event, "drop_newest")
		}
	}
}

// UpdateConfig applies a subset of configuration at runtime (non-destructive)
// Supports MaxRetries, HistorySize (when history enabled), WorkerCount, MaxConcurrentRetries,
// Degradation hysteresis, Throttling settings, Drop policy and critical reserve
func (b *LynxEventBus) UpdateConfig(cfg BusConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// MaxRetries can be changed directly
	b.config.MaxRetries = cfg.MaxRetries
	// BatchSize hot update
	if cfg.BatchSize > 0 && cfg.BatchSize != b.config.BatchSize {
		b.config.BatchSize = cfg.BatchSize
	}
	// WorkerCount can be tuned dynamically if pool exists
	if b.workerPool != nil && cfg.WorkerCount > 0 && cfg.WorkerCount != b.config.WorkerCount {
		// ants v2 supports Tune to resize pool at runtime
		b.workerPool.Tune(cfg.WorkerCount)
		b.config.WorkerCount = cfg.WorkerCount
	}
	// History size can be changed by recreating history buffer
	if b.config.EnableHistory {
		size := cfg.HistorySize
		if size <= 0 {
			size = b.config.HistorySize
		}
		if size <= 0 {
			size = 1000
		}
		if size != b.config.HistorySize {
			b.history = NewEventHistory(size)
			b.config.HistorySize = size
		}
	}
	// Retry concurrency: only support enabling/disabling hot, resizing is skipped to avoid token leak
	if b.retrySem == nil && cfg.MaxConcurrentRetries > 0 {
		b.retrySem = make(chan struct{}, cfg.MaxConcurrentRetries)
	}
	if b.retrySem != nil && cfg.MaxConcurrentRetries <= 0 {
		b.retrySem = nil
	}
	b.config.MaxConcurrentRetries = cfg.MaxConcurrentRetries

	// Degradation hysteresis threshold hot value
	if cfg.DegradationRecoverThreshold >= 0 {
		b.config.DegradationRecoverThreshold = cfg.DegradationRecoverThreshold
	}

	// Throttling settings
	if cfg.EnableThrottling && b.throttler == nil {
		rate := cfg.ThrottleRate
		if rate <= 0 {
			rate = 1000
		}
		burst := cfg.ThrottleBurst
		if burst <= 0 {
			burst = 100
		}
		b.throttler = NewSimpleThrottler(rate, burst)
	} else if !cfg.EnableThrottling {
		b.throttler = nil
	} else if b.throttler != nil {
		if cfg.ThrottleRate != b.config.ThrottleRate || cfg.ThrottleBurst != b.config.ThrottleBurst {
			rate := cfg.ThrottleRate
			if rate <= 0 {
				rate = 1000
			}
			burst := cfg.ThrottleBurst
			if burst <= 0 {
				burst = 100
			}
			b.throttler = NewSimpleThrottler(rate, burst)
		}
	}
	b.config.EnableThrottling = cfg.EnableThrottling
	b.config.ThrottleRate = cfg.ThrottleRate
	b.config.ThrottleBurst = cfg.ThrottleBurst

	// Drop policy and critical reserve
	if cfg.DropPolicy != "" {
		b.config.DropPolicy = cfg.DropPolicy
	}
	if cfg.ReserveForCritical >= 0 {
		b.config.ReserveForCritical = cfg.ReserveForCritical
	}
	if cfg.EnqueueBlockTimeout >= 0 {
		b.config.EnqueueBlockTimeout = cfg.EnqueueBlockTimeout
	}
}

// handleEnqueueOverflow applies unified drop handling, metrics and callback
func (b *LynxEventBus) handleEnqueueOverflow(event LynxEvent, reason string) {
	if b.metrics != nil {
		b.metrics.IncrementDropped()
	}
	GetGlobalMonitor().IncrementDroppedByReason(reason)
	GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
	dropErr := fmt.Errorf("event dropped due to overflow: bus=%d prio=%d type=%d reason=%s", b.busType, event.Priority, event.EventType, reason)
	GetGlobalMonitor().SetError(dropErr)

	if b.config.ErrorCallback != nil {
		b.config.ErrorCallback(event, reason, dropErr)
	}
	if b.logger != nil {
		log.NewHelper(b.logger).Warnf("event bus overflow: bus=%d prio=%d type=%d reason=%s", b.busType, event.Priority, event.EventType, reason)
	}
}

// handleWithRetry executes the handler with panic recovery and schedules retries asynchronously
func (b *LynxEventBus) handleWithRetry(ev LynxEvent, handler func(LynxEvent), attempt int) {
	start := time.Now()
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler(ev)
	}()

	duration := time.Since(start)
	if panicked {
		// metrics & monitor for failure
		if b.metrics != nil {
			b.metrics.IncrementFailed()
		}
		GetGlobalMonitor().IncrementFailed()
		GetGlobalMonitor().SetError(fmt.Errorf("handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt))
		if b.logger != nil {
			log.NewHelper(b.logger).Errorf("event handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt)
		}

		maxRetries := max(b.config.MaxRetries, 0)
		if attempt <= maxRetries {
			// schedule next attempt asynchronously to avoid blocking worker threads
			backoffDelay := calculateBackoffDelay(attempt, maxRetries)
			// Bounded concurrency: respect MaxConcurrentRetries if configured
			if b.retrySem != nil {
				select {
				case b.retrySem <- struct{}{}:
					time.AfterFunc(backoffDelay, func() {
						defer func() { <-b.retrySem }()
						if b.isClosed.Load() {
							return
						}
						if b.workerPool != nil {
							_ = b.workerPool.Submit(func() { b.handleWithRetry(ev, handler, attempt+1) })
						} else {
							go b.handleWithRetry(ev, handler, attempt+1)
						}
					})
				default:
					// Retry capacity exhausted; emit error and stop retrying this event
					if b.logger != nil {
						log.NewHelper(b.logger).Warnf("retry capacity exhausted, dropping retries: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt)
					}
					b.emitErrorEvent(ev, attempt)
				}
			} else {
				time.AfterFunc(backoffDelay, func() {
					if b.isClosed.Load() {
						return
					}
					if b.workerPool != nil {
						_ = b.workerPool.Submit(func() { b.handleWithRetry(ev, handler, attempt+1) })
					} else {
						go b.handleWithRetry(ev, handler, attempt+1)
					}
				})
			}
			return
		}

		// emit error event (DLQ-like)
		b.emitErrorEvent(ev, attempt)
		// update latency (failed)
		if b.metrics != nil {
			b.metrics.UpdateLatency(duration)
		}
		GetGlobalMonitor().UpdateLatency(duration)
		return
	}

	// success path
	if b.metrics != nil {
		b.metrics.UpdateLatency(duration)
		b.metrics.IncrementProcessed()
	}
	GetGlobalMonitor().UpdateLatency(duration)
	GetGlobalMonitor().IncrementProcessed()
}

// helper: emit a system error event to act as DLQ
func (b *LynxEventBus) emitErrorEvent(original LynxEvent, attempts int) {
	// Create a fresh metadata map for the event to avoid referencing pooled objects
	meta := make(map[string]any, 6)
	meta["bus_type"] = b.busType
	meta["event_type"] = original.EventType
	meta["attempts"] = attempts
	meta["reason"] = "handler panic"
	if len(original.Metadata) > 0 {
		// Deep-copy original metadata into a nested map to avoid aliasing
		copied := make(map[string]any, len(original.Metadata))
		for k, v := range original.Metadata {
			// don't overwrite our reserved keys if present at top-level
			copied[k] = v
		}
		meta["metadata"] = copied
	}
	errEv := NewLynxEvent(EventErrorOccurred, original.PluginID, original.Source).WithPriority(PriorityHigh)
	errEv.Category = original.Category
	errEv.Metadata = meta
	// best effort publish via global manager if available
	if manager := GetGlobalEventBus(); manager != nil {
		_ = manager.PublishEvent(errEv)
	}
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculateBackoffDelay calculates exponential backoff delay with jitter
func calculateBackoffDelay(attempt, maxRetries int) time.Duration {
	// Base delay: 10ms * 2^(attempt-1)
	baseDelay := time.Duration(10) * time.Millisecond * time.Duration(1<<(attempt-1))

	// Add jitter (±25% random variation)
	jitter := float64(baseDelay) * (0.75 + 0.5*rand.Float64())

	// Cap maximum delay at 5 seconds
	maxDelay := 5 * time.Second
	if time.Duration(jitter) > maxDelay {
		return maxDelay
	}

	return time.Duration(jitter)
}
