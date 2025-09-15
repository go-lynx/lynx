package grpc

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ClientMetrics represents metrics for gRPC client
type ClientMetrics struct {
	// Connection metrics
	connectionsTotal    prometheus.Gauge
	connectionsActive   prometheus.Gauge
	connectionsCreated  prometheus.Counter
	connectionsClosed   prometheus.Counter
	connectionsFailed   prometheus.Counter

	// Request metrics
	requestsTotal       *prometheus.CounterVec
	requestDuration     *prometheus.HistogramVec
	requestErrors       *prometheus.CounterVec

	// Retry metrics
	retriesTotal        *prometheus.CounterVec
	retryDuration       *prometheus.HistogramVec

	// Health check metrics
	healthChecksTotal   *prometheus.CounterVec
	healthCheckDuration *prometheus.HistogramVec

	// Connection pool metrics
	poolSize            *prometheus.GaugeVec
	poolActive          *prometheus.GaugeVec
	poolIdle            *prometheus.GaugeVec

	// Message size metrics
	messageSize         *prometheus.HistogramVec

	// Circuit breaker metrics
	circuitBreakerState *prometheus.GaugeVec
	circuitBreakerTrips *prometheus.CounterVec

	initialized bool
	mu          sync.RWMutex
}

// NewClientMetrics creates a new ClientMetrics instance
func NewClientMetrics() *ClientMetrics {
	return &ClientMetrics{
		connectionsTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "grpc_client_connections_total",
			Help: "Total number of gRPC client connections",
		}),
		connectionsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "grpc_client_connections_active",
			Help: "Number of active gRPC client connections",
		}),
		connectionsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "grpc_client_connections_created_total",
			Help: "Total number of gRPC client connections created",
		}),
		connectionsClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "grpc_client_connections_closed_total",
			Help: "Total number of gRPC client connections closed",
		}),
		connectionsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "grpc_client_connections_failed_total",
			Help: "Total number of failed gRPC client connection attempts",
		}),
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_client_requests_total",
			Help: "Total number of gRPC client requests",
		}, []string{"service", "method", "status"}),
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_client_request_duration_seconds",
			Help:    "Duration of gRPC client requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method"}),
		requestErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_client_request_errors_total",
			Help: "Total number of gRPC client request errors",
		}, []string{"service", "method", "error_type"}),
		retriesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_client_retries_total",
			Help: "Total number of gRPC client retries",
		}, []string{"service", "method"}),
		retryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_client_retry_duration_seconds",
			Help:    "Duration of gRPC client retries",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method"}),
		healthChecksTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_client_health_checks_total",
			Help: "Total number of gRPC client health checks",
		}, []string{"service", "status"}),
		healthCheckDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_client_health_check_duration_seconds",
			Help:    "Duration of gRPC client health checks",
			Buckets: prometheus.DefBuckets,
		}, []string{"service"}),
		poolSize: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_client_pool_size",
			Help: "Size of gRPC client connection pool",
		}, []string{"service"}),
		poolActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_client_pool_active",
			Help: "Number of active connections in pool",
		}, []string{"service"}),
		poolIdle: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_client_pool_idle",
			Help: "Number of idle connections in pool",
		}, []string{"service"}),
		messageSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_client_message_size_bytes",
			Help:    "Size of gRPC client messages",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to 1GB
		}, []string{"service", "method", "direction"}),
		circuitBreakerState: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_client_circuit_breaker_state",
			Help: "State of gRPC client circuit breaker (0=closed, 1=open, 2=half-open)",
		}, []string{"service"}),
		circuitBreakerTrips: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_client_circuit_breaker_trips_total",
			Help: "Total number of gRPC client circuit breaker trips",
		}, []string{"service"}),
	}
}

// Initialize initializes the metrics
func (m *ClientMetrics) Initialize() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = true
}

// RecordConnectionCreated records a connection creation
func (m *ClientMetrics) RecordConnectionCreated(serviceName string) {
	m.connectionsCreated.Inc()
	m.connectionsTotal.Inc()
	m.connectionsActive.Inc()
}

