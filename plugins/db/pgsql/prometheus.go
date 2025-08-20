package pgsql

import (
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusConfig Prometheus 指标语义配置（插件内部私有 registry 用）
type PrometheusConfig struct {
	// Prometheus 指标的命名空间
	Namespace string
	// Prometheus 指标的子系统
	Subsystem string
	// 指标的额外标签（用于构建静态或扩展标签）
	Labels map[string]string
}

// --- Helpers for connection metrics ---

// IncConnectAttempt 增加一次连接尝试
func (pm *PrometheusMetrics) IncConnectAttempt(config *conf.Pgsql) {
    if pm == nil { return }
    labels := pm.buildLabels(config)
    pm.ConnectAttempts.With(labels).Inc()
}

// IncConnectRetry 增加一次连接重试
func (pm *PrometheusMetrics) IncConnectRetry(config *conf.Pgsql) {
    if pm == nil { return }
    labels := pm.buildLabels(config)
    pm.ConnectRetries.With(labels).Inc()
}

// IncConnectSuccess 增加一次连接成功
func (pm *PrometheusMetrics) IncConnectSuccess(config *conf.Pgsql) {
    if pm == nil { return }
    labels := pm.buildLabels(config)
    pm.ConnectSuccess.With(labels).Inc()
}

// IncConnectFailure 增加一次连接失败
func (pm *PrometheusMetrics) IncConnectFailure(config *conf.Pgsql) {
    if pm == nil { return }
    labels := pm.buildLabels(config)
    pm.ConnectFailures.With(labels).Inc()
}

// RecordQuery 记录一次 SQL 查询的耗时、错误和慢查询计数
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

// RecordTx 记录一次事务的耗时与状态
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

// 从配置创建 PrometheusConfig
func createPrometheusConfig(pgsqlConf *conf.Pgsql) *PrometheusConfig {
	// 默认仅配置指标语义，不涉及 HTTP 暴露
	return &PrometheusConfig{
		Namespace: "lynx",
		Subsystem: "pgsql",
		Labels:    make(map[string]string),
	}
}

// PrometheusMetrics Prometheus 监控指标
type PrometheusMetrics struct {
	// 连接池指标
	MaxOpenConnections *prometheus.GaugeVec
	OpenConnections    *prometheus.GaugeVec
	InUseConnections   *prometheus.GaugeVec
	IdleConnections    *prometheus.GaugeVec
	MaxIdleConnections *prometheus.GaugeVec

	// 等待指标
	WaitCount    *prometheus.CounterVec
	WaitDuration *prometheus.CounterVec

	// 连接关闭指标
	MaxIdleClosed     *prometheus.CounterVec
	MaxLifetimeClosed *prometheus.CounterVec

	// 健康检查指标
	HealthCheckTotal   *prometheus.CounterVec
	HealthCheckSuccess *prometheus.CounterVec
	HealthCheckFailure *prometheus.CounterVec

	// 配置指标
	ConfigMinConn *prometheus.GaugeVec
	ConfigMaxConn *prometheus.GaugeVec

	// 注册表
	registry *prometheus.Registry

	// 查询/事务指标
	QueryDuration *prometheus.HistogramVec
	TxDuration    *prometheus.HistogramVec
	ErrorCounter  *prometheus.CounterVec
	SlowQueryCnt  *prometheus.CounterVec

	// 连接重试/尝试/成功/失败指标
	ConnectAttempts *prometheus.CounterVec
	ConnectRetries  *prometheus.CounterVec
	ConnectSuccess  *prometheus.CounterVec
	ConnectFailures *prometheus.CounterVec
}

// NewPrometheusMetrics 创建新的 Prometheus 监控指标
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil {
		return nil
	}

	// 设置默认值
	if config.Namespace == "" {
		config.Namespace = "lynx"
	}
	if config.Subsystem == "" {
		config.Subsystem = "pgsql"
	}

	// 创建标签
	labels := []string{"instance", "database"}
	for key := range config.Labels {
		labels = append(labels, key)
	}

	metrics := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}

	// 连接池指标
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

	// 等待指标
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

	// 连接关闭指标
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

	// 健康检查指标
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

	// 配置指标
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

	// 直方图桶（5ms ~ 5s）
	buckets := []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 1.5, 2, 3, 5}

	// 查询时延直方图
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

	// 事务时延直方图
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

	// 错误码统计（SQLSTATE）
	metrics.ErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total errors by SQLSTATE code",
		},
		append(labels, "sqlstate"),
	)

	// 慢查询计数（按 op、阈值标签）
	metrics.SlowQueryCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "slow_queries_total",
			Help:      "Slow queries counted by op and threshold",
		},
		append(labels, "op", "threshold"),
	)

	// 连接类指标
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

	// 注册所有指标
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

