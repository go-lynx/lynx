package snowflake

import (
	"sort"
	"time"
)

// NewSnowflakeMetrics creates a new metrics instance
func NewSnowflakeMetrics() *Metrics {
	return &Metrics{
		StartTime:        time.Now(),
		LatencyHistogram: make(map[string]int64),
		MinLatency:       time.Hour, // Initialize with a large value
	}
}

// RecordIDGeneration records metrics for ID generation
func (m *Metrics) RecordIDGeneration(latency time.Duration, cacheHit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IDsGenerated++
	m.LastGenerationTime = time.Now()
	m.UptimeDuration = m.LastGenerationTime.Sub(m.StartTime)

	// Update latency metrics
	m.GenerationLatency = latency
	if latency > m.MaxLatency {
		m.MaxLatency = latency
	}
	if latency < m.MinLatency {
		m.MinLatency = latency
	}

	// Update cache metrics
	if cacheHit {
		m.CacheHits++
	} else {
		m.CacheMisses++
	}

	// Calculate cache hit rate
	totalCacheAccess := m.CacheHits + m.CacheMisses
	if totalCacheAccess > 0 {
		m.CacheHitRate = float64(m.CacheHits) / float64(totalCacheAccess)
	}

	// Update latency histogram
	m.updateLatencyHistogram(latency)

	// Calculate generation rate (IDs per second)
	if m.UptimeDuration.Seconds() > 0 {
		m.IDGenerationRate = float64(m.IDsGenerated) / m.UptimeDuration.Seconds()
		// Update peak generation rate if current rate is higher
		if m.IDGenerationRate > m.PeakGenerationRate {
			m.PeakGenerationRate = m.IDGenerationRate
		}
	}
}

// RecordCacheRefill records cache refill events
func (m *Metrics) RecordCacheRefill() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CacheRefills++
}

// RecordError records different types of errors
func (m *Metrics) RecordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch errorType {
	case "generation":
		m.GenerationErrors++
	case "redis":
		m.RedisErrors++
	case "timeout":
		m.TimeoutErrors++
	case "validation":
		m.ValidationErrors++
	}
}

// RecordClockDrift records clock drift events
func (m *Metrics) RecordClockDrift() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ClockDriftEvents++
}

// RecordSequenceOverflow records sequence overflow events
func (m *Metrics) RecordSequenceOverflow() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SequenceOverflows++
}

// UpdateConnectionMetrics updates Redis connection metrics
func (m *Metrics) UpdateConnectionMetrics(poolSize, active, idle int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RedisConnectionPool = poolSize
	m.ActiveConnections = active
	m.IdleConnections = idle
}

// updateLatencyHistogram updates the latency histogram
func (m *Metrics) updateLatencyHistogram(latency time.Duration) {
	var bucket string

	switch {
	case latency < time.Microsecond:
		bucket = "0-1μs"
	case latency < 10*time.Microsecond:
		bucket = "1-10μs"
	case latency < 100*time.Microsecond:
		bucket = "10-100μs"
	case latency < time.Millisecond:
		bucket = "100μs-1ms"
	case latency < 5*time.Millisecond:
		bucket = "1-5ms"
	case latency < 10*time.Millisecond:
		bucket = "5-10ms"
	case latency < 50*time.Millisecond:
		bucket = "10-50ms"
	default:
		bucket = "50ms+"
	}

	m.LatencyHistogram[bucket]++
}

// CalculatePercentiles calculates P95 and P99 latencies
func (m *Metrics) CalculatePercentiles(latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Sort latencies
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	// Calculate average
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	m.AverageLatency = total / time.Duration(len(latencies))

	// Calculate P95
	p95Index := int(float64(len(latencies)) * 0.95)
	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	m.P95Latency = latencies[p95Index]

	// Calculate P99
	p99Index := int(float64(len(latencies)) * 0.99)
	if p99Index >= len(latencies) {
		p99Index = len(latencies) - 1
	}
	m.P99Latency = latencies[p99Index]
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a deep copy of the metrics
	snapshot := &Metrics{
		IDsGenerated:        m.IDsGenerated,
		ClockDriftEvents:    m.ClockDriftEvents,
		WorkerIDConflicts:   m.WorkerIDConflicts,
		SequenceOverflows:   m.SequenceOverflows,
		GenerationLatency:   m.GenerationLatency,
		AverageLatency:      m.AverageLatency,
		P95Latency:          m.P95Latency,
		P99Latency:          m.P99Latency,
		MaxLatency:          m.MaxLatency,
		MinLatency:          m.MinLatency,
		CacheHitRate:        m.CacheHitRate,
		CacheHits:           m.CacheHits,
		CacheMisses:         m.CacheMisses,
		CacheRefills:        m.CacheRefills,
		IDGenerationRate:    m.IDGenerationRate,
		PeakGenerationRate:  m.PeakGenerationRate,
		GenerationErrors:    m.GenerationErrors,
		RedisErrors:         m.RedisErrors,
		TimeoutErrors:       m.TimeoutErrors,
		ValidationErrors:    m.ValidationErrors,
		RedisConnectionPool: m.RedisConnectionPool,
		ActiveConnections:   m.ActiveConnections,
		IdleConnections:     m.IdleConnections,
		StartTime:           m.StartTime,
		LastGenerationTime:  m.LastGenerationTime,
		UptimeDuration:      m.UptimeDuration,
		LatencyHistogram:    make(map[string]int64),
	}

	// Copy histogram
	for k, v := range m.LatencyHistogram {
		snapshot.LatencyHistogram[k] = v
	}

	return snapshot
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IDsGenerated = 0
	m.ClockDriftEvents = 0
	m.WorkerIDConflicts = 0
	m.SequenceOverflows = 0
	m.GenerationLatency = 0
	m.AverageLatency = 0
	m.P95Latency = 0
	m.P99Latency = 0
	m.MaxLatency = 0
	m.MinLatency = time.Hour
	m.CacheHitRate = 0
	m.CacheHits = 0
	m.CacheMisses = 0
	m.CacheRefills = 0
	m.IDGenerationRate = 0
	m.PeakGenerationRate = 0
	m.GenerationErrors = 0
	m.RedisErrors = 0
	m.TimeoutErrors = 0
	m.ValidationErrors = 0
	m.RedisConnectionPool = 0
	m.ActiveConnections = 0
	m.IdleConnections = 0
	m.StartTime = time.Now()
	m.LastGenerationTime = time.Time{}
	m.UptimeDuration = 0
	m.LatencyHistogram = make(map[string]int64)
}
