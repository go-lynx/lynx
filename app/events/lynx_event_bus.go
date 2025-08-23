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

	// Backpressure queue & subscribers tracking
	// multi-priority queues
	lowQ      chan LynxEvent
	normalQ   chan LynxEvent
	highQ     chan LynxEvent
	criticalQ chan LynxEvent
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

	// Worker pool for parallel dispatching
	workerPool *ants.Pool

	// Throttling
	throttler *SimpleThrottler

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

	// initialize multi-priority queues (split capacity roughly equally)
	capPerQ := max(config.MaxQueue/4, 1)
	bus.lowQ = make(chan LynxEvent, capPerQ)
	bus.normalQ = make(chan LynxEvent, capPerQ)
	bus.highQ = make(chan LynxEvent, capPerQ)
	bus.criticalQ = make(chan LynxEvent, capPerQ)

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
		GetGlobalMonitor().IncrementDropped()
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
	// Update global monitor
	GetGlobalMonitor().IncrementPublished()

	// Add to history if enabled
	if b.history != nil {
		b.history.Add(event)
	}

	// Check degradation before publishing
	b.checkDegradation()

	// choose priority queue
	q := b.queueByPriority(event.Priority)
	// Enqueue with backpressure (drop when full)
	select {
	case q <- event:
		// update global monitor queue size on successful enqueue
		GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
	default:
		// queue full -> drop
		if b.metrics != nil {
			b.metrics.IncrementDropped()
		}
		GetGlobalMonitor().IncrementDropped()
		GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
		dropErr := fmt.Errorf("event dropped due to overflow: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		GetGlobalMonitor().SetError(dropErr)

		// Call error callback if configured
		if b.config.ErrorCallback != nil {
			b.config.ErrorCallback(event, "queue_overflow", dropErr)
		}

		if b.logger != nil {
			log.NewHelper(b.logger).Warnf("event bus overflow: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		}
	}
}

// Subscribe subscribes to events on this bus
func (b *LynxEventBus) Subscribe(handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {} // Return no-op cancel function
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
		return func() {} // Return no-op cancel function
	}

	wrapped := b.wrapHandler(handler)
	cancel := kelindarEvent.SubscribeTo(b.dispatcher, uint32(eventType), wrapped)
	b.subscriberCount.Add(1)
	// track per-type subscriptions
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

// SubscribeWithFilter subscribes with a filter predicate; handler is called only when filter returns true
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
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
	}
}

// SubscribeToWithFilter subscribes to specific event type with a filter
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
		// wait run() to exit after best-effort drain
		b.wg.Wait()
		// release worker pool
		if b.workerPool != nil {
			b.workerPool.Release()
		}
		return b.dispatcher.Close()
	}
	return nil
}

// IsClosed returns whether the bus is closed
func (b *LynxEventBus) IsClosed() bool {
	return b.isClosed.Load()
}

// GetBusType returns the bus type
func (b *LynxEventBus) GetBusType() BusType {
	return b.busType
}

// GetConfig returns the bus configuration
func (b *LynxEventBus) GetConfig() BusConfig {
	return b.config
}

// GetHistory returns the event history (if enabled)
func (b *LynxEventBus) GetHistory() *EventHistory {
	return b.history
}

// GetMetrics returns the event metrics (if enabled)
func (b *LynxEventBus) GetMetrics() *EventMetrics {
	return b.metrics
}

// SetLogger sets the logger for this bus
func (b *LynxEventBus) SetLogger(logger log.Logger) {
	b.logger = logger
}

// GetLogger returns the logger for this bus
func (b *LynxEventBus) GetLogger() log.Logger {
	return b.logger
}

// GetSubscriberCount returns the number of subscribers for a specific event type
func (b *LynxEventBus) GetSubscriberCount(eventType EventType) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if c, ok := b.typeSubs[eventType]; ok {
		return c
	}
	return 0
}

// GetTotalSubscriberCount returns the total number of subscribers across all event types
func (b *LynxEventBus) GetTotalSubscriberCount() int {
	// This is a simplified implementation
	// In a real implementation, you might want to track this separately
	return int(b.subscriberCount.Load())
}

