package mssql

import (
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/mssql/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics represents Prometheus monitoring metrics for Microsoft SQL Server
type PrometheusMetrics struct {
	// Connection pool metrics
	connectionPoolMaxOpen   prometheus.Gauge
	connectionPoolOpen      prometheus.Gauge
	connectionPoolInUse     prometheus.Gauge
	connectionPoolIdle      prometheus.Gauge
	connectionPoolWaitCount prometheus.Counter
	connectionPoolWaitTime  prometheus.Histogram
	connectionPoolMaxIdle   prometheus.Gauge

	// Health check metrics
	healthCheckTotal   prometheus.Counter
	healthCheckSuccess prometheus.Counter
	healthCheckFailure prometheus.Counter

	// Configuration metrics
	configMaxConnections    prometheus.Gauge
	configMinConnections    prometheus.Gauge
	configMaxIdleTime       prometheus.Gauge
	configMaxLifetime       prometheus.Gauge
	configEncryptionEnabled prometheus.Gauge
	configConnectionPooling prometheus.Gauge

	// Query/transaction metrics
	queryDuration prometheus.Histogram
	txDuration    prometheus.Histogram
	errorCounter  prometheus.Counter
	slowQueryCnt  prometheus.Counter

	// Connection retry/attempt/success/failure metrics
	connectAttempts prometheus.Counter
	connectRetries  prometheus.Counter
	connectSuccess  prometheus.Counter
	connectFailures prometheus.Counter
}

// PrometheusConfig represents configuration for Prometheus metrics
type PrometheusConfig struct {
	Namespace string
	Subsystem string
	Instance  string
}

// NewPrometheusMetrics creates a new instance of Prometheus metrics
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil {
		config = &PrometheusConfig{
			Namespace: "lynx",
			Subsystem: "mssql",
			Instance:  "default",
		}
	}

	labels := prometheus.Labels{"instance": config.Instance}

	return &PrometheusMetrics{
		// Connection pool metrics
		connectionPoolMaxOpen: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_max_open",
			Help:        "Maximum number of open connections to the database",
			ConstLabels: labels,
		}),
		connectionPoolOpen: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_open",
			Help:        "The number of established connections both in use and idle",
			ConstLabels: labels,
		}),
		connectionPoolInUse: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_in_use",
			Help:        "The number of connections currently in use",
			ConstLabels: labels,
		}),
		connectionPoolIdle: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_idle",
			Help:        "The number of idle connections",
			ConstLabels: labels,
		}),
		connectionPoolWaitCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_wait_count_total",
			Help:        "The total number of connections waited for",
			ConstLabels: labels,
		}),
		connectionPoolWaitTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_wait_duration_seconds",
			Help:        "The total time blocked waiting for a new connection",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
		connectionPoolMaxIdle: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connection_pool_max_idle",
			Help:        "Maximum number of idle connections",
			ConstLabels: labels,
		}),

		// Health check metrics
		healthCheckTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "health_check_total",
			Help:        "Total number of health checks performed",
			ConstLabels: labels,
		}),
		healthCheckSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "health_check_success_total",
			Help:        "Total number of successful health checks",
			ConstLabels: labels,
		}),
		healthCheckFailure: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "health_check_failure_total",
			Help:        "Total number of failed health checks",
			ConstLabels: labels,
		}),

		// Configuration metrics
		configMaxConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_max_connections",
			Help:        "Maximum number of connections configured",
			ConstLabels: labels,
		}),
		configMinConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_min_connections",
			Help:        "Minimum number of connections configured",
			ConstLabels: labels,
		}),
		configMaxIdleTime: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_max_idle_time_seconds",
			Help:        "Maximum idle time configured in seconds",
			ConstLabels: labels,
		}),
		configMaxLifetime: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_max_lifetime_seconds",
			Help:        "Maximum lifetime configured in seconds",
			ConstLabels: labels,
		}),
		configEncryptionEnabled: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_encryption_enabled",
			Help:        "Whether encryption is enabled (1) or disabled (0)",
			ConstLabels: labels,
		}),
		configConnectionPooling: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "config_connection_pooling",
			Help:        "Whether connection pooling is enabled (1) or disabled (0)",
			ConstLabels: labels,
		}),

		// Query/transaction metrics
		queryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "query_duration_seconds",
			Help:        "Duration of SQL queries in seconds",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
		txDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "tx_duration_seconds",
			Help:        "Duration of SQL transactions in seconds",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
		errorCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "error_count_total",
			Help:        "Total number of errors encountered",
			ConstLabels: labels,
		}),
		slowQueryCnt: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "slow_query_count_total",
			Help:        "Total number of slow queries",
			ConstLabels: labels,
		}),

		// Connection retry/attempt/success/failure metrics
		connectAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connect_attempts_total",
			Help:        "Total number of connection attempts",
			ConstLabels: labels,
		}),
		connectRetries: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connect_retries_total",
			Help:        "Total number of connection retries",
			ConstLabels: labels,
		}),
		connectSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connect_success_total",
			Help:        "Total number of successful connections",
			ConstLabels: labels,
		}),
		connectFailures: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "connect_failures_total",
			Help:        "Total number of failed connections",
			ConstLabels: labels,
		}),
	}
}

