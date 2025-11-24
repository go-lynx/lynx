package events

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
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
		// Close done channel to signal all goroutines to stop
		close(b.done)

		// Use configurable timeout instead of hardcoded value
		closeTimeout := b.config.CloseTimeout
		if closeTimeout <= 0 {
			closeTimeout = 30 * time.Second // Default timeout
		}

		// Wait for all goroutines to finish with timeout
		// Optimized: Use buffered channel and context for better cleanup
		done := make(chan struct{}, 1) // Buffered to prevent goroutine leak
		ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()

		// Track goroutine count before wait
		goroutinesBefore := runtime.NumGoroutine()

		// Start goroutine to wait for WaitGroup
		go func() {
			defer func() {
				// Always signal completion to prevent goroutine leak
				select {
				case done <- struct{}{}:
				default:
				}
			}()
			b.wg.Wait()
		}()

		select {
		case <-done:
			// All goroutines finished successfully
			if b.logger != nil {
				log.NewHelper(b.logger).Infof("event bus closed successfully, all goroutines finished")
			}
		case <-ctx.Done():
			// Timeout: log warning with detailed information and attempt force cleanup
			goroutinesAfter := runtime.NumGoroutine()
			leakedGoroutines := goroutinesAfter - goroutinesBefore
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf(
					"event bus close timeout after %v: %d goroutines may still be running (before: %d, after: %d), forcing cleanup",
					closeTimeout, leakedGoroutines, goroutinesBefore, goroutinesAfter)
			}
			// Force close dispatcher to prevent further event processing
			if b.dispatcher != nil {
				if err := b.dispatcher.Close(); err != nil {
					if b.logger != nil {
						log.NewHelper(b.logger).Errorf("failed to force close dispatcher: %v", err)
					}
				}
			}
			// Ensure done goroutine exits by draining channel
			select {
			case <-done:
				// Goroutine completed
			default:
				// Goroutine may still be running, but we've timed out
			}
		}

		// Release worker pool
		if b.workerPool != nil {
			b.workerPool.Release()
		}

		// Clear processed events map
		b.processedEvents.Range(func(key, value interface{}) bool {
			b.processedEvents.Delete(key)
			return true
		})

		// Close dispatcher
		if b.dispatcher != nil {
			return b.dispatcher.Close()
		}
		return nil
	}
	return nil
}

// cleanupProcessedEvents periodically cleans up old entries from processed events map
// Optimized: More frequent cleanup to prevent memory growth, with size-based early cleanup
func (b *LynxEventBus) cleanupProcessedEvents() {
	defer b.wg.Done()
	// Use shorter ticker interval for more frequent cleanup (half of dedup window)
	cleanupInterval := b.dedupWindow / 2
	if cleanupInterval < 30*time.Second {
		cleanupInterval = 30 * time.Second // Minimum 30 seconds
	}
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	// Track cleanup statistics for monitoring
	maxCleanupCount := 0

	for {
		select {
		case <-b.done:
			// Final cleanup before exit
			now := time.Now()
			cleaned := 0
			b.processedEvents.Range(func(key, value interface{}) bool {
				if lastTime, ok := value.(time.Time); ok {
					if now.Sub(lastTime) > b.dedupWindow {
						b.processedEvents.Delete(key)
						cleaned++
					}
				}
				return true
			})
			if cleaned > 0 && b.logger != nil {
				log.NewHelper(b.logger).Debugf("final cleanup of processed events: removed %d entries", cleaned)
			}
			return
		case <-ticker.C:
			now := time.Now()
			cleaned := 0
			// Count entries first to decide if cleanup is needed
			entryCount := 0
			b.processedEvents.Range(func(key, value interface{}) bool {
				entryCount++
				return true
			})
			
			// Only perform cleanup if there are entries
			if entryCount > 0 {
				b.processedEvents.Range(func(key, value interface{}) bool {
					if lastTime, ok := value.(time.Time); ok {
						if now.Sub(lastTime) > b.dedupWindow {
							b.processedEvents.Delete(key)
							cleaned++
						}
					}
					return true
				})
				
				// Track max entries for monitoring
				if entryCount > maxCleanupCount {
					maxCleanupCount = entryCount
				}
				
				// If cleaned many entries or map is growing, log warning
				if cleaned > 1000 || (entryCount > 10000 && cleaned > 0) {
					if b.logger != nil {
						log.NewHelper(b.logger).Warnf("processed events map cleanup: removed %d entries, remaining %d (max seen: %d)",
							cleaned, entryCount-cleaned, maxCleanupCount)
					}
				}
			}
		}
	}
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
		pauseEvent := NewLynxEvent(EventSystemError, "system", "event-bus").
			WithPriority(PriorityHigh).
			WithCategory("system").
			WithStatus("paused").
			WithMetadata("bus_type", b.busType).
			WithMetadata("reason", "manual_pause")
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
		resumeEvent := NewLynxEvent(EventSystemError, "system", "event-bus").
			WithPriority(PriorityNormal).
			WithCategory("system").
			WithStatus("resumed").
			WithMetadata("bus_type", b.busType).
			WithMetadata("pause_duration", b.pauseDuration.String()).
			WithMetadata("reason", "manual_resume")
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
	b.checkDegradationWithSize(b.totalQueueSize(), capTotal)
}

