package kafka

import (
	"sync"
	"time"
)

// Metrics Kafka monitoring metrics
type Metrics struct {
	mu sync.RWMutex

	// Producer metrics
	ProducedMessages int64
	ProducedBytes    int64
	ProducerErrors   int64
	ProducerLatency  time.Duration

	// Consumer metrics
	ConsumedMessages   int64
	ConsumedBytes      int64
	ConsumerErrors     int64
	ConsumerLatency    time.Duration
	OffsetCommits      int64
	OffsetCommitErrors int64

	// Connection metrics
	ConnectionErrors int64
	Reconnections    int64
}

// NewMetrics creates a new monitoring metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementProducedMessages increments produced message count
func (m *Metrics) IncrementProducedMessages(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducedMessages += count
}

// IncrementProducedBytes increments produced byte count
func (m *Metrics) IncrementProducedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducedBytes += bytes
}

// IncrementProducerErrors increments producer error count
func (m *Metrics) IncrementProducerErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducerErrors++
}

// IncrementConsumedMessages increments consumed message count
func (m *Metrics) IncrementConsumedMessages(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumedMessages += count
}

// IncrementConsumedBytes increments consumed byte count
func (m *Metrics) IncrementConsumedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumedBytes += bytes
}

// IncrementConsumerErrors increments consumer error count
func (m *Metrics) IncrementConsumerErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumerErrors++
}

// IncrementOffsetCommits increments offset commit count
func (m *Metrics) IncrementOffsetCommits() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OffsetCommits++
}

// IncrementOffsetCommitErrors increments offset commit error count
func (m *Metrics) IncrementOffsetCommitErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OffsetCommitErrors++
}

// IncrementConnectionErrors increments connection error count
func (m *Metrics) IncrementConnectionErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConnectionErrors++
}

// IncrementReconnections increments reconnection count
func (m *Metrics) IncrementReconnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Reconnections++
}

// SetProducerLatency sets producer latency
func (m *Metrics) SetProducerLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducerLatency = latency
}

// SetConsumerLatency sets consumer latency
func (m *Metrics) SetConsumerLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumerLatency = latency
}

// GetStats gets statistics
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"produced_messages":    m.ProducedMessages,
		"produced_bytes":       m.ProducedBytes,
		"producer_errors":      m.ProducerErrors,
		"producer_latency":     m.ProducerLatency.String(),
		"consumed_messages":    m.ConsumedMessages,
		"consumed_bytes":       m.ConsumedBytes,
		"consumer_errors":      m.ConsumerErrors,
		"consumer_latency":     m.ConsumerLatency.String(),
		"offset_commits":       m.OffsetCommits,
		"offset_commit_errors": m.OffsetCommitErrors,
		"connection_errors":    m.ConnectionErrors,
		"reconnections":        m.Reconnections,
	}
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ProducedMessages = 0
	m.ProducedBytes = 0
	m.ProducerErrors = 0
	m.ProducerLatency = 0
	m.ConsumedMessages = 0
	m.ConsumedBytes = 0
	m.ConsumerErrors = 0
	m.ConsumerLatency = 0
	m.OffsetCommits = 0
	m.OffsetCommitErrors = 0
	m.ConnectionErrors = 0
	m.Reconnections = 0
}
