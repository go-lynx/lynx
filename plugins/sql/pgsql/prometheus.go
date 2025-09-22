package pgsql

import (
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/sql/base"

	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusConfig Prometheus metric semantic configuration (for plugin internal private registry)
type PrometheusConfig struct {
	// Prometheus metric namespace
	Namespace string
	// Prometheus metric subsystem
	Subsystem string
	// Additional labels for metrics (used to build static or extended labels)
	Labels map[string]string
}

// IncConnectAttempt Increments connection attempt counter
func (pm *PrometheusMetrics) IncConnectAttempt(config *conf.Pgsql) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	pm.ConnectAttempts.With(labels).Inc()
}

// IncConnectRetry Increments connection retry counter
func (pm *PrometheusMetrics) IncConnectRetry(config *conf.Pgsql) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	pm.ConnectRetries.With(labels).Inc()
}

// IncConnectSuccess Increments connection success counter
func (pm *PrometheusMetrics) IncConnectSuccess(config *conf.Pgsql) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	pm.ConnectSuccess.With(labels).Inc()
}

// IncConnectFailure Increments connection failure counter
func (pm *PrometheusMetrics) IncConnectFailure(config *conf.Pgsql) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	pm.ConnectFailures.With(labels).Inc()
}

// RecordQuery Records SQL query duration, errors and slow query count
func (pm *PrometheusMetrics) RecordQuery(op string, dur time.Duration, err error, threshold time.Duration, config *conf.Pgsql, sqlState string) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	status := "ok"
	if err != nil {
		status = "error"
	}
	l := cloneLabels(labels)
	l["op"] = op
	l["status"] = status
	pm.QueryDuration.With(l).Observe(dur.Seconds())

	if err != nil {
		le := cloneLabels(labels)
		if sqlState == "" {
			sqlState = "unknown"
		}
		le["sqlstate"] = sqlState
		pm.ErrorCounter.With(le).Inc()
	}

	if threshold > 0 && dur >= threshold {
		ls := cloneLabels(labels)
		ls["op"] = op
		ls["threshold"] = threshold.String()
		pm.SlowQueryCnt.With(ls).Inc()
	}
}

// RecordTx Records transaction duration and status
func (pm *PrometheusMetrics) RecordTx(dur time.Duration, committed bool, config *conf.Pgsql) {
	if pm == nil {
		return
	}
	labels := pm.buildLabels(config)
	l := cloneLabels(labels)
	if committed {
		l["status"] = "commit"
	} else {
		l["status"] = "rollback"
	}
	pm.TxDuration.With(l).Observe(dur.Seconds())
}

// Create PrometheusConfig from configuration
func createPrometheusConfig(pgsqlConf *conf.Pgsql) *PrometheusConfig {
	// Default only configures metric semantics, does not involve HTTP exposure
	return &PrometheusConfig{
		Namespace: "lynx",
		Subsystem: "pgsql",
		Labels:    make(map[string]string),
	}
}

// PrometheusMetrics Prometheus monitoring metrics
type PrometheusMetrics struct {
	// Connection pool metrics
	MaxOpenConnections *prometheus.GaugeVec
	OpenConnections    *prometheus.GaugeVec
	InUseConnections   *prometheus.GaugeVec
	IdleConnections    *prometheus.GaugeVec
	MaxIdleConnections *prometheus.GaugeVec

	// Wait metrics
	WaitCount    *prometheus.CounterVec
	WaitDuration *prometheus.CounterVec

	// Connection close metrics
	MaxIdleClosed     *prometheus.CounterVec
	MaxLifetimeClosed *prometheus.CounterVec

	// Health check metrics
	HealthCheckTotal   *prometheus.CounterVec
	HealthCheckSuccess *prometheus.CounterVec
	HealthCheckFailure *prometheus.CounterVec

	// Configuration metrics
	ConfigMinConn *prometheus.GaugeVec
	ConfigMaxConn *prometheus.GaugeVec

	// Registry
	registry *prometheus.Registry

	// Query/transaction metrics
	QueryDuration *prometheus.HistogramVec
	TxDuration    *prometheus.HistogramVec
	ErrorCounter  *prometheus.CounterVec
	SlowQueryCnt  *prometheus.CounterVec

	// Connection retry/attempt/success/failure metrics
	ConnectAttempts *prometheus.CounterVec
	ConnectRetries  *prometheus.CounterVec
	ConnectSuccess  *prometheus.CounterVec
	ConnectFailures *prometheus.CounterVec
}

