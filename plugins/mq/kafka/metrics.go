package kafka

import (
	"sync"
	"time"
)

// Metrics Kafka 监控指标
type Metrics struct {
	mu sync.RWMutex

	// 生产者指标
	ProducedMessages int64
	ProducedBytes    int64
	ProducerErrors   int64
	ProducerLatency  time.Duration

	// 消费者指标
	ConsumedMessages   int64
	ConsumedBytes      int64
	ConsumerErrors     int64
	ConsumerLatency    time.Duration
	OffsetCommits      int64
	OffsetCommitErrors int64

	// 连接指标
	ConnectionErrors int64
	Reconnections    int64
}

// NewMetrics 创建新的监控指标实例
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementProducedMessages 增加生产消息计数
func (m *Metrics) IncrementProducedMessages(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducedMessages += count
}

// IncrementProducedBytes 增加生产字节计数
func (m *Metrics) IncrementProducedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducedBytes += bytes
}

// IncrementProducerErrors 增加生产者错误计数
func (m *Metrics) IncrementProducerErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducerErrors++
}

// IncrementConsumedMessages 增加消费消息计数
func (m *Metrics) IncrementConsumedMessages(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumedMessages += count
}

// IncrementConsumedBytes 增加消费字节计数
func (m *Metrics) IncrementConsumedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumedBytes += bytes
}

// IncrementConsumerErrors 增加消费者错误计数
func (m *Metrics) IncrementConsumerErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumerErrors++
}

// IncrementOffsetCommits 增加偏移量提交计数
func (m *Metrics) IncrementOffsetCommits() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OffsetCommits++
}

// IncrementOffsetCommitErrors 增加偏移量提交错误计数
func (m *Metrics) IncrementOffsetCommitErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OffsetCommitErrors++
}

// IncrementConnectionErrors 增加连接错误计数
func (m *Metrics) IncrementConnectionErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConnectionErrors++
}

// IncrementReconnections 增加重连计数
func (m *Metrics) IncrementReconnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Reconnections++
}

// SetProducerLatency 设置生产者延迟
func (m *Metrics) SetProducerLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProducerLatency = latency
}

// SetConsumerLatency 设置消费者延迟
func (m *Metrics) SetConsumerLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsumerLatency = latency
}

// GetStats 获取统计信息
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

// Reset 重置所有指标
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