// IsHealthy returns whether the bus is healthy
func (b *LynxEventBus) IsHealthy() bool {
	if b.isClosed.Load() {
		return false
	}
	if b.paused.Load() {
		// paused is considered unhealthy for external alerting
		return false
	}
	if b.isDegraded.Load() {
		// degraded is considered unhealthy
		return false
	}
	// mark unhealthy if queue usage exceeds 80% capacity
	totalCap := b.totalQueueCap()
	if totalCap > 0 {
		if b.totalQueueSize()*100/totalCap >= 80 {
			return false
		}
	}
	return true
}

// checkDegradation checks if degradation should be triggered
func (b *LynxEventBus) checkDegradation() {
	if !b.config.EnableDegradation {
		return
	}

	totalCap := b.totalQueueCap()
	if totalCap <= 0 {
		return
	}

	usagePercent := b.totalQueueSize() * 100 / totalCap
	if usagePercent >= b.config.DegradationThreshold {
		b.triggerDegradation()
	} else if b.isDegraded.Load() {
		b.clearDegradation()
	}
}

// triggerDegradation triggers degradation mode
func (b *LynxEventBus) triggerDegradation() {
	if b.isDegraded.CompareAndSwap(false, true) {
		b.degradationStartTime = time.Now()

		// Emit degradation event
		degradationEvent := LynxEvent{
			EventType: EventSystemError,
			Priority:  PriorityHigh,
			Source:    "event-bus",
			Category:  "system",
			PluginID:  "system",
			Status:    "degraded",
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"bus_type": b.busType,
				"mode":     b.config.DegradationMode,
				"reason":   "queue_overflow",
			},
		}

		// Try to publish degradation event (best effort)
		if manager := GetGlobalEventBus(); manager != nil {
			_ = manager.PublishEvent(degradationEvent)
		}

		// Apply degradation strategy
		switch b.config.DegradationMode {
		case DegradationModePause:
			b.Pause()
		case DegradationModeThrottle:
			// TODO: Implement throttling
		}
	}
}

// clearDegradation clears degradation mode
func (b *LynxEventBus) clearDegradation() {
	if b.isDegraded.CompareAndSwap(true, false) {
		// Emit recovery event
		recoveryEvent := LynxEvent{
			EventType: EventSystemError,
			Priority:  PriorityNormal,
			Source:    "event-bus",
			Category:  "system",
			PluginID:  "system",
			Status:    "recovered",
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"bus_type": b.busType,
				"duration": time.Since(b.degradationStartTime).String(),
				"reason":   "queue_normalized",
			},
		}

		// Try to publish recovery event (best effort)
		if manager := GetGlobalEventBus(); manager != nil {
			_ = manager.PublishEvent(recoveryEvent)
		}

		// Resume if was paused
		if b.config.DegradationMode == DegradationModePause && b.IsPaused() {
			b.Resume()
		}
	}
}

// IsDegraded returns whether the bus is in degradation mode
func (b *LynxEventBus) IsDegraded() bool {
	return b.isDegraded.Load()
}

// GetQueueSize returns the current queue size (approximate)
func (b *LynxEventBus) GetQueueSize() int {
	return b.totalQueueSize()
}

