package events

import (
	"fmt"
	"sync"
	"time"
)

// EventMonitor monitors the health and performance of the event system
type EventMonitor struct {
	// Health status
	healthy   bool
	lastCheck time.Time

	// Performance metrics
	totalEventsPublished int64
	totalEventsProcessed int64
	totalEventsDropped   int64
	totalEventsFailed    int64

	// Latency metrics
	avgLatency time.Duration
	maxLatency time.Duration
	minLatency time.Duration
	// percentile sampling (ring buffer of recent latencies in ns)
	latSamples []int64
	sampleIdx  int
	sampleCap  int

	// Queue metrics
	maxQueueSize     int
	currentQueueSize int

	// Error tracking
	lastError  error
	errorCount int64

	// Enhanced observability
	droppedByReason      map[string]int64
	publishedByPriority  map[Priority]int64

	// Mutex for thread safety
	mu sync.RWMutex
}

// copyReasonBuckets returns a shallow copy to avoid leaking internal map
func copyReasonBuckets(in map[string]int64) map[string]int64 {
	if in == nil {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// copyPriorityBuckets returns a shallow copy to avoid leaking internal map
func copyPriorityBuckets(in map[Priority]int64) map[Priority]int64 {
	if in == nil {
		return nil
	}
	out := make(map[Priority]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// NewEventMonitor creates a new event monitor
func NewEventMonitor() *EventMonitor {
	return &EventMonitor{
		healthy:    true,
		lastCheck:  time.Now(),
		minLatency: time.Hour, // Start with a high value
		sampleCap:  512,       // default window size
		latSamples: make([]int64, 512),
		droppedByReason:     make(map[string]int64),
		publishedByPriority: make(map[Priority]int64),
	}
}

// UpdateHealth updates the health status
func (m *EventMonitor) UpdateHealth(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthy = healthy
	m.lastCheck = time.Now()
}

// IsHealthy returns the current health status
func (m *EventMonitor) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.healthy
}

// GetLastCheck returns the last health check time
func (m *EventMonitor) GetLastCheck() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lastCheck
}

// IncrementPublished increments the published events counter
func (m *EventMonitor) IncrementPublished() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEventsPublished++
}

// IncrementPublishedByPriority increments published counter and bucket by priority
func (m *EventMonitor) IncrementPublishedByPriority(p Priority) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalEventsPublished++
	m.publishedByPriority[p] = m.publishedByPriority[p] + 1
}

// IncrementProcessed increments the processed events counter
func (m *EventMonitor) IncrementProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEventsProcessed++
}

// IncrementDropped increments the dropped events counter
func (m *EventMonitor) IncrementDropped() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEventsDropped++
}

// IncrementDroppedByReason increments dropped counter and bucket by reason
func (m *EventMonitor) IncrementDroppedByReason(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalEventsDropped++
	if reason == "" {
		reason = "unknown"
	}
	m.droppedByReason[reason] = m.droppedByReason[reason] + 1
}

// IncrementFailed increments the failed events counter
func (m *EventMonitor) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEventsFailed++
	m.errorCount++
}

// UpdateLatency updates latency metrics
func (m *EventMonitor) UpdateLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update average latency using EMA to smooth spikes
	// avg = avg + (latency - avg) / k  (k=5 => alpha=0.2)
	const k = 5
	if m.totalEventsProcessed == 0 || m.avgLatency <= 0 {
		m.avgLatency = latency
	} else {
		m.avgLatency += (latency - m.avgLatency) / k
	}

	// Update max latency
	if latency > m.maxLatency {
		m.maxLatency = latency
	}

	// Update min latency
	if latency < m.minLatency {
		m.minLatency = latency
	}

	// Record sample into ring buffer (nanoseconds)
	if m.sampleCap > 0 {
		m.latSamples[m.sampleIdx%m.sampleCap] = int64(latency)
		m.sampleIdx++
	}
}

