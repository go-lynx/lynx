package events

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// busMonitorMinInterval is the floor for the periodic queue-size monitoring
	// tick. Dispatch is event-driven (run blocks on b.queue), so the ticker only
	// needs to fire often enough to keep monitoring/metrics fresh. Without this
	// floor an idle bus would wake on every FlushInterval (which defaults to
	// microseconds across 8 buses), burning a full CPU core for no work.
	busMonitorMinInterval = 1 * time.Second
	// busShutdownDrainTimeout bounds how long run() spends draining queued events
	// after b.done is closed.
	busShutdownDrainTimeout = 200 * time.Millisecond
	// busPausePollInterval is how often a paused bus re-checks for resume. Paused
	// is an exceptional, short-lived state, so a small poll is acceptable.
	busPausePollInterval = 10 * time.Millisecond
)

// busMonitorInterval resolves the queue-size monitoring tick from a configured
// FlushInterval, falling back to the default and flooring at busMonitorMinInterval.
// The floor is what keeps an idle bus from waking thousands of times per second.
func busMonitorInterval(flushInterval time.Duration) time.Duration {
	if flushInterval <= 0 {
		flushInterval = DefaultBusConfig().FlushInterval
	}
	if flushInterval < busMonitorMinInterval {
		return busMonitorMinInterval
	}
	return flushInterval
}

// run drains the queue and publishes to the dispatcher.
//
// Dispatch is event-driven: the goroutine blocks on b.queue and wakes the moment
// an event is enqueued, so dispatch latency does not depend on any polling
// interval. The ticker only drives periodic queue-size monitoring, clamped to
// busMonitorMinInterval so an idle bus stays asleep instead of spinning.
func (b *LynxEventBus) run() {
	defer b.wg.Done()

	ticker := time.NewTicker(busMonitorInterval(b.configSnapshot().FlushInterval))
	defer ticker.Stop()

	for {
		// While paused, do not dispatch. Wait for resume, shutdown, or a monitor
		// tick so a paused bus does not drain events early.
		if b.paused.Load() {
			select {
			case <-b.done:
				b.drainOnShutdown()
				return
			case <-ticker.C:
				b.monitor().UpdateQueueSize(b.totalQueueSize())
			case <-time.After(busPausePollInterval):
			}
			continue
		}

		select {
		case <-b.done:
			b.drainOnShutdown()
			return
		case <-ticker.C:
			b.monitor().UpdateQueueSize(b.totalQueueSize())
		case ev := <-b.queue:
			b.queueSize.Add(-1)
			b.dispatchBatch(ev)
		}
	}
}

// dispatchBatch publishes first plus up to BatchSize-1 additional queued events
// in weighted-priority order, reusing a pooled buffer.
func (b *LynxEventBus) dispatchBatch(first LynxEvent) {
	batchSize := max(b.configSnapshot().BatchSize, 1)
	buf := b.bufferPool.GetWithCapacity(batchSize)
	buf = append(buf, first)
	for len(buf) < batchSize {
		ev, ok := b.tryRecvShared()
		if !ok {
			break
		}
		buf = append(buf, ev)
	}
	b.publishBatch(orderByWeightedPriority(buf))
	b.bufferPool.Put(buf)
}

// drainOnShutdown publishes any events still queued when b.done is closed,
// bounded by busShutdownDrainTimeout.
func (b *LynxEventBus) drainOnShutdown() {
	deadline := time.Now().Add(busShutdownDrainTimeout)
	for time.Now().Before(deadline) {
		batchSize := max(b.configSnapshot().BatchSize, 1)
		buf := b.bufferPool.GetWithCapacity(batchSize)
		for len(buf) < batchSize {
			ev, ok := b.tryRecvShared()
			if !ok {
				break
			}
			buf = append(buf, ev)
		}
		drained := len(buf) > 0
		if drained {
			b.publishBatch(buf)
		}
		b.bufferPool.Put(buf)
		b.monitor().UpdateQueueSize(b.totalQueueSize())
		if !drained {
			return
		}
	}
}

func (b *LynxEventBus) configSnapshot() BusConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

// UpdateConfig applies a subset of configuration at runtime (non-destructive).
func (b *LynxEventBus) UpdateConfig(cfg BusConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldConfig := b.config
	var rollbackNeeded bool
	defer func() {
		if rollbackNeeded {
			b.config = oldConfig
		}
	}()

	b.config.MaxRetries = cfg.MaxRetries
	if cfg.BatchSize > 0 && cfg.BatchSize != b.config.BatchSize {
		b.config.BatchSize = cfg.BatchSize
	}
	if b.workerPool != nil && cfg.WorkerCount > 0 && cfg.WorkerCount != b.config.WorkerCount {
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

	if b.config.EnableHistory {
		size := cfg.HistorySize
		if size <= 0 {
			size = b.config.HistorySize
		}
		if size <= 0 {
			size = 1000
		}
		if size != b.config.HistorySize {
			var existingEvents []LynxEvent
			if b.history != nil {
				existingEvents = b.history.GetEvents()
			}
			newHistory := NewEventHistory(size)
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

	if b.retrySem == nil && cfg.MaxConcurrentRetries > 0 {
		b.retrySem = make(chan struct{}, cfg.MaxConcurrentRetries)
	}
	if b.retrySem != nil && cfg.MaxConcurrentRetries <= 0 {
		b.retrySem = nil
	}
	b.config.MaxConcurrentRetries = cfg.MaxConcurrentRetries

	if cfg.DegradationRecoverThreshold >= 0 {
		b.config.DegradationRecoverThreshold = cfg.DegradationRecoverThreshold
	}

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
