package events

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// handleEnqueueOverflow applies unified drop handling, metrics and callback.
func (b *LynxEventBus) handleEnqueueOverflow(event LynxEvent, reason string) {
	if b.metrics != nil {
		b.metrics.IncrementDropped()
	}
	b.monitor().IncrementDroppedByReason(reason)
	b.monitor().UpdateQueueSize(b.totalQueueSize())
	dropErr := fmt.Errorf("event dropped due to overflow: bus=%d prio=%d type=%d reason=%s", b.busType, event.Priority, event.EventType, reason)
	b.monitor().SetError(dropErr)

	cfg := b.configSnapshot()
	if cfg.ErrorCallback != nil {
		cfg.ErrorCallback(event, reason, dropErr)
	}
	if b.logger != nil {
		log.NewHelper(b.logger).Warnf("event bus overflow: bus=%d prio=%d type=%d reason=%s", b.busType, event.Priority, event.EventType, reason)
	}
}

// handleWithRetry executes the handler with panic recovery and schedules retries asynchronously.
func (b *LynxEventBus) handleWithRetry(ev LynxEvent, handler func(LynxEvent), attempt int) {
	shouldCheckDedup := attempt == 0 && ev.EventID != ""
	if shouldCheckDedup {
		now := time.Now()
		if lastProcessed, ok := b.processedEvents.Load(ev.EventID); ok {
			if lastTime, ok := lastProcessed.(time.Time); ok && now.Sub(lastTime) < b.dedupWindow {
				if b.logger != nil {
					log.NewHelper(b.logger).Debugf("duplicate event detected and skipped: eventID=%s, lastProcessed=%v", ev.EventID, lastTime)
				}
				return
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

	if !panicked && ev.EventID != "" {
		b.processedEvents.Store(ev.EventID, time.Now())
	}

	duration := time.Since(start)
	if panicked {
		if b.metrics != nil {
			b.metrics.IncrementFailed()
		}
		b.monitor().IncrementFailed()
		b.monitor().SetError(fmt.Errorf("handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt))
		if b.logger != nil {
			log.NewHelper(b.logger).Errorf("event handler panic: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt)
		}

		maxRetries := max(b.configSnapshot().MaxRetries, 0)
		if attempt <= maxRetries {
			b.scheduleRetry(ev, handler, attempt, calculateBackoffDelay(attempt, maxRetries))
			return
		}

		b.emitErrorEvent(ev, attempt)
		if b.metrics != nil {
			b.metrics.UpdateLatency(duration)
		}
		b.monitor().UpdateLatency(duration)
		return
	}

	if b.metrics != nil {
		b.metrics.UpdateLatency(duration)
		b.metrics.IncrementProcessed()
	}
	b.monitor().UpdateLatency(duration)
	b.monitor().IncrementProcessed()
}

func (b *LynxEventBus) scheduleRetry(ev LynxEvent, handler func(LynxEvent), attempt int, backoffDelay time.Duration) {
	_, _, _, _, retrySem := b.runtimeSnapshot()
	if retrySem != nil {
		select {
		case retrySem <- struct{}{}:
			timer := time.NewTimer(backoffDelay)
			go func() {
				defer func() {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					<-retrySem
					if r := recover(); r != nil && b.logger != nil {
						log.NewHelper(b.logger).Errorf("panic in retry goroutine: %v", r)
					}
				}()

				b.executeRetry(timer, ev, handler, attempt)
			}()
		default:
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf("retry capacity exhausted, dropping retries: bus=%d type=%d attempt=%d", b.busType, ev.EventType, attempt)
			}
			b.emitErrorEvent(ev, attempt)
		}
		return
	}

	timer := time.NewTimer(backoffDelay)
	go func() {
		defer func() {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if r := recover(); r != nil && b.logger != nil {
				log.NewHelper(b.logger).Errorf("panic in retry goroutine: %v", r)
			}
		}()

		b.executeRetry(timer, ev, handler, attempt)
	}()
}

func (b *LynxEventBus) executeRetry(timer *time.Timer, ev LynxEvent, handler func(LynxEvent), attempt int) {
	select {
	case <-timer.C:
		if b.isClosed.Load() {
			return
		}
		_, _, _, workerPool, _ := b.runtimeSnapshot()
		if workerPool != nil {
			if err := workerPool.Submit(func() { b.handleWithRetry(ev, handler, attempt+1) }); err != nil {
				if !b.isClosed.Load() {
					go b.handleWithRetry(ev, handler, attempt+1)
				}
			}
			return
		}
		if !b.isClosed.Load() {
			go b.handleWithRetry(ev, handler, attempt+1)
		}
	case <-b.done:
		return
	}
}

// emitErrorEvent publishes a system error event to act as a DLQ-like signal.
func (b *LynxEventBus) emitErrorEvent(original LynxEvent, attempts int) {
	meta := make(map[string]any, 6)
	meta["bus_type"] = b.busType
	meta["event_type"] = original.EventType
	meta["attempts"] = attempts
	meta["reason"] = "handler panic"
	if len(original.Metadata) > 0 {
		copied := make(map[string]any, len(original.Metadata))
		for k, v := range original.Metadata {
			copied[k] = v
		}
		meta["metadata"] = copied
	}
	errEv := NewLynxEvent(EventErrorOccurred, original.PluginID, original.Source).WithPriority(PriorityHigh)
	errEv.Category = original.Category
	errEv.Metadata = meta
	if manager := b.manager; manager != nil {
		if err := manager.PublishEvent(errEv); err != nil && b.logger != nil {
			log.NewHelper(b.logger).Errorf("failed to publish error event: %v", err)
		}
	}
}

// calculateBackoffDelay calculates exponential backoff delay with jitter.
func calculateBackoffDelay(attempt, maxRetries int) time.Duration {
	baseDelay := time.Duration(10) * time.Millisecond * time.Duration(1<<(attempt-1))
	jitter := float64(baseDelay) * (0.75 + 0.5*rand.Float64())

	maxDelay := 5 * time.Second
	if time.Duration(jitter) > maxDelay {
		return maxDelay
	}

	return time.Duration(jitter)
}
