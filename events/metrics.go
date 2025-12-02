package events

import (
	"sync/atomic"
	"time"
)

// EventMetrics tracks metrics for event processing
type EventMetrics struct {
	eventsPublished   int64
	eventsProcessed   int64
	eventsDropped     int64
	eventsFailed      int64
	processingLatency int64 // in nanoseconds
	lastUpdateTime    int64 // Unix timestamp

	// Aggregated statistics
	totalLatency int64 // Sum of all latencies for average calculation
	latencyCount int64 // Count of latency measurements
	minLatency   int64 // Minimum latency observed
	maxLatency   int64 // Maximum latency observed

	// Sliding window for recent metrics (last 100 measurements)
	recentLatencies []int64
	latencyIndex    int64
	windowSize      int
}

// NewEventMetrics creates a new EventMetrics instance
func NewEventMetrics() *EventMetrics {
	return &EventMetrics{
		lastUpdateTime:  time.Now().Unix(),
		minLatency:      time.Hour.Nanoseconds(), // Start with a high value
		windowSize:      100,
		recentLatencies: make([]int64, 100),
	}
}

// IncrementPublished increments the published events counter
func (m *EventMetrics) IncrementPublished() {
	atomic.AddInt64(&m.eventsPublished, 1)
	atomic.StoreInt64(&m.lastUpdateTime, time.Now().Unix())
}

// IncrementProcessed increments the processed events counter
func (m *EventMetrics) IncrementProcessed() {
	atomic.AddInt64(&m.eventsProcessed, 1)
}

// IncrementDropped increments the dropped events counter
func (m *EventMetrics) IncrementDropped() {
	atomic.AddInt64(&m.eventsDropped, 1)
}

// IncrementFailed increments the failed events counter
func (m *EventMetrics) IncrementFailed() {
	atomic.AddInt64(&m.eventsFailed, 1)
}

// UpdateLatency updates the processing latency
func (m *EventMetrics) UpdateLatency(latency time.Duration) {
	latencyNs := int64(latency)

	// Update current latency
	atomic.StoreInt64(&m.processingLatency, latencyNs)

	// Update aggregated statistics
	atomic.AddInt64(&m.totalLatency, latencyNs)
	atomic.AddInt64(&m.latencyCount, 1)

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&m.minLatency)
		if latencyNs >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.minLatency, currentMin, latencyNs) {
			break
		}
	}

	for {
		currentMax := atomic.LoadInt64(&m.maxLatency)
		if latencyNs <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxLatency, currentMax, latencyNs) {
			break
		}
	}

	// Update sliding window
	index := atomic.AddInt64(&m.latencyIndex, 1) % int64(m.windowSize)
	m.recentLatencies[index] = latencyNs
}

// GetPublished returns the number of published events
func (m *EventMetrics) GetPublished() int64 {
	return atomic.LoadInt64(&m.eventsPublished)
}

// GetProcessed returns the number of processed events
func (m *EventMetrics) GetProcessed() int64 {
	return atomic.LoadInt64(&m.eventsProcessed)
}

// GetDropped returns the number of dropped events
func (m *EventMetrics) GetDropped() int64 {
	return atomic.LoadInt64(&m.eventsDropped)
}

// GetFailed returns the number of failed events
func (m *EventMetrics) GetFailed() int64 {
	return atomic.LoadInt64(&m.eventsFailed)
}

// GetLatency returns the current processing latency
func (m *EventMetrics) GetLatency() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.processingLatency))
}

// GetLastUpdateTime returns the last update time
func (m *EventMetrics) GetLastUpdateTime() time.Time {
	return time.Unix(atomic.LoadInt64(&m.lastUpdateTime), 0)
}

// GetMetrics returns all metrics as a map
func (m *EventMetrics) GetMetrics() map[string]interface{} {
	totalLatency := atomic.LoadInt64(&m.totalLatency)
	latencyCount := atomic.LoadInt64(&m.latencyCount)
	minLatency := atomic.LoadInt64(&m.minLatency)
	maxLatency := atomic.LoadInt64(&m.maxLatency)

	var avgLatency time.Duration
	if latencyCount > 0 {
		avgLatency = time.Duration(totalLatency / latencyCount)
	}

	return map[string]interface{}{
		"published":        m.GetPublished(),
		"processed":        m.GetProcessed(),
		"dropped":          m.GetDropped(),
		"failed":           m.GetFailed(),
		"latency_ns":       m.GetLatency().Nanoseconds(),
		"latency_ms":       m.GetLatency().Milliseconds(),
		"avg_latency_ns":   avgLatency.Nanoseconds(),
		"avg_latency_ms":   avgLatency.Milliseconds(),
		"min_latency_ns":   minLatency,
		"min_latency_ms":   time.Duration(minLatency).Milliseconds(),
		"max_latency_ns":   maxLatency,
		"max_latency_ms":   time.Duration(maxLatency).Milliseconds(),
		"latency_count":    latencyCount,
		"last_update_time": m.GetLastUpdateTime(),
	}
}

// Reset resets all metrics to zero
func (m *EventMetrics) Reset() {
	atomic.StoreInt64(&m.eventsPublished, 0)
	atomic.StoreInt64(&m.eventsProcessed, 0)
	atomic.StoreInt64(&m.eventsDropped, 0)
	atomic.StoreInt64(&m.eventsFailed, 0)
	atomic.StoreInt64(&m.processingLatency, 0)
	atomic.StoreInt64(&m.lastUpdateTime, time.Now().Unix())

	// Reset aggregated statistics
	atomic.StoreInt64(&m.totalLatency, 0)
	atomic.StoreInt64(&m.latencyCount, 0)
	atomic.StoreInt64(&m.minLatency, time.Hour.Nanoseconds())
	atomic.StoreInt64(&m.maxLatency, 0)
	atomic.StoreInt64(&m.latencyIndex, 0)

	// Clear sliding window
	for i := range m.recentLatencies {
		m.recentLatencies[i] = 0
	}
}
