package rabbitmq

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics represents RabbitMQ metrics
type Metrics struct {
	mu sync.RWMutex

	// Producer metrics
	producerMessagesSent   int64
	producerMessagesFailed int64
	producerLatency        int64 // in nanoseconds

	// Consumer metrics
	consumerMessagesReceived int64
	consumerMessagesFailed   int64
	consumerLatency          int64 // in nanoseconds

	// Connection metrics
	connectionErrors  int64
	reconnectionCount int64
	lastReconnectTime time.Time

	// Health metrics
	healthCheckCount  int64
	healthCheckErrors int64
	lastHealthCheck   time.Time
	isHealthy         int32
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		lastHealthCheck: time.Now(),
	}
}

// IncrementProducerMessagesSent increments the sent message count
func (m *Metrics) IncrementProducerMessagesSent() {
	atomic.AddInt64(&m.producerMessagesSent, 1)
}

// IncrementProducerMessagesFailed increments the failed message count
func (m *Metrics) IncrementProducerMessagesFailed() {
	atomic.AddInt64(&m.producerMessagesFailed, 1)
}

// RecordProducerLatency records producer latency
func (m *Metrics) RecordProducerLatency(duration time.Duration) {
	atomic.StoreInt64(&m.producerLatency, int64(duration))
}

// IncrementConsumerMessagesReceived increments the received message count
func (m *Metrics) IncrementConsumerMessagesReceived() {
	atomic.AddInt64(&m.consumerMessagesReceived, 1)
}

// IncrementConsumerMessagesFailed increments the failed consumption count
func (m *Metrics) IncrementConsumerMessagesFailed() {
	atomic.AddInt64(&m.consumerMessagesFailed, 1)
}

// RecordConsumerLatency records consumer latency
func (m *Metrics) RecordConsumerLatency(duration time.Duration) {
	atomic.StoreInt64(&m.consumerLatency, int64(duration))
}

// IncrementConnectionErrors increments connection error count
func (m *Metrics) IncrementConnectionErrors() {
	atomic.AddInt64(&m.connectionErrors, 1)
}

// IncrementReconnectionCount increments reconnection count
func (m *Metrics) IncrementReconnectionCount() {
	atomic.AddInt64(&m.reconnectionCount, 1)
	m.mu.Lock()
	m.lastReconnectTime = time.Now()
	m.mu.Unlock()
}

// IncrementHealthCheckCount increments health check count
func (m *Metrics) IncrementHealthCheckCount() {
	atomic.AddInt64(&m.healthCheckCount, 1)
}

// IncrementHealthCheckErrors increments health check error count
func (m *Metrics) IncrementHealthCheckErrors() {
	atomic.AddInt64(&m.healthCheckErrors, 1)
}

// SetHealthy sets health status
func (m *Metrics) SetHealthy(healthy bool) {
	if healthy {
		atomic.StoreInt32(&m.isHealthy, 1)
	} else {
		atomic.StoreInt32(&m.isHealthy, 0)
	}
}

// UpdateLastHealthCheck updates last health check time
func (m *Metrics) UpdateLastHealthCheck() {
	m.mu.Lock()
	m.lastHealthCheck = time.Now()
	m.mu.Unlock()
}

// GetStats returns all metrics as a map
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"producer": map[string]interface{}{
			"messages_sent":   atomic.LoadInt64(&m.producerMessagesSent),
			"messages_failed": atomic.LoadInt64(&m.producerMessagesFailed),
			"latency_ns":      atomic.LoadInt64(&m.producerLatency),
		},
		"consumer": map[string]interface{}{
			"messages_received": atomic.LoadInt64(&m.consumerMessagesReceived),
			"messages_failed":   atomic.LoadInt64(&m.consumerMessagesFailed),
			"latency_ns":        atomic.LoadInt64(&m.consumerLatency),
		},
		"connection": map[string]interface{}{
			"errors":             atomic.LoadInt64(&m.connectionErrors),
			"reconnection_count": atomic.LoadInt64(&m.reconnectionCount),
			"last_reconnect":     m.lastReconnectTime,
		},
		"health": map[string]interface{}{
			"check_count":  atomic.LoadInt64(&m.healthCheckCount),
			"check_errors": atomic.LoadInt64(&m.healthCheckErrors),
			"last_check":   m.lastHealthCheck,
			"is_healthy":   atomic.LoadInt32(&m.isHealthy) == 1,
		},
	}
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.StoreInt64(&m.producerMessagesSent, 0)
	atomic.StoreInt64(&m.producerMessagesFailed, 0)
	atomic.StoreInt64(&m.producerLatency, 0)
	atomic.StoreInt64(&m.consumerMessagesReceived, 0)
	atomic.StoreInt64(&m.consumerMessagesFailed, 0)
	atomic.StoreInt64(&m.consumerLatency, 0)
	atomic.StoreInt64(&m.connectionErrors, 0)
	atomic.StoreInt64(&m.reconnectionCount, 0)
	atomic.StoreInt64(&m.healthCheckCount, 0)
	atomic.StoreInt64(&m.healthCheckErrors, 0)
	atomic.StoreInt32(&m.isHealthy, 0)

	m.lastReconnectTime = time.Time{}
	m.lastHealthCheck = time.Now()
}

