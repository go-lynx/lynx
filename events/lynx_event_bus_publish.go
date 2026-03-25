package events

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kelindarEvent "github.com/kelindar/event"
)

// orderByWeightedPriority reorders the batch by weighted fair policy: critical>high>normal>low.
func orderByWeightedPriority(in []LynxEvent) []LynxEvent {
	if len(in) <= 1 {
		return in
	}

	if len(in) <= 4 {
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
		for i := 0; i < len(in)-1; i++ {
			for j := i + 1; j < len(in); j++ {
				if in[i].Priority < in[j].Priority {
					in[i], in[j] = in[j], in[i]
				}
			}
		}
		return in
	}

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

	crit := make([]LynxEvent, 0, critCount)
	high := make([]LynxEvent, 0, highCount)
	norm := make([]LynxEvent, 0, normCount)
	low := make([]LynxEvent, 0, lowCount)
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

func (b *LynxEventBus) totalQueueSize() int {
	size := int(b.queueSize.Load())
	capVal := cap(b.queue)
	if size > capVal {
		return capVal
	}
	return size
}

func (b *LynxEventBus) totalQueueCap() int { return cap(b.queue) }

func (b *LynxEventBus) tryRecvShared() (LynxEvent, bool) {
	select {
	case ev := <-b.queue:
		b.queueSize.Add(-1)
		return ev, true
	default:
		return LynxEvent{}, false
	}
}

func (b *LynxEventBus) publish(ev LynxEvent) {
	if b.workerPool != nil {
		if err := b.workerPool.Submit(func() { kelindarEvent.Publish(b.dispatcher, ev) }); err != nil {
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf("worker pool submit failed: %v", err)
			}
			b.monitor().SetError(fmt.Errorf("worker_pool_submit_failed: %v", err))
			kelindarEvent.Publish(b.dispatcher, ev)
		}
	} else {
		kelindarEvent.Publish(b.dispatcher, ev)
	}
	b.monitor().UpdateQueueSize(b.totalQueueSize())
}

func (b *LynxEventBus) publishBatch(events []LynxEvent) {
	if b.isDegraded.Load() && b.throttler != nil {
		throttledEvents := make([]LynxEvent, 0, len(events))
		for _, ev := range events {
			if b.throttler.Allow() {
				throttledEvents = append(throttledEvents, ev)
			} else if b.logger != nil {
				log.NewHelper(b.logger).Debugf("event throttled during degradation: type=%d, plugin=%s", ev.EventType, ev.PluginID)
			}
		}
		events = throttledEvents
	}

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
				b.monitor().SetError(fmt.Errorf("worker_pool_submit_failed: %v", err))
				kelindarEvent.Publish(b.dispatcher, ev)
			}
		}
		for i := limit; i < len(events); i++ {
			kelindarEvent.Publish(b.dispatcher, events[i])
		}
	} else {
		for i := range events {
			kelindarEvent.Publish(b.dispatcher, events[i])
		}
	}
	b.monitor().UpdateQueueSize(b.totalQueueSize())
}

// NewSimpleThrottler creates a new throttler.
func NewSimpleThrottler(rate, burst int) *SimpleThrottler {
	return &SimpleThrottler{
		rate:     rate,
		burst:    burst,
		tokens:   burst,
		lastTime: time.Now(),
	}
}

// Allow checks if an event is allowed to proceed.
func (t *SimpleThrottler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastTime)
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