// GetGatherer 返回该插件私有的 Prometheus Gatherer（用于在应用装配阶段统一聚合到全局 /metrics）
func (pm *PrometheusMetrics) GetGatherer() prometheus.Gatherer {
	if pm == nil {
		return nil
	}
	return pm.registry
}

// UpdateMetrics 更新监控指标
func (pm *PrometheusMetrics) UpdateMetrics(stats *ConnectionPoolStats, config *conf.Pgsql) {
	if pm == nil || stats == nil {
		return
	}

	// 构建标签
	labels := pm.buildLabels(config)

	// 更新连接池指标
	pm.MaxOpenConnections.With(labels).Set(float64(stats.MaxOpenConnections))
	pm.OpenConnections.With(labels).Set(float64(stats.OpenConnections))
	pm.InUseConnections.With(labels).Set(float64(stats.InUse))
	pm.IdleConnections.With(labels).Set(float64(stats.Idle))
	pm.MaxIdleConnections.With(labels).Set(float64(stats.MaxIdleConnections))

	// 更新等待指标
	pm.WaitCount.With(labels).Add(float64(stats.WaitCount))
	pm.WaitDuration.With(labels).Add(stats.WaitDuration.Seconds())

	// 更新连接关闭指标
	pm.MaxIdleClosed.With(labels).Add(float64(stats.MaxIdleClosed))
	pm.MaxLifetimeClosed.With(labels).Add(float64(stats.MaxLifetimeClosed))

	// 更新配置指标
	if config != nil {
		pm.ConfigMinConn.With(labels).Set(float64(config.MinConn))
		pm.ConfigMaxConn.With(labels).Set(float64(config.MaxConn))
	}
}

// RecordHealthCheck 记录健康检查结果
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

// buildLabels 构建标签
func (pm *PrometheusMetrics) buildLabels(config *conf.Pgsql) prometheus.Labels {
	labels := prometheus.Labels{
		"instance": "pgsql",
		"database": "postgres",
	}

	// 从连接字符串中提取数据库名称
	if config != nil && config.Source != "" {
		if dbName := pm.extractDatabaseName(config.Source); dbName != "" {
			labels["database"] = dbName
		}
	}

	return labels
}

// cloneLabels 浅拷贝 labels，便于追加维度
func cloneLabels(in prometheus.Labels) prometheus.Labels {
    out := prometheus.Labels{}
    for k, v := range in {
        out[k] = v
    }
    return out
}

// extractDatabaseName 从连接字符串中提取数据库名称
func (pm *PrometheusMetrics) extractDatabaseName(source string) string {
	// 解析 postgres://user:pass@host:port/dbname 格式
	if strings.Contains(source, "://") {
		parts := strings.Split(source, "/")
		if len(parts) >= 2 {
			dbPart := parts[len(parts)-1]
			// 移除查询参数
			if idx := strings.Index(dbPart, "?"); idx != -1 {
				dbPart = dbPart[:idx]
			}
			return dbPart
		}
	}
	return ""
}