// GetPrometheusMetrics returns metrics in Prometheus format
func (m *Metrics) GetPrometheusMetrics() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var metrics []string

	// Producer metrics
	metrics = append(metrics,
		"# HELP lynx_rabbitmq_producer_messages_sent_total Total number of messages sent",
		"# TYPE lynx_rabbitmq_producer_messages_sent_total counter",
		fmt.Sprintf("lynx_rabbitmq_producer_messages_sent_total %d", atomic.LoadInt64(&m.producerMessagesSent)),
		"",
		"# HELP lynx_rabbitmq_producer_messages_failed_total Total number of failed messages",
		"# TYPE lynx_rabbitmq_producer_messages_failed_total counter",
		fmt.Sprintf("lynx_rabbitmq_producer_messages_failed_total %d", atomic.LoadInt64(&m.producerMessagesFailed)),
		"",
		"# HELP lynx_rabbitmq_producer_latency_seconds Producer latency in seconds",
		"# TYPE lynx_rabbitmq_producer_latency_seconds gauge",
		fmt.Sprintf("lynx_rabbitmq_producer_latency_seconds %f", 
			time.Duration(atomic.LoadInt64(&m.producerLatency)).Seconds()),
		"",
	)

	// Consumer metrics
	metrics = append(metrics,
		"# HELP lynx_rabbitmq_consumer_messages_received_total Total number of messages received",
		"# TYPE lynx_rabbitmq_consumer_messages_received_total counter",
		fmt.Sprintf("lynx_rabbitmq_consumer_messages_received_total %d", 
			atomic.LoadInt64(&m.consumerMessagesReceived)),
		"",
		"# HELP lynx_rabbitmq_consumer_messages_failed_total Total number of failed message consumptions",
		"# TYPE lynx_rabbitmq_consumer_messages_failed_total counter",
		fmt.Sprintf("lynx_rabbitmq_consumer_messages_failed_total %d", 
			atomic.LoadInt64(&m.consumerMessagesFailed)),
		"",
		"# HELP lynx_rabbitmq_consumer_latency_seconds Consumer latency in seconds",
		"# TYPE lynx_rabbitmq_consumer_latency_seconds gauge",
		fmt.Sprintf("lynx_rabbitmq_consumer_latency_seconds %f",
			time.Duration(atomic.LoadInt64(&m.consumerLatency)).Seconds()),
		"",
	)

	// Connection metrics
	metrics = append(metrics,
		"# HELP lynx_rabbitmq_connection_errors_total Total number of connection errors",
		"# TYPE lynx_rabbitmq_connection_errors_total counter",
		fmt.Sprintf("lynx_rabbitmq_connection_errors_total %d", atomic.LoadInt64(&m.connectionErrors)),
		"",
		"# HELP lynx_rabbitmq_reconnection_count_total Total number of reconnections",
		"# TYPE lynx_rabbitmq_reconnection_count_total counter",
		fmt.Sprintf("lynx_rabbitmq_reconnection_count_total %d", atomic.LoadInt64(&m.reconnectionCount)),
		"",
	)

	// Health metrics
	healthyValue := 0
	if atomic.LoadInt32(&m.isHealthy) == 1 {
		healthyValue = 1
	}
	
	metrics = append(metrics,
		"# HELP lynx_rabbitmq_health_check_count_total Total number of health checks",
		"# TYPE lynx_rabbitmq_health_check_count_total counter",
		fmt.Sprintf("lynx_rabbitmq_health_check_count_total %d", atomic.LoadInt64(&m.healthCheckCount)),
		"",
		"# HELP lynx_rabbitmq_health_check_errors_total Total number of health check errors",
		"# TYPE lynx_rabbitmq_health_check_errors_total counter",
		fmt.Sprintf("lynx_rabbitmq_health_check_errors_total %d", atomic.LoadInt64(&m.healthCheckErrors)),
		"",
		"# HELP lynx_rabbitmq_health_status Current health status (1=healthy, 0=unhealthy)",
		"# TYPE lynx_rabbitmq_health_status gauge",
		fmt.Sprintf("lynx_rabbitmq_health_status %d", healthyValue),
		"",
		"# HELP lynx_rabbitmq_last_health_check_timestamp Unix timestamp of last health check",
		"# TYPE lynx_rabbitmq_last_health_check_timestamp gauge",
		fmt.Sprintf("lynx_rabbitmq_last_health_check_timestamp %d", m.lastHealthCheck.Unix()),
	)

	if !m.lastReconnectTime.IsZero() {
		metrics = append(metrics,
			"",
			"# HELP lynx_rabbitmq_last_reconnect_timestamp Unix timestamp of last reconnection",
			"# TYPE lynx_rabbitmq_last_reconnect_timestamp gauge",
			fmt.Sprintf("lynx_rabbitmq_last_reconnect_timestamp %d", m.lastReconnectTime.Unix()),
		)
	}

	return strings.Join(metrics, "\n")
}
