package events

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// run drains the queue and publishes to the dispatcher.
func (b *LynxEventBus) run() {
	cfg := b.configSnapshot()
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = DefaultBusConfig().FlushInterval
	}
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	defer b.wg.Done()

	for {
		select {
		case <-b.done:
			drainDeadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(drainDeadline) {
				drained := false
				bs := max(b.configSnapshot().BatchSize, 1)
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
				b.monitor().UpdateQueueSize(b.totalQueueSize())
				if !drained {
					return
				}
			}
			return
		case <-ticker.C:
			b.monitor().UpdateQueueSize(b.totalQueueSize())
		default:
			if b.paused.Load() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			batchSize := max(b.configSnapshot().BatchSize, 1)
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

			select {
			case <-b.done:
				continue
			case <-ticker.C:
				b.monitor().UpdateQueueSize(b.totalQueueSize())
			case ev := <-b.queue:
				b.queueSize.Add(-1)
				batchSize := max(b.configSnapshot().BatchSize, 1)
				buf := b.bufferPool.GetWithCapacity(batchSize)
				buf = append(buf, ev)
				for len(buf) < batchSize {
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