// NewPrometheusMetrics Creates new Prometheus monitoring metrics
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil {
		return nil
	}

	// Set default values
	if config.Namespace == "" {
		config.Namespace = "lynx"
	}
	if config.Subsystem == "" {
		config.Subsystem = "pgsql"
	}

	// Create labels
	labels := []string{"instance", "database"}
	for key := range config.Labels {
		labels = append(labels, key)
	}

	metrics := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}

	// Connection pool metrics
	metrics.MaxOpenConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_open_connections",
			Help:      "Maximum number of open connections to the database",
		},
		labels,
	)

	metrics.OpenConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "open_connections",
			Help:      "The number of established connections both in use and idle",
		},
		labels,
	)

	metrics.InUseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "in_use_connections",
			Help:      "The number of connections currently in use",
		},
		labels,
	)

	metrics.IdleConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "idle_connections",
			Help:      "The number of idle connections",
		},
		labels,
	)

	metrics.MaxIdleConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_idle_connections",
			Help:      "Maximum number of idle connections",
		},
		labels,
	)

	// Wait metrics
	metrics.WaitCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "wait_count_total",
			Help:      "The total number of connections waited for",
		},
		labels,
	)

	metrics.WaitDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "wait_duration_seconds_total",
			Help:      "The total time blocked waiting for a new connection",
		},
		labels,
	)

	// Connection close metrics
	metrics.MaxIdleClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_idle_closed_total",
			Help:      "The total number of connections closed due to SetMaxIdleConns",
		},
		labels,
	)

	metrics.MaxLifetimeClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "max_lifetime_closed_total",
			Help:      "The total number of connections closed due to SetConnMaxLifetime",
		},
		labels,
	)

	// Health check metrics
	metrics.HealthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_total",
			Help:      "Total number of health checks performed",
		},
		labels,
	)

	metrics.HealthCheckSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_success_total",
			Help:      "Total number of successful health checks",
		},
		labels,
	)

	metrics.HealthCheckFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "health_check_failure_total",
			Help:      "Total number of failed health checks",
		},
		labels,
	)

	// Configuration metrics
	metrics.ConfigMinConn = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "config_min_connections",
			Help:      "Configured minimum number of connections",
		},
		labels,
	)

	metrics.ConfigMaxConn = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "config_max_connections",
			Help:      "Configured maximum number of connections",
		},
		labels,
	)

	// Histogram buckets (5ms ~ 5s)
	buckets := []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5}

	// Query duration histogram
	metrics.QueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "query_duration_seconds",
			Help:      "SQL query duration in seconds",
			Buckets:   buckets,
		},
		append(labels, "op", "status"),
	)

	// Transaction duration histogram
	metrics.TxDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "tx_duration_seconds",
			Help:      "Transaction duration in seconds",
			Buckets:   buckets,
		},
		append(labels, "status"),
	)

	// Error code statistics (SQLSTATE)
	metrics.ErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total errors by SQLSTATE code",
		},
		append(labels, "sqlstate"),
	)

	// Slow query count (by op, threshold labels)
	metrics.SlowQueryCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "slow_queries_total",
			Help:      "Slow queries counted by op and threshold",
		},
		append(labels, "op", "threshold"),
	)

	// Connection metrics
	metrics.ConnectAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_attempts_total",
			Help:      "Total number of database connection attempts",
		},
		labels,
	)

	metrics.ConnectRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_retries_total",
			Help:      "Total number of database connection retries",
		},
		labels,
	)

	metrics.ConnectSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "connect_success_total",
			Help:      "Total number of successful database connections",
		},
		labels,
	)

	metrics.ConnectFailures = prometheus.NewCounterVec(
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
		metrics.MaxOpenConnections,
		metrics.OpenConnections,
		metrics.InUseConnections,
		metrics.IdleConnections,
		metrics.MaxIdleConnections,
		metrics.WaitCount,
		metrics.WaitDuration,
		metrics.MaxIdleClosed,
		metrics.MaxLifetimeClosed,
		metrics.HealthCheckTotal,
		metrics.HealthCheckSuccess,
		metrics.HealthCheckFailure,
		metrics.ConfigMinConn,
		metrics.ConfigMaxConn,
		metrics.QueryDuration,
		metrics.TxDuration,
		metrics.ErrorCounter,
		metrics.SlowQueryCnt,
		metrics.ConnectAttempts,
		metrics.ConnectRetries,
		metrics.ConnectSuccess,
		metrics.ConnectFailures,
	)

	return metrics
}

