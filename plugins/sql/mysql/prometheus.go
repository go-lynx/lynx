package mysql

import (
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/mysql/conf"
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

	// Configuration metrics
	configMinConnections *prometheus.GaugeVec
	configMaxConnections *prometheus.GaugeVec

	// Query/transaction metrics
	queryDuration *prometheus.HistogramVec
	txDuration    *prometheus.HistogramVec
	errorCounter  *prometheus.CounterVec
	slowQueryCnt  *prometheus.CounterVec

	// Connection retry/attempt/success/failure metrics
	connectAttempts *prometheus.CounterVec
	connectRetries  *prometheus.CounterVec
	connectSuccess  *prometheus.CounterVec
	connectFailures *prometheus.CounterVec
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

	// Add default labels
	labelNames = append(labelNames, "instance", "database")

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
		maxIdleConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "max_idle_connections",
				Help:      "Maximum number of idle connections",
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

		// Query/transaction metrics
		queryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "query_duration_seconds",
				Help:      "SQL query duration in seconds",
				Buckets:   []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5},
			},
			append(labelNames, "op", "status"),
		),
		txDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "tx_duration_seconds",
				Help:      "Transaction duration in seconds",
				Buckets:   []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5},
			},
			append(labelNames, "status"),
		),
		errorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "errors_total",
				Help:      "Total errors by error type",
			},
			append(labelNames, "error_type"),
		),
		slowQueryCnt: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "slow_queries_total",
				Help:      "Slow queries counted by op and threshold",
			},
			append(labelNames, "op", "threshold"),
		),

		// Connection metrics
		connectAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "connect_attempts_total",
				Help:      "Total number of database connection attempts",
			},
			labelNames,
		),
		connectRetries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "connect_retries_total",
				Help:      "Total number of database connection retries",
			},
			labelNames,
		),
		connectSuccess: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "connect_success_total",
				Help:      "Total number of successful database connections",
			},
			labelNames,
		),
		connectFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "connect_failures_total",
				Help:      "Total number of failed database connection attempts",
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
		m.maxIdleConnections,
		m.waitCount,
		m.waitDuration,
		m.maxIdleClosed,
		m.maxLifetimeClosed,
		m.healthCheckTotal,
		m.healthCheckSuccess,
		m.healthCheckFailure,
		m.configMinConnections,
		m.configMaxConnections,
		m.queryDuration,
		m.txDuration,
		m.errorCounter,
		m.slowQueryCnt,
		m.connectAttempts,
		m.connectRetries,
		m.connectSuccess,
		m.connectFailures,
	)

	return m
}

// UpdateMetrics updates all metrics with current stats
func (m *PrometheusMetrics) UpdateMetrics(stats *base.ConnectionPoolStats, config *conf.Mysql) {
	if m == nil || stats == nil || config == nil {
		return
	}

	labels := m.buildLabels(config)

	// Update connection pool metrics
	m.maxOpenConnections.With(labels).Set(float64(stats.MaxOpenConnections))
	m.openConnections.With(labels).Set(float64(stats.OpenConnections))
	m.inUseConnections.With(labels).Set(float64(stats.InUse))
	m.idleConnections.With(labels).Set(float64(stats.Idle))
	m.maxIdleConnections.With(labels).Set(float64(stats.MaxIdleConnections))

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

	labels := m.buildLabels(config)

	m.healthCheckTotal.With(labels).Inc()
	if success {
		m.healthCheckSuccess.With(labels).Inc()
	} else {
		m.healthCheckFailure.With(labels).Inc()
	}
}

// RecordQuery records SQL query duration, errors and slow query count
func (m *PrometheusMetrics) RecordQuery(op string, dur time.Duration, err error, threshold time.Duration, config *conf.Mysql, errorType string) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	status := "ok"
	if err != nil {
		status = "error"
	}
	l := cloneLabels(labels)
	l["op"] = op
	l["status"] = status
	m.queryDuration.With(l).Observe(dur.Seconds())

	if err != nil {
		le := cloneLabels(labels)
		if errorType == "" {
			errorType = "unknown"
		}
		le["error_type"] = errorType
		m.errorCounter.With(le).Inc()
	}

	if threshold > 0 && dur >= threshold {
		ls := cloneLabels(labels)
		ls["op"] = op
		ls["threshold"] = threshold.String()
		m.slowQueryCnt.With(ls).Inc()
	}
}

// RecordTx records transaction duration and status
func (m *PrometheusMetrics) RecordTx(dur time.Duration, committed bool, config *conf.Mysql) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	l := cloneLabels(labels)
	if committed {
		l["status"] = "commit"
	} else {
		l["status"] = "rollback"
	}
	m.txDuration.With(l).Observe(dur.Seconds())
}

// IncConnectAttempt increments connection attempt counter
func (m *PrometheusMetrics) IncConnectAttempt(config *conf.Mysql) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	m.connectAttempts.With(labels).Inc()
}

// IncConnectRetry increments connection retry counter
func (m *PrometheusMetrics) IncConnectRetry(config *conf.Mysql) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	m.connectRetries.With(labels).Inc()
}

// IncConnectSuccess increments connection success counter
func (m *PrometheusMetrics) IncConnectSuccess(config *conf.Mysql) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	m.connectSuccess.With(labels).Inc()
}

// IncConnectFailure increments connection failure counter
func (m *PrometheusMetrics) IncConnectFailure(config *conf.Mysql) {
	if m == nil {
		return
	}
	labels := m.buildLabels(config)
	m.connectFailures.With(labels).Inc()
}

// GetGatherer returns the Prometheus gatherer
func (m *PrometheusMetrics) GetGatherer() prometheus.Gatherer {
	if m == nil || m.registry == nil {
		return nil
	}
	return m.registry
}

// buildLabels builds labels for metrics
func (m *PrometheusMetrics) buildLabels(config *conf.Mysql) prometheus.Labels {
	labels := prometheus.Labels{
		"instance": "mysql",
		"database": "mysql",
	}

	// Extract database name from DSN if available
	if config != nil && config.Source != "" {
		if dbName := m.extractDatabaseName(config.Source); dbName != "" {
			labels["database"] = dbName
		}
	}

	return labels
}

// cloneLabels shallow copies labels for appending dimensions
func cloneLabels(in prometheus.Labels) prometheus.Labels {
	out := prometheus.Labels{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// extractDatabaseName extracts database name from DSN
func (m *PrometheusMetrics) extractDatabaseName(dsn string) string {
	// Parse mysql://user:pass@host:port/dbname format
	if len(dsn) > 0 {
		// Simple extraction - look for database name after last slash
		parts := strings.Split(dsn, "/")
		if len(parts) >= 2 {
			dbPart := parts[len(parts)-1]
			// Remove query parameters
			if idx := strings.Index(dbPart, "?"); idx != -1 {
				dbPart = dbPart[:idx]
			}
			return dbPart
		}
	}
	return ""
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
