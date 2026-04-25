package events

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// IsHealthy returns whether the bus is healthy.
func (b *LynxEventBus) IsHealthy() bool {
	if b.isClosed.Load() || b.paused.Load() || b.isDegraded.Load() {
		return false
	}
	totalCap := b.totalQueueCap()
	if totalCap > 0 && b.totalQueueSize()*100/totalCap >= 80 {
		return false
	}
	return true
}

// IsPaused returns whether the bus is paused.
func (b *LynxEventBus) IsPaused() bool { return b.paused.Load() }

// IsDegraded returns whether the bus is currently degraded.
func (b *LynxEventBus) IsDegraded() bool { return b.isDegraded.Load() }

// GetQueueSize returns the current queue size (approximate).
func (b *LynxEventBus) GetQueueSize() int { return b.totalQueueSize() }

// GetTotalSubscriberCount returns total subscribers.
func (b *LynxEventBus) GetTotalSubscriberCount() int { return int(b.subscriberCount.Load()) }

// GetSubscriberCount returns the number of subscribers for a specific event type.
func (b *LynxEventBus) GetSubscriberCount(eventType EventType) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.typeSubs[eventType]
}

// Pause stops consuming events from the internal queue while publishing still enqueues.
func (b *LynxEventBus) Pause() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.paused.Load() {
		b.pauseStartTime = time.Now()
		b.paused.Store(true)
		b.pauseCount.Add(1)
		pauseEvent := NewLynxEvent(EventSystemError, "system", "event-bus").
			WithPriority(PriorityHigh).
			WithCategory("system").
			WithStatus("paused").
			WithMetadata("bus_type", b.busType).
			WithMetadata("reason", "manual_pause")
		b.publishManagerEvent(pauseEvent)
	}
}

// Resume resumes consuming events.
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
		b.publishManagerEvent(resumeEvent)
	}
}

// GetPauseStats returns pause statistics.
func (b *LynxEventBus) GetPauseStats() (time.Duration, time.Time) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.paused.Load() {
		return b.pauseDuration + time.Since(b.pauseStartTime), b.pauseStartTime
	}
	return b.pauseDuration, time.Time{}
}

// GetPauseCount returns how many times the bus has been paused.
func (b *LynxEventBus) GetPauseCount() int64 { return b.pauseCount.Load() }

// GetDegradationDuration returns how long the bus has been degraded; zero if not degraded.
func (b *LynxEventBus) GetDegradationDuration() time.Duration {
	if !b.isDegraded.Load() {
		return 0
	}
	return time.Since(b.degradationStartTime)
}

// GetWorkerPoolStats exposes current worker pool stats; returns zeros if pool not initialized.
func (b *LynxEventBus) GetWorkerPoolStats() (cap int, running int, free int, waiting int) {
	b.mu.RLock()
	pool := b.workerPool
	b.mu.RUnlock()
	if pool == nil {
		return 0, 0, 0, 0
	}
	return pool.Cap(), pool.Running(), pool.Free(), pool.Waiting()
}

// checkDegradation toggles degradation state based on configured thresholds and queue usage.
func (b *LynxEventBus) checkDegradation() {
	capTotal := b.totalQueueCap()
	thr := b.configSnapshot().DegradationThreshold
	if capTotal <= 0 || thr <= 0 {
		return
	}
	b.checkDegradationWithSize(b.totalQueueSize(), capTotal)
}

// checkDegradationWithSize checks degradation using pre-calculated queue size and capacity.
func (b *LynxEventBus) checkDegradationWithSize(queueSize, queueCap int) {
	cfg := b.configSnapshot()
	thr := cfg.DegradationThreshold
	if queueCap <= 0 || thr <= 0 {
		return
	}
	usage := queueSize * 100 / queueCap

	if !b.isDegraded.Load() {
		if usage >= thr {
			b.isDegraded.Store(true)
			b.degradationStartTime = time.Now()
			if cfg.DegradationMode == DegradationModePause {
				b.Pause()
			}
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf("bus degraded: bus=%d usage=%d%% thr=%d%% mode=%s", b.busType, usage, thr, cfg.DegradationMode)
			}
			b.monitor().UpdateHealth(false)
		}
		return
	}

	rec := cfg.DegradationRecoverThreshold
	if rec <= 0 {
		rec = thr - 10
		if rec < 1 {
			rec = 1
		}
	}
	if usage <= rec {
		b.isDegraded.Store(false)
		if cfg.DegradationMode == DegradationModePause {
			b.Resume()
		}
		if b.logger != nil {
			log.NewHelper(b.logger).Infof("bus recovered: bus=%d usage=%d%% rec=%d%%", b.busType, usage, rec)
		}
		b.monitor().UpdateHealth(true)
	}
}

// GetEventHistory returns events from history that match the given filter.
func (b *LynxEventBus) GetEventHistory(filter *EventFilter) []LynxEvent {
	_, _, history, _, _ := b.runtimeSnapshot()
	if history == nil {
		return []LynxEvent{}
	}
	if filter == nil {
		return history.GetEvents()
	}
	return history.GetEventsByFilter(filter)
}

// GetPluginEventHistory returns events from history for a specific plugin.
func (b *LynxEventBus) GetPluginEventHistory(pluginID string, filter *EventFilter) []LynxEvent {
	_, _, history, _, _ := b.runtimeSnapshot()
	if history == nil {
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
	return history.GetEventsByFilter(pluginFilter)
}

func (b *LynxEventBus) monitor() *EventMonitor {
	if b != nil && b.manager != nil && b.manager.monitor != nil {
		return b.manager.monitor
	}
	return NewEventMonitor()
}

func (b *LynxEventBus) publishManagerEvent(event LynxEvent) {
	if b != nil && b.manager != nil {
		_ = b.manager.PublishEvent(event)
	}
}