// checkDegradationWithSize checks degradation using pre-calculated queue size and capacity
// This avoids redundant queue size checks for better performance
func (b *LynxEventBus) checkDegradationWithSize(queueSize, queueCap int) {
	thr := b.config.DegradationThreshold
	if queueCap <= 0 || thr <= 0 {
		return
	}
	usage := queueSize * 100 / queueCap

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
				// Update monitor less frequently during drain
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
				// Decrement queue size when receiving from channel
				b.queueSize.Add(-1)
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
// Optimized: Skip sorting for very small batches to reduce overhead
func orderByWeightedPriority(in []LynxEvent) []LynxEvent {
	if len(in) <= 1 {
		return in
	}
	
	// For very small batches (<=4), skip sorting to reduce overhead
	// The overhead of sorting may exceed the benefit for tiny batches
	if len(in) <= 4 {
		// Quick check: if already sorted by priority, return as-is
		allSamePriority := true
		firstPriority := in[0].Priority
		for i := 1; i < len(in); i++ {
			if in[i].Priority != firstPriority {
				allSamePriority = false
				break
			}
		}
		if allSamePriority {
			return in
		}
		// For small mixed batches, use simple in-place swap instead of full sort
		// This is faster than full partitioning for tiny batches
		for i := 0; i < len(in)-1; i++ {
			for j := i + 1; j < len(in); j++ {
				if in[i].Priority < in[j].Priority {
					in[i], in[j] = in[j], in[i]
				}
			}
		}
		return in
	}
	
	// For larger batches, use optimized partitioning approach
	// Count events by priority to pre-allocate slices
	var critCount, highCount, normCount, lowCount int
	for i := range in {
		switch in[i].Priority {
		case PriorityCritical:
			critCount++
		case PriorityHigh:
			highCount++
		case PriorityLow:
			lowCount++
		default:
			normCount++
		}
	}
	
	// Pre-allocate slices with known capacity
	crit := make([]LynxEvent, 0, critCount)
	high := make([]LynxEvent, 0, highCount)
	norm := make([]LynxEvent, 0, normCount)
	low := make([]LynxEvent, 0, lowCount)
	
	// Partition events by priority
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
	
	// Use pre-allocated output slice
	out := make([]LynxEvent, 0, len(in))
	wc, wh, wn, wl := 8, 4, 2, 1
	ic, ih, inx, il := 0, 0, 0, 0
	
	// Weighted round-robin distribution
	for ic < len(crit) || ih < len(high) || inx < len(norm) || il < len(low) {
		// Critical priority: highest weight
		for k := 0; k < wc && ic < len(crit); k++ {
			out = append(out, crit[ic])
			ic++
		}
		// High priority
		for k := 0; k < wh && ih < len(high); k++ {
			out = append(out, high[ih])
			ih++
		}
		// Normal priority
		for k := 0; k < wn && inx < len(norm); k++ {
			out = append(out, norm[inx])
			inx++
		}
		// Low priority: lowest weight
		for k := 0; k < wl && il < len(low); k++ {
			out = append(out, low[il])
			il++
		}
		// Gradually reduce weights to ensure fairness
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
// Use atomic counter for better performance under high concurrency
func (b *LynxEventBus) totalQueueSize() int { 
	size := int(b.queueSize.Load())
	// Cap the reported size at actual capacity to handle overflow cases
	capVal := cap(b.queue)
	if size > capVal {
		return capVal
	}
	return size
}

// helper: total queue capacity (single shared queue)
func (b *LynxEventBus) totalQueueCap() int { return cap(b.queue) }

// helper: non-blocking receive from shared queue
func (b *LynxEventBus) tryRecvShared() (LynxEvent, bool) {
	select {
	case ev := <-b.queue:
		// Decrement queue size atomically
		b.queueSize.Add(-1)
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
	// Atomic counter for queue size to avoid expensive len() calls
	queueSize atomic.Int64
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
	// Performance optimization: sample degradation checks instead of checking every event
	lastDegradationCheck atomic.Int64 // Unix timestamp in nanoseconds

	// Throttling for degradation mode
	throttler *SimpleThrottler

	// Worker pool for parallel dispatching
	workerPool *ants.Pool

	// Retry bounding (limit concurrent scheduled retries)
	retrySem chan struct{}

	// Memory optimization
	bufferPool   *EventBufferPool
	metadataPool *MetadataPool

	// Event deduplication
	processedEvents sync.Map // map[string]time.Time - eventID -> processing timestamp
	dedupWindow     time.Duration
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

	// Initialize deduplication (default: 5 minutes window)
	bus.dedupWindow = 5 * time.Minute

	// Start worker to drain queue and publish to dispatcher
	bus.wg.Add(1)
	go bus.run()

	// Start cleanup goroutine for processed events map
	bus.wg.Add(1)
	go bus.cleanupProcessedEvents()

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

	// Update metrics (use atomic operations where possible to reduce lock contention)
	if b.metrics != nil {
		b.metrics.IncrementPublished()
	}
	// Update global monitor (bucketed by priority) - uses atomic operations internally
	GetGlobalMonitor().IncrementPublishedByPriority(event.Priority)

	// Add to history if enabled (non-blocking, uses internal locks)
	if b.history != nil {
		b.history.Add(event)
	}

	// Check degradation before publishing (adaptive sampling to reduce overhead)
	// Use adaptive check interval based on queue usage with probabilistic sampling:
	// - Low usage (<50%): check every 500ms or 1% probability
	// - Medium usage (50-80%): check every 100ms or 5% probability
	// - High usage (>80%): check every 10ms or 20% probability
	queueSize := int(b.queueSize.Load())
	queueCap := b.totalQueueCap()
	queueUsage := queueSize * 100 / max(queueCap, 1)
	
	// Fast path: skip check if queue is very empty (<25%)
	if queueUsage < 25 {
		// Only check periodically even when queue is empty
		now := time.Now().UnixNano()
		lastCheck := b.lastDegradationCheck.Load()
		if now-lastCheck > int64(1*time.Second) {
			if b.lastDegradationCheck.CompareAndSwap(lastCheck, now) {
				b.checkDegradation()
			}
		}
	} else {
		// Use probabilistic sampling to reduce overhead at high event rates
		now := time.Now().UnixNano()
		lastCheck := b.lastDegradationCheck.Load()
		
		var checkInterval int64
		var checkProbability float64
		if queueUsage < 50 {
			checkInterval = int64(500 * time.Millisecond)
			checkProbability = 0.01 // 1% chance
		} else if queueUsage < 80 {
			checkInterval = int64(100 * time.Millisecond)
			checkProbability = 0.05 // 5% chance
		} else {
			checkInterval = int64(10 * time.Millisecond)
			checkProbability = 0.20 // 20% chance
		}

		// Check if interval passed or random sample triggers check
		shouldCheck := (now-lastCheck > checkInterval) || (rand.Float64() < checkProbability)
		
		// Always check if queue is getting very full (>90%)
		if queueUsage > 90 {
			shouldCheck = true
		}
		
		if shouldCheck {
			if b.lastDegradationCheck.CompareAndSwap(lastCheck, now) {
				b.checkDegradation()
			}
		}
	}

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
		// Increment queue size atomically
		newSize := b.queueSize.Add(1)
		GetGlobalMonitor().UpdateQueueSize(int(newSize))
	default:
		// queue full - apply backpressure strategy
		switch b.config.DropPolicy {
		case DropBlock:
			timeout := b.config.EnqueueBlockTimeout
			if timeout <= 0 {
				timeout = 5 * time.Millisecond
			}
			// For critical events, use longer timeout
			if event.Priority == PriorityCritical {
				timeout = timeout * 2
			}

			// Use context for better cancellation support
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			select {
			case b.queue <- event:
				// Increment queue size atomically
				newSize := b.queueSize.Add(1)
				GetGlobalMonitor().UpdateQueueSize(int(newSize))
			case <-ctx.Done():
				// Timeout reached - check if we should still try to enqueue critical events
				if event.Priority == PriorityCritical {
					// For critical events, try one more time with non-blocking attempt
					select {
					case b.queue <- event:
						// Increment queue size atomically
						newSize := b.queueSize.Add(1)
						GetGlobalMonitor().UpdateQueueSize(int(newSize))
					default:
						b.handleEnqueueOverflow(event, "block_timeout_critical")
					}
				} else {
					b.handleEnqueueOverflow(event, "block_timeout")
				}
			}
		case DropOldest:
			// Drop one oldest item to make room, then enqueue
			// Use non-blocking select to avoid deadlock
			dropped := false
			select {
			case <-b.queue:
				// Decrement queue size when dropping
				b.queueSize.Add(-1)
				dropped = true
			default:
				// Queue might have been consumed, try direct enqueue
			}

			// Try to enqueue with timeout
			enqueueCtx, enqueueCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer enqueueCancel()

			select {
			case b.queue <- event:
				// Increment queue size atomically (oldest was already decremented)
				newSize := b.queueSize.Add(1)
				GetGlobalMonitor().UpdateQueueSize(int(newSize))
				if dropped && b.logger != nil {
					log.NewHelper(b.logger).Debugf("dropped oldest event to make room for new event")
				}
			case <-enqueueCtx.Done():
				// Still full after dropping, handle overflow
				b.handleEnqueueOverflow(event, "drop_oldest_failed")
			}
		default: // DropNewest (or unset)
			// For critical events, try to force enqueue by dropping non-critical
			if event.Priority == PriorityCritical {
				// Try to drop a non-critical event
				dropped := false
			dropLoop:
				for i := 0; i < 3 && !dropped; i++ {
					select {
					case oldEvent := <-b.queue:
						// Decrement queue size when removing event
						b.queueSize.Add(-1)
						if oldEvent.Priority != PriorityCritical {
							dropped = true
							if b.logger != nil {
								log.NewHelper(b.logger).Debugf("dropped non-critical event to make room for critical event")
							}
							break dropLoop
						} else {
							// Put it back, try another
							select {
							case b.queue <- oldEvent:
								// Increment size back
								b.queueSize.Add(1)
							default:
								// Queue full, can't put back (size already decremented)
							}
						}
					default:
						break dropLoop
					}
				}

				// Try to enqueue critical event
				select {
				case b.queue <- event:
					// Increment queue size atomically (dropped event already decremented)
					newSize := b.queueSize.Add(1)
					GetGlobalMonitor().UpdateQueueSize(int(newSize))
				default:
					b.handleEnqueueOverflow(event, "drop_newest_critical_failed")
				}
			} else {
				b.handleEnqueueOverflow(event, "drop_newest")
			}
		}
	}
}

// UpdateConfig applies a subset of configuration at runtime (non-destructive)
// Supports MaxRetries, HistorySize (when history enabled), WorkerCount, MaxConcurrentRetries,
// Degradation hysteresis, Throttling settings, Drop policy and critical reserve
// Uses atomic configuration update to prevent inconsistent states
func (b *LynxEventBus) UpdateConfig(cfg BusConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Create a snapshot of current config for rollback if needed
	oldConfig := b.config
	var rollbackNeeded bool
	defer func() {
		if rollbackNeeded {
			b.config = oldConfig
		}
	}()

	// MaxRetries can be changed directly
	b.config.MaxRetries = cfg.MaxRetries
	// BatchSize hot update
	if cfg.BatchSize > 0 && cfg.BatchSize != b.config.BatchSize {
		b.config.BatchSize = cfg.BatchSize
	}
	// WorkerCount can be tuned dynamically if pool exists
	if b.workerPool != nil && cfg.WorkerCount > 0 && cfg.WorkerCount != b.config.WorkerCount {
		// ants v2 supports Tune to resize pool at runtime
		// Tune doesn't return error, but we validate the size is reasonable
		if cfg.WorkerCount > 0 && cfg.WorkerCount <= 10000 {
			b.workerPool.Tune(cfg.WorkerCount)
			b.config.WorkerCount = cfg.WorkerCount
		} else {
			rollbackNeeded = true
			if b.logger != nil {
				log.NewHelper(b.logger).Errorf("Invalid worker pool size: %d (must be 1-10000)", cfg.WorkerCount)
			}
			return
		}
	}
	// History size can be changed by recreating history buffer
	// Preserve existing events when possible
	if b.config.EnableHistory {
		size := cfg.HistorySize
		if size <= 0 {
			size = b.config.HistorySize
		}
		if size <= 0 {
			size = 1000
		}
		if size != b.config.HistorySize {
			// Preserve existing events if possible
			var existingEvents []LynxEvent
			if b.history != nil {
				existingEvents = b.history.GetEvents()
			}
			newHistory := NewEventHistory(size)
			// Restore events up to new size limit
			if len(existingEvents) > 0 {
				startIdx := 0
				if len(existingEvents) > size {
					startIdx = len(existingEvents) - size
				}
				for i := startIdx; i < len(existingEvents); i++ {
					newHistory.Add(existingEvents[i])
				}
			}
			b.history = newHistory
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
	// Check for duplicate events (deduplication)
	// Only check deduplication for first attempt to allow retries
	shouldCheckDedup := attempt == 0 && ev.EventID != ""
	if shouldCheckDedup {
		now := time.Now()
		if lastProcessed, ok := b.processedEvents.Load(ev.EventID); ok {
			if lastTime, ok := lastProcessed.(time.Time); ok {
				if now.Sub(lastTime) < b.dedupWindow {
					// Event was recently processed, skip to prevent duplicate handling
					if b.logger != nil {
						log.NewHelper(b.logger).Debugf("duplicate event detected and skipped: eventID=%s, lastProcessed=%v", ev.EventID, lastTime)
					}
					return
				}
			}
		}
	}

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

	// Only mark as processed if handler executed successfully (no panic)
	// This allows retries to proceed even if the event was previously marked
	if !panicked && ev.EventID != "" {
		b.processedEvents.Store(ev.EventID, time.Now())
	}

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
					// Use timer with proper cleanup to prevent goroutine leaks
					timer := time.NewTimer(backoffDelay)
					go func() {
						defer func() {
							// Always stop timer to prevent resource leak
							if !timer.Stop() {
								// Timer already fired, drain channel
								select {
								case <-timer.C:
								default:
								}
							}
							// Always release semaphore
							<-b.retrySem
							if r := recover(); r != nil {
								// Log panic but don't crash
								if b.logger != nil {
									log.NewHelper(b.logger).Errorf("panic in retry goroutine: %v", r)
								}
							}
						}()
						
						select {
						case <-timer.C:
							// Timer expired, proceed with retry
							if b.isClosed.Load() {
								return
							}
							if b.workerPool != nil {
								if err := b.workerPool.Submit(func() { b.handleWithRetry(ev, handler, attempt+1) }); err != nil {
									// Worker pool rejected, fallback to direct goroutine
									if !b.isClosed.Load() {
										go b.handleWithRetry(ev, handler, attempt+1)
									}
								}
							} else {
								if !b.isClosed.Load() {
									go b.handleWithRetry(ev, handler, attempt+1)
								}
							}
						case <-b.done:
							// Bus is closing, cancel retry (timer cleanup handled by defer)
							return
						}
					}()
				default:
					// Retry capacity exhausted; emit error and stop retrying this event
					if b.logger != nil {
						log.NewHelper(b.logger).Warnf("retry capacity exhausted, dropping retries: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt)
					}
					b.emitErrorEvent(ev, attempt)
				}
			} else {
				// Use timer with proper cleanup to prevent goroutine leaks
				timer := time.NewTimer(backoffDelay)
				go func() {
					defer func() {
						// Always stop timer to prevent resource leak
						if !timer.Stop() {
							// Timer already fired, drain channel
							select {
							case <-timer.C:
							default:
							}
						}
						if r := recover(); r != nil {
							// Log panic but don't crash
							if b.logger != nil {
								log.NewHelper(b.logger).Errorf("panic in retry goroutine: %v", r)
							}
						}
					}()
					
					select {
					case <-timer.C:
						// Timer expired, proceed with retry
						if b.isClosed.Load() {
							return
						}
						if b.workerPool != nil {
							if err := b.workerPool.Submit(func() { b.handleWithRetry(ev, handler, attempt+1) }); err != nil {
								// Worker pool rejected, fallback to direct goroutine
								if !b.isClosed.Load() {
									go b.handleWithRetry(ev, handler, attempt+1)
								}
							}
						} else {
							if !b.isClosed.Load() {
								go b.handleWithRetry(ev, handler, attempt+1)
							}
						}
					case <-b.done:
						// Bus is closing, cancel retry (timer cleanup handled by defer)
						return
					}
				}()
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
		if err := manager.PublishEvent(errEv); err != nil {
			// Log error instead of ignoring it
			if b.logger != nil {
				log.NewHelper(b.logger).Errorf("failed to publish error event: %v", err)
			}
		}
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