// UpdateQueueSize updates queue size metrics
func (m *EventMonitor) UpdateQueueSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentQueueSize = size
	if size > m.maxQueueSize {
		m.maxQueueSize = size
	}
}

// SetError sets the last error
func (m *EventMonitor) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastError = err
	m.errorCount++
}

// GetMetrics returns all monitoring metrics
func (m *EventMonitor) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// compute percentiles from samples snapshot
	p95, p99 := m.computePercentilesLocked()

	return map[string]interface{}{
		"healthy":                m.healthy,
		"last_check":             m.lastCheck,
		"total_events_published": m.totalEventsPublished,
		"total_events_processed": m.totalEventsProcessed,
		"total_events_dropped":   m.totalEventsDropped,
		"total_events_failed":    m.totalEventsFailed,
		"published_by_priority":  copyPriorityBuckets(m.publishedByPriority),
		"dropped_by_reason":      copyReasonBuckets(m.droppedByReason),
		"avg_latency_ms":         m.avgLatency.Milliseconds(),
		"max_latency_ms":         m.maxLatency.Milliseconds(),
		"min_latency_ms":         m.minLatency.Milliseconds(),
		"p95_latency_ms":         p95.Milliseconds(),
		"p99_latency_ms":         p99.Milliseconds(),
		"max_queue_size":         m.maxQueueSize,
		"current_queue_size":     m.currentQueueSize,
		"error_count":            m.errorCount,
		"last_error":             m.lastError,
	}
}

// computePercentilesLocked computes p95/p99 from the current samples under read lock
func (m *EventMonitor) computePercentilesLocked() (time.Duration, time.Duration) {
	// determine valid sample count
	n := m.sampleIdx
	if n > m.sampleCap {
		n = m.sampleCap
	}
	if n <= 0 {
		return 0, 0
	}

	// copy samples to buffer
	buf := make([]int64, n)
	start := 0
	if m.sampleIdx > m.sampleCap {
		start = m.sampleIdx % m.sampleCap
	}
	for i := 0; i < n; i++ {
		buf[i] = m.latSamples[(start+i)%m.sampleCap]
	}

	// Use quick select for better performance (O(n) average case)
	p95 := quickSelect(buf, (95*n+99)/100)
	p99 := quickSelect(buf, (99*n+99)/100)

	return time.Duration(p95), time.Duration(p99)
}

// quickSelect finds the kth smallest element in O(n) average time
func quickSelect(arr []int64, k int) int64 {
	if len(arr) == 0 {
		return 0
	}
	if len(arr) == 1 {
		return arr[0]
	}

	// Ensure k is within bounds
	if k < 0 {
		k = 0
	}
	if k >= len(arr) {
		k = len(arr) - 1
	}

	pivot := arr[len(arr)/2]
	var left, right, equal []int64

	for _, x := range arr {
		switch {
		case x < pivot:
			left = append(left, x)
		case x > pivot:
			right = append(right, x)
		default:
			equal = append(equal, x)
		}
	}

	if k < len(left) {
		return quickSelect(left, k)
	} else if k < len(left)+len(equal) {
		return pivot
	} else {
		return quickSelect(right, k-len(left)-len(equal))
	}
}

// Reset resets all metrics
func (m *EventMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEventsPublished = 0
	m.totalEventsProcessed = 0
	m.totalEventsDropped = 0
	m.totalEventsFailed = 0
	m.avgLatency = 0
	m.maxLatency = 0
	m.minLatency = time.Hour
	m.maxQueueSize = 0
	m.currentQueueSize = 0
	m.errorCount = 0
	m.lastError = nil
}

// EventSystemHealth represents the overall health of the event system
type EventSystemHealth struct {
	OverallHealthy bool
	BusesHealthy   map[BusType]bool
	LastCheck      time.Time
	Issues         []string
}