// run drains the queue and publishes to the dispatcher
func (b *LynxEventBus) run() {
	ticker := time.NewTicker(b.config.FlushInterval)
	defer ticker.Stop()
	defer b.wg.Done()
	for {
		select {
		case <-b.done:
			// best-effort drain remaining items before exit
			drainDeadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(drainDeadline) {
				drained := false
				// batch drain with priority fill
				bs := max(b.config.BatchSize, 1)
				buf := b.bufferPool.GetWithCapacity(bs)
				// Return to pool immediately after processing
				defer func() {
					b.bufferPool.Put(buf)
				}()
				for len(buf) < bs {
					if ev, ok := b.tryRecv(b.criticalQ); ok {
						buf = append(buf, ev)
						continue
					}
					if ev, ok := b.tryRecv(b.highQ); ok {
						buf = append(buf, ev)
						continue
					}
					if ev, ok := b.tryRecv(b.normalQ); ok {
						buf = append(buf, ev)
						continue
					}
					if ev, ok := b.tryRecv(b.lowQ); ok {
						buf = append(buf, ev)
						continue
					}
					break
				}
				if len(buf) > 0 {
					b.publishBatch(buf)
					drained = true
				}
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
				if !drained {
					return
				}
			}
			return
		case <-ticker.C:
			// periodic heartbeat, update monitor queue size
			GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
		default:
			if b.paused.Load() {
				// When paused, do not consume queue, only maintain heartbeat monitoring
				time.Sleep(10 * time.Millisecond)
				continue
			}
			// Try to batch collect non-blocking events, fill by priority
			batchSize := max(b.config.BatchSize, 1)
			buf := b.bufferPool.GetWithCapacity(batchSize)
			// Return to pool immediately after processing
			defer func() {
				b.bufferPool.Put(buf)
			}()
			for len(buf) < batchSize {
				if ev, ok := b.tryRecv(b.criticalQ); ok {
					buf = append(buf, ev)
					continue
				}
				if ev, ok := b.tryRecv(b.highQ); ok {
					buf = append(buf, ev)
					continue
				}
				if ev, ok := b.tryRecv(b.normalQ); ok {
					buf = append(buf, ev)
					continue
				}
				if ev, ok := b.tryRecv(b.lowQ); ok {
					buf = append(buf, ev)
					continue
				}
				break
			}
			if len(buf) > 0 {
				b.publishBatch(buf)
				continue
			}
			// block until any event arrives or ticker/done
			select {
			case <-b.done:
				continue
			case <-ticker.C:
				GetGlobalMonitor().UpdateQueueSize(b.totalQueueSize())
			case ev := <-b.criticalQ:
				buf := b.bufferPool.GetWithCapacity(max(b.config.BatchSize, 1))
				buf = append(buf, ev)
				// Return to pool immediately after processing
				defer func() {
					b.bufferPool.Put(buf)
				}()
				// Additional non-blocking fill
				for len(buf) < max(b.config.BatchSize, 1) {
					if ev2, ok := b.tryRecv(b.criticalQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.highQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.normalQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.lowQ); ok {
						buf = append(buf, ev2)
						continue
					}
					break
				}
				b.publishBatch(buf)
			case ev := <-b.highQ:
				buf := b.bufferPool.GetWithCapacity(max(b.config.BatchSize, 1))
				buf = append(buf, ev)
				// Return to pool immediately after processing
				defer func() {
					b.bufferPool.Put(buf)
				}()
				for len(buf) < max(b.config.BatchSize, 1) {
					if ev2, ok := b.tryRecv(b.highQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.normalQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.lowQ); ok {
						buf = append(buf, ev2)
						continue
					}
					break
				}
				b.publishBatch(buf)
			case ev := <-b.normalQ:
				buf := b.bufferPool.GetWithCapacity(max(b.config.BatchSize, 1))
				buf = append(buf, ev)
				// Return to pool immediately after processing
				defer func() {
					b.bufferPool.Put(buf)
				}()
				for len(buf) < max(b.config.BatchSize, 1) {
					if ev2, ok := b.tryRecv(b.normalQ); ok {
						buf = append(buf, ev2)
						continue
					}
					if ev2, ok := b.tryRecv(b.lowQ); ok {
						buf = append(buf, ev2)
						continue
					}
					break
				}
				b.publishBatch(buf)
			case ev := <-b.lowQ:
				buf := b.bufferPool.GetWithCapacity(max(b.config.BatchSize, 1))
				buf = append(buf, ev)
				// Return to pool immediately after processing
				defer func() {
					b.bufferPool.Put(buf)
				}()
				for len(buf) < max(b.config.BatchSize, 1) {
					if ev2, ok := b.tryRecv(b.lowQ); ok {
						buf = append(buf, ev2)
						continue
					}
					break
				}
				b.publishBatch(buf)
			}
		}
	}
}