// Publish publishes an event to this bus.
func (b *LynxEventBus) Publish(event LynxEvent) {
	if b.isClosed.Load() {
		return
	}

	if b.throttler != nil && !b.throttler.Allow() {
		if b.metrics != nil {
			b.metrics.IncrementDropped()
		}
		b.monitor().IncrementDroppedByReason("throttled")
		throttleErr := fmt.Errorf("event throttled: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		b.monitor().SetError(throttleErr)
		if b.config.ErrorCallback != nil {
			b.config.ErrorCallback(event, "throttled", throttleErr)
		}
		if b.logger != nil {
			log.NewHelper(b.logger).Warnf("event throttled: bus=%d prio=%d type=%d", b.busType, event.Priority, event.EventType)
		}
		return
	}

	if b.metrics != nil {
		b.metrics.IncrementPublished()
	}
	b.monitor().IncrementPublishedByPriority(event.Priority)
	if b.history != nil {
		b.history.Add(event)
	}

	queueSize := int(b.queueSize.Load())
	queueCap := b.totalQueueCap()
	queueUsage := queueSize * 100 / max(queueCap, 1)

	if queueUsage < 25 {
		now := time.Now().UnixNano()
		lastCheck := b.lastDegradationCheck.Load()
		if now-lastCheck > int64(time.Second) && b.lastDegradationCheck.CompareAndSwap(lastCheck, now) {
			b.checkDegradation()
		}
	} else {
		now := time.Now().UnixNano()
		lastCheck := b.lastDegradationCheck.Load()

		var checkInterval int64
		var checkProbability float64
		if queueUsage < 50 {
			checkInterval = int64(500 * time.Millisecond)
			checkProbability = 0.01
		} else if queueUsage < 80 {
			checkInterval = int64(100 * time.Millisecond)
			checkProbability = 0.05
		} else {
			checkInterval = int64(10 * time.Millisecond)
			checkProbability = 0.20
		}

		shouldCheck := (now-lastCheck > checkInterval) || (rand.Float64() < checkProbability)
		if queueUsage > 90 {
			shouldCheck = true
		}
		if shouldCheck && b.lastDegradationCheck.CompareAndSwap(lastCheck, now) {
			b.checkDegradation()
		}
	}

	if b.config.ReserveForCritical > 0 && event.Priority != PriorityCritical {
		if b.totalQueueSize() >= max(b.totalQueueCap()-b.config.ReserveForCritical, 0) {
			b.handleEnqueueOverflow(event, "reserve_for_critical")
			return
		}
	}

	select {
	case b.queue <- event:
		newSize := b.queueSize.Add(1)
		b.monitor().UpdateQueueSize(int(newSize))
	default:
		switch b.config.DropPolicy {
		case DropBlock:
			timeout := b.config.EnqueueBlockTimeout
			if timeout <= 0 {
				timeout = 5 * time.Millisecond
			}
			if event.Priority == PriorityCritical {
				timeout *= 2
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			select {
			case b.queue <- event:
				newSize := b.queueSize.Add(1)
				b.monitor().UpdateQueueSize(int(newSize))
			case <-ctx.Done():
				if event.Priority == PriorityCritical {
					select {
					case b.queue <- event:
						newSize := b.queueSize.Add(1)
						b.monitor().UpdateQueueSize(int(newSize))
					default:
						b.handleEnqueueOverflow(event, "block_timeout_critical")
					}
				} else {
					b.handleEnqueueOverflow(event, "block_timeout")
				}
			}
		case DropOldest:
			dropped := false
			select {
			case <-b.queue:
				b.queueSize.Add(-1)
				dropped = true
			default:
			}

			enqueueCtx, enqueueCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer enqueueCancel()

			select {
			case b.queue <- event:
				newSize := b.queueSize.Add(1)
				b.monitor().UpdateQueueSize(int(newSize))
				if dropped && b.logger != nil {
					log.NewHelper(b.logger).Debugf("dropped oldest event to make room for new event")
				}
			case <-enqueueCtx.Done():
				b.handleEnqueueOverflow(event, "drop_oldest_failed")
			}
		default:
			if event.Priority == PriorityCritical {
				dropped := false
			dropLoop:
				for i := 0; i < 3 && !dropped; i++ {
					select {
					case oldEvent := <-b.queue:
						b.queueSize.Add(-1)
						if oldEvent.Priority != PriorityCritical {
							dropped = true
							if b.logger != nil {
								log.NewHelper(b.logger).Debugf("dropped non-critical event to make room for critical event")
							}
							break dropLoop
						}
						select {
						case b.queue <- oldEvent:
							b.queueSize.Add(1)
						default:
						}
					default:
						break dropLoop
					}
				}

				select {
				case b.queue <- event:
					newSize := b.queueSize.Add(1)
					b.monitor().UpdateQueueSize(int(newSize))
				default:
					b.handleEnqueueOverflow(event, "drop_newest_critical_failed")
				}
			} else {
				b.handleEnqueueOverflow(event, "drop_newest")
			}
		}
	}
}
