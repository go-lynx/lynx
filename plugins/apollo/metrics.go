package apollo

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics defines Apollo-related monitoring metrics
type Metrics struct {
	// Client operation metrics
	clientOperationsTotal    *prometheus.CounterVec
	clientOperationsDuration *prometheus.HistogramVec
	clientErrorsTotal        *prometheus.CounterVec

	// Configuration management metrics
	configOperationsTotal    *prometheus.CounterVec
	configOperationsDuration *prometheus.HistogramVec
	configChangesTotal       *prometheus.CounterVec

	// Notification metrics
	notificationTotal    *prometheus.CounterVec
	notificationDuration *prometheus.HistogramVec
	notificationErrors   *prometheus.CounterVec

	// Health check metrics
	healthCheckTotal    *prometheus.CounterVec
	healthCheckDuration *prometheus.HistogramVec
	healthCheckFailed   *prometheus.CounterVec

	// Connection metrics
	connectionTotal       *prometheus.GaugeVec
	connectionErrorsTotal *prometheus.CounterVec

	// Cache metrics
	cacheHitsTotal   *prometheus.CounterVec
	cacheMissesTotal *prometheus.CounterVec
}

// NewApolloMetrics creates new monitoring metrics instance
func NewApolloMetrics() *Metrics {
	return &Metrics{
		// Client operation metrics
		clientOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "client_operations_total",
				Help:      "Total number of client operations",
			},
			[]string{"operation", "status"},
		),
		clientOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "client_operations_duration_seconds",
				Help:      "Duration of client operations",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		clientErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "client_errors_total",
				Help:      "Total number of client errors",
			},
			[]string{"operation", "error_type"},
		),

		// Configuration management metrics
		configOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "config_operations_total",
				Help:      "Total number of configuration operations",
			},
			[]string{"namespace", "operation", "status"},
		),
		configOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "config_operations_duration_seconds",
				Help:      "Duration of configuration operations",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"namespace", "operation"},
		),
		configChangesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "config_changes_total",
				Help:      "Total number of configuration changes",
			},
			[]string{"namespace"},
		),

		// Notification metrics
		notificationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "notification_total",
				Help:      "Total number of notifications",
			},
			[]string{"namespace", "status"},
		),
		notificationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "notification_duration_seconds",
				Help:      "Duration of notification operations",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"namespace"},
		),
		notificationErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "notification_errors_total",
				Help:      "Total number of notification errors",
			},
			[]string{"namespace", "error_type"},
		),

		// Health check metrics
		healthCheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "health_check_total",
				Help:      "Total number of health checks",
			},
			[]string{"status"},
		),
		healthCheckDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "health_check_duration_seconds",
				Help:      "Duration of health checks",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{},
		),
		healthCheckFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "health_check_failed_total",
				Help:      "Total number of failed health checks",
			},
			[]string{"error_type"},
		),

		// Connection metrics
		connectionTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "connection_total",
				Help:      "Total number of connections",
			},
			[]string{"status"},
		),
		connectionErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "connection_errors_total",
				Help:      "Total number of connection errors",
			},
			[]string{"error_type"},
		),

		// Cache metrics
		cacheHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "cache_hits_total",
				Help:      "Total number of cache hits",
			},
			[]string{"namespace"},
		),
		cacheMissesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "apollo",
				Name:      "cache_misses_total",
				Help:      "Total number of cache misses",
			},
			[]string{"namespace"},
		),
	}
}

// RecordClientOperation records client operation
func (m *Metrics) RecordClientOperation(operation, status string) {
	if m == nil {
		return
	}
	m.clientOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordConfigOperation records configuration operation
func (m *Metrics) RecordConfigOperation(namespace, operation, status string) {
	if m == nil {
		return
	}
	m.configOperationsTotal.WithLabelValues(namespace, operation, status).Inc()
}

// RecordConfigChange records configuration change
func (m *Metrics) RecordConfigChange(namespace string) {
	if m == nil {
		return
	}
	m.configChangesTotal.WithLabelValues(namespace).Inc()
}

// RecordNotification records notification
func (m *Metrics) RecordNotification(namespace, status string) {
	if m == nil {
		return
	}
	m.notificationTotal.WithLabelValues(namespace, status).Inc()
}

// RecordHealthCheck records health check
func (m *Metrics) RecordHealthCheck(status string) {
	if m == nil {
		return
	}
	m.healthCheckTotal.WithLabelValues(status).Inc()
}

// RecordConnectionError records connection error
func (m *Metrics) RecordConnectionError(errorType string) {
	if m == nil {
		return
	}
	m.connectionErrorsTotal.WithLabelValues(errorType).Inc()
}

// RecordCacheHit records cache hit
func (m *Metrics) RecordCacheHit(namespace string) {
	if m == nil {
		return
	}
	m.cacheHitsTotal.WithLabelValues(namespace).Inc()
}

// RecordCacheMiss records cache miss
func (m *Metrics) RecordCacheMiss(namespace string) {
	if m == nil {
		return
	}
	m.cacheMissesTotal.WithLabelValues(namespace).Inc()
}

