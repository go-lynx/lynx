package pgsql

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusConfig Prometheus 监控配置
type PrometheusConfig struct {
	// 是否启用 Prometheus 指标导出
	Enabled bool
	// 指标暴露的 HTTP 端点路径
	MetricsPath string
	// 指标 HTTP 服务器的端口
	MetricsPort int32
	// Prometheus 指标的命名空间
	Namespace string
	// Prometheus 指标的子系统
	Subsystem string
	// 指标的额外标签
	Labels map[string]string
}

// 从配置创建 PrometheusConfig
func createPrometheusConfig(pgsqlConf *conf.Pgsql) *PrometheusConfig {
	// 默认配置
	config := &PrometheusConfig{
		Enabled:     false,
		MetricsPath: "/metrics",
		MetricsPort: 9090,
		Namespace:   "lynx",
		Subsystem:   "pgsql",
		Labels:      make(map[string]string),
	}

	// 这里可以从环境变量或配置文件读取 Prometheus 配置
	// 暂时使用默认配置，后续可以通过配置文件扩展
	return config
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
}

// NewPrometheusMetrics 创建新的 Prometheus 监控指标
func NewPrometheusMetrics(config *PrometheusConfig) *PrometheusMetrics {
	if config == nil || !config.Enabled {
		return nil
	}

	// 设置默认值
	if config.Namespace == "" {
		config.Namespace = "lynx"
	}
	if config.Subsystem == "" {
		config.Subsystem = "pgsql"
	}
	if config.MetricsPath == "" {
		config.MetricsPath = "/metrics"
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = 9090
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
	)

	// 额外：同时注册到默认全局注册表，便于由统一 Handler 聚合暴露
	prometheus.MustRegister(
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
	)

	return metrics
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

// StartMetricsServer 启动指标服务器
func (pm *PrometheusMetrics) StartMetricsServer(config *PrometheusConfig) {
	if pm == nil || !config.Enabled {
		return
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle(config.MetricsPath, promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.MetricsPort),
		Handler: mux,
	}

	log.Infof("启动 Prometheus 指标服务器: http://localhost:%d%s", config.MetricsPort, config.MetricsPath)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Prometheus 指标服务器启动失败: %v", err)
		}
	}()
}

// GetMetricsHandler 获取指标处理器
func (pm *PrometheusMetrics) GetMetricsHandler() http.Handler {
	if pm == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{})
}
