package mysql

import (
	conf "github.com/go-lynx/lynx/api/plugins/sql/mysql"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics holds all Prometheus metrics for MySQL
type PrometheusMetrics struct {
	registry *prometheus.Registry

	// Connection pool metrics
	maxOpenConnections *prometheus.GaugeVec
	openConnections    *prometheus.GaugeVec
	inUseConnections   *prometheus.GaugeVec
	idleConnections    *prometheus.GaugeVec

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

	// Configuration metrics
	configMinConnections *prometheus.GaugeVec
	configMaxConnections *prometheus.GaugeVec
}

// PrometheusConfig configuration for Prometheus metrics
type PrometheusConfig struct {
	Namespace string
	Subsystem string
	Labels    map[string]string
}

// NewPrometheusMetrics creates new Prometheus metrics instance
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil {
		config = &PrometheusConfig{
			Namespace: "lynx",
			Subsystem: "mysql",
			Labels:    make(map[string]string),
		}
	}

	registry := prometheus.NewRegistry()

	labelNames := make([]string, 0, len(config.Labels))
	for k := range config.Labels {
		labelNames = append(labelNames, k)
	}

	m := &PrometheusMetrics{
		registry: registry,

		// Connection pool metrics
		maxOpenConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "max_open_connections",
				Help:      "Maximum number of open connections to the database",
			},
			labelNames,
		),
		openConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "open_connections",
				Help:      "The number of established connections both in use and idle",
			},
			labelNames,
		),
		inUseConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "in_use_connections",
				Help:      "The number of connections currently in use",
			},
			labelNames,
		),
		idleConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "idle_connections",
				Help:      "The number of idle connections",
			},
			labelNames,
		),

		// Wait metrics
		waitCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "wait_count_total",
				Help:      "The total number of connections waited for",
			},
			labelNames,
		),
		waitDuration: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "wait_duration_seconds_total",
				Help:      "The total time blocked waiting for a new connection",
			},
			labelNames,
		),

		// Connection close metrics
		maxIdleClosed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "max_idle_closed_total",
				Help:      "The total number of connections closed due to SetMaxIdleConns",
			},
			labelNames,
		),
		maxLifetimeClosed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "max_lifetime_closed_total",
				Help:      "The total number of connections closed due to SetConnMaxLifetime",
			},
			labelNames,
		),

		// Health check metrics
		healthCheckTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "health_check_total",
				Help:      "Total number of health checks performed",
			},
			labelNames,
		),
		healthCheckSuccess: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "health_check_success_total",
				Help:      "Total number of successful health checks",
			},
			labelNames,
		),
		healthCheckFailure: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "health_check_failure_total",
				Help:      "Total number of failed health checks",
			},
			labelNames,
		),

		// Configuration metrics
		configMinConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "config_min_connections",
				Help:      "Configured minimum number of connections",
			},
			labelNames,
		),
		configMaxConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "config_max_connections",
				Help:      "Configured maximum number of connections",
			},
			labelNames,
		),
	}

	// Register all metrics
	registry.MustRegister(
		m.maxOpenConnections,
		m.openConnections,
		m.inUseConnections,
		m.idleConnections,
		m.waitCount,
		m.waitDuration,
		m.maxIdleClosed,
		m.maxLifetimeClosed,
		m.healthCheckTotal,
		m.healthCheckSuccess,
		m.healthCheckFailure,
		m.configMinConnections,
		m.configMaxConnections,
	)

	return m
}

// UpdateMetrics updates all metrics with current stats
func (m *PrometheusMetrics) UpdateMetrics(stats *base.ConnectionPoolStats, config *conf.Mysql) {
	if m == nil || stats == nil || config == nil {
		return
	}

	labels := prometheus.Labels{}

	// Update connection pool metrics
	m.maxOpenConnections.With(labels).Set(float64(stats.MaxOpenConnections))
	m.openConnections.With(labels).Set(float64(stats.OpenConnections))
	m.inUseConnections.With(labels).Set(float64(stats.InUse))
	m.idleConnections.With(labels).Set(float64(stats.Idle))

	// Update wait metrics
	m.waitCount.With(labels).Add(float64(stats.WaitCount))
	m.waitDuration.With(labels).Add(stats.WaitDuration.Seconds())

	// Update connection close metrics
	m.maxIdleClosed.With(labels).Add(float64(stats.MaxIdleClosed))
	m.maxLifetimeClosed.With(labels).Add(float64(stats.MaxLifetimeClosed))

	// Update configuration metrics
	m.configMinConnections.With(labels).Set(float64(config.MinConn))
	m.configMaxConnections.With(labels).Set(float64(config.MaxConn))
}

// RecordHealthCheck records health check result
func (m *PrometheusMetrics) RecordHealthCheck(success bool, config *conf.Mysql) {
	if m == nil {
		return
	}

	labels := prometheus.Labels{}

	m.healthCheckTotal.With(labels).Inc()
	if success {
		m.healthCheckSuccess.With(labels).Inc()
	} else {
		m.healthCheckFailure.With(labels).Inc()
	}
}

// GetGatherer returns the Prometheus gatherer
func (m *PrometheusMetrics) GetGatherer() prometheus.Gatherer {
	if m == nil || m.registry == nil {
		return nil
	}
	return m.registry
}

// createPrometheusConfig creates Prometheus configuration from MySQL config
func createPrometheusConfig(config *conf.Mysql) *PrometheusConfig {
	promConfig := &PrometheusConfig{
		Namespace: "lynx",
		Subsystem: "mysql",
		Labels:    make(map[string]string),
	}

	// Add default labels if needed
	if config.GetDriver() != "" {
		promConfig.Labels["driver"] = config.GetDriver()
	}

	return promConfig
}