// Pause stops consuming events from internal queues (publishing still enqueues)
func (b *LynxEventBus) Pause() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.paused.Load() {
		b.pauseStartTime = time.Now()
		b.paused.Store(true)
		b.pauseCount.Add(1)

		// Emit pause event
		pauseEvent := LynxEvent{
			EventType: EventSystemError,
			Priority:  PriorityHigh,
			Source:    "event-bus",
			Category:  "system",
			PluginID:  "system",
			Status:    "paused",
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"bus_type": b.busType,
				"reason":   "manual_pause",
			},
		}

		// Try to publish pause event (best effort)
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

		// Emit resume event
		resumeEvent := LynxEvent{
			EventType: EventSystemError,
			Priority:  PriorityNormal,
			Source:    "event-bus",
			Category:  "system",
			PluginID:  "system",
			Status:    "resumed",
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"bus_type":       b.busType,
				"pause_duration": b.pauseDuration.String(),
				"reason":         "manual_resume",
			},
		}

		// Try to publish resume event (best effort)
		if manager := GetGlobalEventBus(); manager != nil {
			_ = manager.PublishEvent(resumeEvent)
		}
	}
}

// IsPaused returns whether the bus is paused
func (b *LynxEventBus) IsPaused() bool { return b.paused.Load() }

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
func (b *LynxEventBus) GetPauseCount() int64 {
	return b.pauseCount.Load()
}

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
	// ants v2 exposes Cap(), Running(), Free(), Waiting()
	return pool.Cap(), pool.Running(), pool.Free(), pool.Waiting()
}

// UpdateConfig applies a subset of configuration at runtime (non-destructive)
// Currently supports MaxRetries and HistorySize (when history enabled)
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
	// Note: Changing FlushInterval/MaxQueue requires rebuilding the run loop/queue, not currently supported online
}

// wrapHandler wraps user handler to collect metrics & latency and recover panics
func (b *LynxEventBus) wrapHandler(handler func(LynxEvent)) func(LynxEvent) {
	return func(ev LynxEvent) {
		var attempts int64
		start := time.Now()
		for {
			attempts++
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
				GetGlobalMonitor().SetError(fmt.Errorf("handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempts))
				if b.logger != nil {
					log.NewHelper(b.logger).Errorf("event handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempts)
				}

				maxRetries := max(b.config.MaxRetries, 0)
				if attempts <= int64(maxRetries) {
					// Improved exponential backoff with jitter and max delay
					backoffDelay := calculateBackoffDelay(int(attempts), maxRetries)
					time.Sleep(backoffDelay)
					continue
				}

				// emit error event (DLQ-like)
				b.emitErrorEvent(ev, int(attempts))
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
			return
		}
	}
}

// helper: choose queue by priority
func (b *LynxEventBus) queueByPriority(p Priority) chan LynxEvent {
	switch p {
	case PriorityCritical:
		return b.criticalQ
	case PriorityHigh:
		return b.highQ
	case PriorityLow:
		return b.lowQ
	default:
		return b.normalQ
	}
}

// helper: total queue size
func (b *LynxEventBus) totalQueueSize() int {
	return len(b.lowQ) + len(b.normalQ) + len(b.highQ) + len(b.criticalQ)
}

// helper: total queue capacity
func (b *LynxEventBus) totalQueueCap() int {
	return cap(b.lowQ) + cap(b.normalQ) + cap(b.highQ) + cap(b.criticalQ)
}

// helper: non-blocking receive
func (b *LynxEventBus) tryRecv(q chan LynxEvent) (LynxEvent, bool) {
	select {
	case ev := <-q:
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

// helper: emit a system error event to act as DLQ
func (b *LynxEventBus) emitErrorEvent(original LynxEvent, attempts int) {
	m := b.metadataPool.Get()
	defer b.metadataPool.Put(m)

	m["bus_type"] = b.busType
	m["event_type"] = original.EventType
	m["attempts"] = attempts
	m["reason"] = "handler panic"
	// copy some metadata if present
	for k, v := range original.Metadata {
		// don't overwrite our keys
		if k == "bus_type" || k == "event_type" || k == "attempts" || k == "reason" {
			continue
		}
		if m["metadata"] == nil {
			m["metadata"] = map[string]any{}
		}
		mmeta := m["metadata"].(map[string]any)
		mmeta[k] = v
	}
	errEv := NewLynxEvent(EventErrorOccurred, original.PluginID, original.Source).WithPriority(PriorityHigh)
	errEv.Category = original.Category
	errEv.Metadata = m
	// best effort publish via global manager if available
	if manager := GetGlobalEventBus(); manager != nil {
		_ = manager.PublishEvent(errEv)
	}
}

// max returns the maximum of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
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
