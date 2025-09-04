package base

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics provides a unified Prometheus metrics implementation
type PrometheusMetrics struct {
	// Connection pool metrics
	maxOpenConnections *prometheus.GaugeVec
	openConnections    *prometheus.GaugeVec
	inUseConnections   *prometheus.GaugeVec
	idleConnections    *prometheus.GaugeVec
	maxIdleConnections *prometheus.GaugeVec

	// Wait metrics
	waitCount    *prometheus.CounterVec
	waitDuration *prometheus.CounterVec

	// Connection close metrics
	maxIdleClosed     *prometheus.CounterVec
	maxLifetimeClosed *prometheus.CounterVec

	// Health check metrics
	healthCheckTotal   *prometheus.CounterVec
	healthCheckSuccess *prometheus.CounterVec
	healthCheckFailure *prometheus.CounterVec

	// Query/transaction metrics
	queryDuration *prometheus.HistogramVec
	txDuration    *prometheus.HistogramVec
	errorCounter  *prometheus.CounterVec
	slowQueryCnt  *prometheus.CounterVec

	// Connection metrics
	connectAttempts *prometheus.CounterVec
	connectRetries  *prometheus.CounterVec
	connectSuccess  *prometheus.CounterVec
	connectFailures *prometheus.CounterVec

	// Registry
	registry *prometheus.Registry
}

// NewPrometheusMetrics creates a new Prometheus metrics instance
func NewPrometheusMetrics(config *MetricsConfig) *PrometheusMetrics {
	if config == nil {
		config = DefaultMetricsConfig()
	}

	// Create labels
	labels := []string{"instance", "database", "driver"}
	for key := range config.Labels {
		labels = append(labels, key)
	}

	metrics := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}

	// Connection pool metrics
	metrics.maxOpenConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_open_connections",
			Help:      "Maximum number of open connections to the database",
		},
		labels,
	)

	metrics.openConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "open_connections",
			Help:      "The number of established connections both in use and idle",
		},
		labels,
	)

	metrics.inUseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "in_use_connections",
			Help:      "The number of connections currently in use",
		},
		labels,
	)

	metrics.idleConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "idle_connections",
			Help:      "The number of idle connections",
		},
		labels,
	)

	metrics.maxIdleConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_idle_connections",
			Help:      "Maximum number of idle connections",
		},
		labels,
	)

	// Wait metrics
	metrics.waitCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "wait_count_total",
			Help:      "The total number of connections waited for",
		},
		labels,
	)

	metrics.waitDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "wait_duration_seconds_total",
			Help:      "The total time blocked waiting for a new connection",
		},
		labels,
	)

	// Connection close metrics
	metrics.maxIdleClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_idle_closed_total",
			Help:      "The total number of connections closed due to SetMaxIdleConns",
		},
		labels,
	)

	metrics.maxLifetimeClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_lifetime_closed_total",
			Help:      "The total number of connections closed due to SetConnMaxLifetime",
		},
		labels,
	)

	// Health check metrics
	metrics.healthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_total",
			Help:      "Total number of health checks performed",
		},
		labels,
	)

	metrics.healthCheckSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_success_total",
			Help:      "Total number of successful health checks",
		},
		labels,
	)

	metrics.healthCheckFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_failure_total",
			Help:      "Total number of failed health checks",
		},
		labels,
	)

	// Query/transaction metrics
	metrics.queryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "query_duration_seconds",
			Help:      "SQL query duration in seconds",
			Buckets:   []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5},
		},
		append(labels, "op", "status"),
	)

	metrics.txDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "tx_duration_seconds",
			Help:      "Transaction duration in seconds",
			Buckets:   []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5},
		},
		append(labels, "status"),
	)

	metrics.errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total errors by error type",
		},
		append(labels, "error_type"),
	)

	metrics.slowQueryCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "slow_queries_total",
			Help:      "Slow queries counted by op and threshold",
		},
		append(labels, "op", "threshold"),
	)

	// Connection metrics
	metrics.connectAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_attempts_total",
			Help:      "Total number of database connection attempts",
		},
		labels,
	)

	metrics.connectRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_retries_total",
			Help:      "Total number of database connection retries",
		},
		labels,
	)

	metrics.connectSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_success_total",
			Help:      "Total number of successful database connections",
		},
		labels,
	)

	metrics.connectFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_failures_total",
			Help:      "Total number of failed database connection attempts",
		},
		labels,
	)

	// Register all metrics
	metrics.registry.MustRegister(
		metrics.maxOpenConnections,
		metrics.openConnections,
		metrics.inUseConnections,
		metrics.idleConnections,
		metrics.maxIdleConnections,
		metrics.waitCount,
		metrics.waitDuration,
		metrics.maxIdleClosed,
		metrics.maxLifetimeClosed,
		metrics.healthCheckTotal,
		metrics.healthCheckSuccess,
		metrics.healthCheckFailure,
		metrics.queryDuration,
		metrics.txDuration,
		metrics.errorCounter,
		metrics.slowQueryCnt,
		metrics.connectAttempts,
		metrics.connectRetries,
		metrics.connectSuccess,
		metrics.connectFailures,
	)

	return metrics
}