// GetGatherer Returns the plugin's private Prometheus Gatherer (used to aggregate to global /metrics during application assembly phase)
func (pm *PrometheusMetrics) GetGatherer() prometheus.Gatherer {
	if pm == nil {
		return nil
	}
	return pm.registry
}

// UpdateMetrics Updates monitoring metrics
func (m *PrometheusMetrics) UpdateMetrics(stats *base.ConnectionPoolStats, config *conf.Pgsql) {
	if m == nil || stats == nil {
		return
	}

	// Build labels
	labels := m.buildLabels(config)

	// Update connection pool metrics
	m.MaxOpenConnections.With(labels).Set(float64(stats.MaxOpenConnections))
	m.OpenConnections.With(labels).Set(float64(stats.OpenConnections))
	m.InUseConnections.With(labels).Set(float64(stats.InUse))
	m.IdleConnections.With(labels).Set(float64(stats.Idle))
	m.MaxIdleConnections.With(labels).Set(float64(stats.MaxIdleConnections))

	// Update wait metrics
	m.WaitCount.With(labels).Add(float64(stats.WaitCount))
	m.WaitDuration.With(labels).Add(stats.WaitDuration.Seconds())

	// Update connection close metrics
	m.MaxIdleClosed.With(labels).Add(float64(stats.MaxIdleClosed))
	m.MaxLifetimeClosed.With(labels).Add(float64(stats.MaxLifetimeClosed))

	// Update configuration metrics
	if config != nil {
		m.ConfigMinConn.With(labels).Set(float64(config.MinConn))
		m.ConfigMaxConn.With(labels).Set(float64(config.MaxConn))
	}
}

// RecordHealthCheck Records health check results
func (pm *PrometheusMetrics) RecordHealthCheck(success bool, config *conf.Pgsql) {
	if pm == nil {
		return
	}

	labels := pm.buildLabels(config)
	pm.HealthCheckTotal.With(labels).Inc()

	if success {
		pm.HealthCheckSuccess.With(labels).Inc()
	} else {
		pm.HealthCheckFailure.With(labels).Inc()
	}
}

// buildLabels Builds labels
func (pm *PrometheusMetrics) buildLabels(config *conf.Pgsql) prometheus.Labels {
	labels := prometheus.Labels{
		"instance": "pgsql",
		"database": "postgres",
	}

	// Extract database name from connection string
	if config != nil && config.Source != "" {
		if dbName := pm.extractDatabaseName(config.Source); dbName != "" {
			labels["database"] = dbName
		}
	}

	return labels
}

// cloneLabels Shallow copies labels for appending dimensions
func cloneLabels(in prometheus.Labels) prometheus.Labels {
	out := prometheus.Labels{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// extractDatabaseName Extracts database name from connection string
func (pm *PrometheusMetrics) extractDatabaseName(source string) string {
	// Parse postgres://user:pass@host:port/dbname format
	if strings.Contains(source, "://") {
		parts := strings.Split(source, "/")
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