// GetEventSystemHealth returns the overall health of the event system
func GetEventSystemHealth() *EventSystemHealth {
	eventManager := GetGlobalEventBus()
	if eventManager == nil {
		return &EventSystemHealth{
			OverallHealthy: false,
			BusesHealthy:   make(map[BusType]bool),
			LastCheck:      time.Now(),
			Issues:         []string{"Global event bus not initialized"},
		}
	}

	busStatus := eventManager.GetBusStatus()
	overallHealthy := true
	issues := make([]string, 0)
	busesHealthy := make(map[BusType]bool, len(busStatus))

	for busType, status := range busStatus {
		busesHealthy[busType] = status.IsHealthy
		if !status.IsHealthy {
			overallHealthy = false
			issues = append(issues, fmt.Sprintf("Bus %d unhealthy (queue=%d, subs=%d)", busType, status.QueueSize, status.Subscribers))
		}
	}

	return &EventSystemHealth{
		OverallHealthy: overallHealthy,
		BusesHealthy:   busesHealthy,
		LastCheck:      time.Now(),
		Issues:         issues,
	}
}

// Global monitor instance
var (
	globalMonitor     *EventMonitor
	globalMonitorOnce sync.Once
	healthCheckDone   chan struct{}
	healthCheckOnce   sync.Once
)

// GetGlobalMonitor returns the global event monitor
func GetGlobalMonitor() *EventMonitor {
	globalMonitorOnce.Do(func() {
		globalMonitor = NewEventMonitor()
	})
	return globalMonitor
}

// UpdateGlobalHealth updates the global health status
func UpdateGlobalHealth(healthy bool) {
	GetGlobalMonitor().UpdateHealth(healthy)
}

// GetGlobalHealth returns the global health status
func GetGlobalHealth() bool {
	return GetGlobalMonitor().IsHealthy()
}

// GetGlobalMetrics returns the global monitoring metrics
func GetGlobalMetrics() map[string]interface{} {
	return GetGlobalMonitor().GetMetrics()
}

// StartHealthCheck starts periodic health checks for the event system
func StartHealthCheck(interval time.Duration) {
	healthCheckOnce.Do(func() {
		healthCheckDone = make(chan struct{})
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-healthCheckDone:
					return
				case <-ticker.C:
					performHealthCheck()
				}
			}
		}()
	})
}

// StopHealthCheck stops the health check goroutine
func StopHealthCheck() {
	healthCheckOnce.Do(func() {
		// If not started, do nothing
	})
	if healthCheckDone != nil {
		close(healthCheckDone)
		healthCheckDone = nil
	}
}

// performHealthCheck performs a health check and emits health events
func performHealthCheck() {
	health := GetEventSystemHealth()

	// Update monitor health status
	GetGlobalMonitor().UpdateHealth(health.OverallHealthy)

	// Emit health check event
	healthEvent := LynxEvent{
		EventType: EventHealthCheckStarted,
		Priority:  PriorityNormal,
		Source:    "event-system",
		Category:  "health",
		PluginID:  "system",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"overall_health": health.OverallHealthy,
			"bus_count":      len(health.BusesHealthy),
			"healthy_buses":  countHealthyBuses(health.BusesHealthy),
			"issues":         health.Issues,
		},
	}

	// Publish health event
	PublishEvent(healthEvent)

	// Emit health status event
	statusEvent := LynxEvent{
		EventType: EventHealthStatusOK,
		Priority:  PriorityNormal,
		Source:    "event-system",
		Category:  "health",
		PluginID:  "system",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"health_status": health.OverallHealthy,
			"last_check":    health.LastCheck,
		},
	}

	if !health.OverallHealthy {
		statusEvent.EventType = EventHealthStatusCritical
		statusEvent.Priority = PriorityHigh
		statusEvent.Status = "error"
	}

	PublishEvent(statusEvent)
}

// countHealthyBuses counts the number of healthy buses
func countHealthyBuses(busesHealthy map[BusType]bool) int {
	count := 0
	for _, healthy := range busesHealthy {
		if healthy {
			count++
		}
	}
	return count
}