// RecordConnectionPoolStats implements MetricsRecorder
func (pm *PrometheusMetrics) RecordConnectionPoolStats(stats *ConnectionPoolStats) {
	if pm == nil || stats == nil {
		return
	}

	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}

	pm.maxOpenConnections.With(labels).Set(float64(stats.MaxOpenConnections))
	pm.openConnections.With(labels).Set(float64(stats.OpenConnections))
	pm.inUseConnections.With(labels).Set(float64(stats.InUse))
	pm.idleConnections.With(labels).Set(float64(stats.Idle))
	pm.maxIdleConnections.With(labels).Set(float64(stats.MaxIdleConnections))
	pm.waitCount.With(labels).Add(float64(stats.WaitCount))
	pm.waitDuration.With(labels).Add(stats.WaitDuration.Seconds())
	pm.maxIdleClosed.With(labels).Add(float64(stats.MaxIdleClosed))
	pm.maxLifetimeClosed.With(labels).Add(float64(stats.MaxLifetimeClosed))
}

// RecordHealthCheck implements MetricsRecorder
func (pm *PrometheusMetrics) RecordHealthCheck(success bool) {
	if pm == nil {
		return
	}

	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}

	pm.healthCheckTotal.With(labels).Inc()
	if success {
		pm.healthCheckSuccess.With(labels).Inc()
	} else {
		pm.healthCheckFailure.With(labels).Inc()
	}
}

// RecordQuery implements MetricsRecorder
func (pm *PrometheusMetrics) RecordQuery(duration time.Duration, err error, threshold time.Duration) {
	if pm == nil {
		return
	}

	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}

	status := "ok"
	if err != nil {
		status = "error"
	}

	queryLabels := cloneLabels(labels)
	queryLabels["op"] = "query"
	queryLabels["status"] = status
	pm.queryDuration.With(queryLabels).Observe(duration.Seconds())

	if err != nil {
		errorLabels := cloneLabels(labels)
		errorLabels["error_type"] = "query_error"
		pm.errorCounter.With(errorLabels).Inc()
	}

	if threshold > 0 && duration >= threshold {
		slowLabels := cloneLabels(labels)
		slowLabels["op"] = "query"
		slowLabels["threshold"] = threshold.String()
		pm.slowQueryCnt.With(slowLabels).Inc()
	}
}

// RecordTx implements MetricsRecorder
func (pm *PrometheusMetrics) RecordTx(duration time.Duration, committed bool) {
	if pm == nil {
		return
	}

	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}

	txLabels := cloneLabels(labels)
	if committed {
		txLabels["status"] = "commit"
	} else {
		txLabels["status"] = "rollback"
	}
	pm.txDuration.With(txLabels).Observe(duration.Seconds())
}

// IncConnectAttempt implements MetricsRecorder
func (pm *PrometheusMetrics) IncConnectAttempt() {
	if pm == nil {
		return
	}
	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}
	pm.connectAttempts.With(labels).Inc()
}

// IncConnectRetry implements MetricsRecorder
func (pm *PrometheusMetrics) IncConnectRetry() {
	if pm == nil {
		return
	}
	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}
	pm.connectRetries.With(labels).Inc()
}

// IncConnectSuccess implements MetricsRecorder
func (pm *PrometheusMetrics) IncConnectSuccess() {
	if pm == nil {
		return
	}
	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}
	pm.connectSuccess.With(labels).Inc()
}

// IncConnectFailure implements MetricsRecorder
func (pm *PrometheusMetrics) IncConnectFailure() {
	if pm == nil {
		return
	}
	labels := prometheus.Labels{
		"instance": "default",
		"database": "default",
		"driver":   "default",
	}
	pm.connectFailures.With(labels).Inc()
}

// GetGatherer returns the Prometheus gatherer
func (pm *PrometheusMetrics) GetGatherer() prometheus.Gatherer {
	if pm == nil {
		return nil
	}
	return pm.registry
}

// cloneLabels shallow copies labels for appending dimensions
func cloneLabels(in prometheus.Labels) prometheus.Labels {
	out := prometheus.Labels{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