// RecordConnectionClosed records a connection closure
func (m *ClientMetrics) RecordConnectionClosed(serviceName string) {
	m.connectionsClosed.Inc()
	m.connectionsTotal.Dec()
	m.connectionsActive.Dec()
}

// RecordConnectionFailed records a failed connection attempt
func (m *ClientMetrics) RecordConnectionFailed(serviceName string) {
	m.connectionsFailed.Inc()
}

// RecordRequest records a request
func (m *ClientMetrics) RecordRequest(duration time.Duration, status string) {
	m.requestsTotal.WithLabelValues("unknown", "unknown", status).Inc()
	m.requestDuration.WithLabelValues("unknown", "unknown").Observe(duration.Seconds())
}

// RecordRequestWithDetails records a request with service and method details
func (m *ClientMetrics) RecordRequestWithDetails(serviceName, method string, duration time.Duration, status string) {
	m.requestsTotal.WithLabelValues(serviceName, method, status).Inc()
	m.requestDuration.WithLabelValues(serviceName, method).Observe(duration.Seconds())
}

// RecordRequestError records a request error
func (m *ClientMetrics) RecordRequestError(serviceName, method, errorType string) {
	m.requestErrors.WithLabelValues(serviceName, method, errorType).Inc()
}

// RecordRetry records a retry attempt
func (m *ClientMetrics) RecordRetry(serviceName, method string, duration time.Duration) {
	m.retriesTotal.WithLabelValues(serviceName, method).Inc()
	m.retryDuration.WithLabelValues(serviceName, method).Observe(duration.Seconds())
}

// RecordHealthCheck records a health check
func (m *ClientMetrics) RecordHealthCheck(serviceName string, duration time.Duration, status string) {
	m.healthChecksTotal.WithLabelValues(serviceName, status).Inc()
	m.healthCheckDuration.WithLabelValues(serviceName).Observe(duration.Seconds())
}

// RecordPoolSize records connection pool size
func (m *ClientMetrics) RecordPoolSize(serviceName string, size int) {
	m.poolSize.WithLabelValues(serviceName).Set(float64(size))
}

// RecordPoolActive records active connections in pool
func (m *ClientMetrics) RecordPoolActive(serviceName string, active int) {
	m.poolActive.WithLabelValues(serviceName).Set(float64(active))
}

// RecordPoolIdle records idle connections in pool
func (m *ClientMetrics) RecordPoolIdle(serviceName string, idle int) {
	m.poolIdle.WithLabelValues(serviceName).Set(float64(idle))
}

// RecordMessageSize records message size
func (m *ClientMetrics) RecordMessageSize(serviceName, method, direction string, size int) {
	m.messageSize.WithLabelValues(serviceName, method, direction).Observe(float64(size))
}

// RecordCircuitBreakerState records circuit breaker state
func (m *ClientMetrics) RecordCircuitBreakerState(serviceName string, state int) {
	m.circuitBreakerState.WithLabelValues(serviceName).Set(float64(state))
}

// RecordCircuitBreakerTrip records circuit breaker trip
func (m *ClientMetrics) RecordCircuitBreakerTrip(serviceName string) {
	m.circuitBreakerTrips.WithLabelValues(serviceName).Inc()
}

// GetConnectionCount returns the current connection count
func (m *ClientMetrics) GetConnectionCount() float64 {
	return m.connectionsTotal.Get()
}

// GetActiveConnectionCount returns the current active connection count
func (m *ClientMetrics) GetActiveConnectionCount() float64 {
	return m.connectionsActive.Get()
}

// GetRequestCount returns the total request count
func (m *ClientMetrics) GetRequestCount() float64 {
	return m.requestsTotal.GetMetricWithLabelValues("unknown", "unknown", "success").GetCounter().Get()
}

// GetErrorCount returns the total error count
func (m *ClientMetrics) GetErrorCount() float64 {
	return m.requestErrors.GetMetricWithLabelValues("unknown", "unknown", "unknown").GetCounter().Get()
}

// IsInitialized returns whether metrics are initialized
func (m *ClientMetrics) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