// RecordConnectionPoolStats records connection pool statistics
func (pm *PrometheusMetrics) RecordConnectionPoolStats(stats *base.ConnectionPoolStats) {
	if pm == nil || stats == nil {
		return
	}

	pm.connectionPoolMaxOpen.Set(float64(stats.MaxOpenConnections))
	pm.connectionPoolOpen.Set(float64(stats.OpenConnections))
	pm.connectionPoolInUse.Set(float64(stats.InUse))
	pm.connectionPoolIdle.Set(float64(stats.Idle))
	pm.connectionPoolMaxIdle.Set(float64(stats.MaxIdleConnections))
	pm.connectionPoolWaitCount.Add(float64(stats.WaitCount))
	pm.connectionPoolWaitTime.Observe(stats.WaitDuration.Seconds())
}

// RecordHealthCheck records health check results
func (pm *PrometheusMetrics) RecordHealthCheck(success bool, config *conf.Mssql) {
	if pm == nil {
		return
	}

	pm.healthCheckTotal.Inc()
	if success {
		pm.healthCheckSuccess.Inc()
	} else {
		pm.healthCheckFailure.Inc()
	}

	// Record configuration metrics
	if config != nil {
		pm.configMaxConnections.Set(float64(config.MaxConn))
		pm.configMinConnections.Set(float64(config.MinConn))

		if config.MaxIdleTime != nil {
			pm.configMaxIdleTime.Set(float64(config.MaxIdleTime.Seconds))
		}

		if config.MaxLifeTime != nil {
			pm.configMaxLifetime.Set(float64(config.MaxLifeTime.Seconds))
		}

		if config.ServerConfig != nil {
			if config.ServerConfig.Encrypt {
				pm.configEncryptionEnabled.Set(1)
			} else {
				pm.configEncryptionEnabled.Set(0)
			}

			if config.ServerConfig.ConnectionPooling {
				pm.configConnectionPooling.Set(1)
			} else {
				pm.configConnectionPooling.Set(0)
			}
		}
	}
}

// createPrometheusConfig creates Prometheus configuration from MSSQL config
func createPrometheusConfig(config *conf.Mssql) *PrometheusConfig {
	if config == nil {
		return &PrometheusConfig{
			Namespace: "lynx",
			Subsystem: "mssql",
			Instance:  "default",
		}
	}

	instance := "default"
	if config.ServerConfig != nil && config.ServerConfig.InstanceName != "" {
		instance = config.ServerConfig.InstanceName
	}

	return &PrometheusConfig{
		Namespace: "lynx",
		Subsystem: "mssql",
		Instance:  instance,
	}
}
