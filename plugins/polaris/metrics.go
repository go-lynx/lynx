package polaris

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 定义 Polaris 相关的监控指标
type Metrics struct {
	// SDK 操作指标
	sdkOperationsTotal    *prometheus.CounterVec
	sdkOperationsDuration *prometheus.HistogramVec
	sdkErrorsTotal        *prometheus.CounterVec

	// 服务发现指标
	serviceDiscoveryTotal    *prometheus.CounterVec
	serviceDiscoveryDuration *prometheus.HistogramVec
	serviceInstancesTotal    *prometheus.GaugeVec

	// 服务注册指标
	serviceRegistrationTotal    *prometheus.CounterVec
	serviceRegistrationDuration *prometheus.HistogramVec
	serviceHeartbeatTotal       *prometheus.CounterVec

	// 配置管理指标
	configOperationsTotal    *prometheus.CounterVec
	configOperationsDuration *prometheus.HistogramVec
	configChangesTotal       *prometheus.CounterVec

	// 路由指标
	routeOperationsTotal    *prometheus.CounterVec
	routeOperationsDuration *prometheus.HistogramVec

	// 限流指标
	rateLimitRequestsTotal *prometheus.CounterVec
	rateLimitRejectedTotal *prometheus.CounterVec
	rateLimitQuotaUsed     *prometheus.GaugeVec

	// 健康检查指标
	healthCheckTotal    *prometheus.CounterVec
	healthCheckDuration *prometheus.HistogramVec
	healthCheckFailed   *prometheus.CounterVec

	// 连接指标
	connectionTotal       *prometheus.GaugeVec
	connectionErrorsTotal *prometheus.CounterVec
}

// NewPolarisMetrics 创建新的监控指标实例
func NewPolarisMetrics() *Metrics {
	return &Metrics{
		// SDK 操作指标
		sdkOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_sdk_operations_total",
				Help: "Total number of SDK operations",
			},
			[]string{"operation", "status"},
		),
		sdkOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_sdk_operations_duration_seconds",
				Help:    "Duration of SDK operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		sdkErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_sdk_errors_total",
				Help: "Total number of SDK errors",
			},
			[]string{"operation", "error_type"},
		),

		// 服务发现指标
		serviceDiscoveryTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_service_discovery_total",
				Help: "Total number of service discovery operations",
			},
			[]string{"service", "namespace", "status"},
		),
		serviceDiscoveryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_service_discovery_duration_seconds",
				Help:    "Duration of service discovery operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),
		serviceInstancesTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "polaris_service_instances_total",
				Help: "Total number of service instances",
			},
			[]string{"service", "namespace", "status"},
		),

		// 服务注册指标
		serviceRegistrationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_service_registration_total",
				Help: "Total number of service registration operations",
			},
			[]string{"service", "namespace", "status"},
		),
		serviceRegistrationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_service_registration_duration_seconds",
				Help:    "Duration of service registration operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),
		serviceHeartbeatTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_service_heartbeat_total",
				Help: "Total number of service heartbeat operations",
			},
			[]string{"service", "namespace", "status"},
		),

		// 配置管理指标
		configOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_config_operations_total",
				Help: "Total number of config operations",
			},
			[]string{"operation", "file", "group", "status"},
		),
		configOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_config_operations_duration_seconds",
				Help:    "Duration of config operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "file", "group"},
		),
		configChangesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_config_changes_total",
				Help: "Total number of config changes",
			},
			[]string{"file", "group"},
		),

		// 路由指标
		routeOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_route_operations_total",
				Help: "Total number of route operations",
			},
			[]string{"service", "namespace", "status"},
		),
		routeOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_route_operations_duration_seconds",
				Help:    "Duration of route operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),

		// 限流指标
		rateLimitRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_rate_limit_requests_total",
				Help: "Total number of rate limit requests",
			},
			[]string{"service", "namespace", "status"},
		),
		rateLimitRejectedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_rate_limit_rejected_total",
				Help: "Total number of rate limit rejections",
			},
			[]string{"service", "namespace"},
		),
		rateLimitQuotaUsed: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "polaris_rate_limit_quota_used",
				Help: "Rate limit quota usage",
			},
			[]string{"service", "namespace"},
		),

		// 健康检查指标
		healthCheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_health_check_total",
				Help: "Total number of health checks",
			},
			[]string{"component", "status"},
		),
		healthCheckDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "polaris_health_check_duration_seconds",
				Help:    "Duration of health checks",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"component"},
		),
		healthCheckFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_health_check_failed_total",
				Help: "Total number of failed health checks",
			},
			[]string{"component", "error_type"},
		),

		// 连接指标
		connectionTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "polaris_connection_total",
				Help: "Total number of connections",
			},
			[]string{"type", "status"},
		),
		connectionErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "polaris_connection_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"type", "error_type"},
		),
	}
}

// RecordSDKOperation 记录 SDK 操作
func (m *Metrics) RecordSDKOperation(operation, status string) {
	m.sdkOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordSDKOperationDuration 记录 SDK 操作耗时
func (m *Metrics) RecordSDKOperationDuration(operation string, duration float64) {
	m.sdkOperationsDuration.WithLabelValues(operation).Observe(duration)
}

// RecordSDKError 记录 SDK 错误
func (m *Metrics) RecordSDKError(operation, errorType string) {
	m.sdkErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// RecordServiceDiscovery 记录服务发现操作
func (m *Metrics) RecordServiceDiscovery(service, namespace, status string) {
	m.serviceDiscoveryTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordServiceDiscoveryDuration 记录服务发现耗时
func (m *Metrics) RecordServiceDiscoveryDuration(service, namespace string, duration float64) {
	m.serviceDiscoveryDuration.WithLabelValues(service, namespace).Observe(duration)
}

// SetServiceInstances 设置服务实例数量
func (m *Metrics) SetServiceInstances(service, namespace, status string, count float64) {
	m.serviceInstancesTotal.WithLabelValues(service, namespace, status).Set(count)
}

// RecordServiceRegistration 记录服务注册操作
func (m *Metrics) RecordServiceRegistration(service, namespace, status string) {
	m.serviceRegistrationTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordServiceRegistrationDuration 记录服务注册耗时
func (m *Metrics) RecordServiceRegistrationDuration(service, namespace string, duration float64) {
	m.serviceRegistrationDuration.WithLabelValues(service, namespace).Observe(duration)
}

// RecordServiceHeartbeat 记录服务心跳
func (m *Metrics) RecordServiceHeartbeat(service, namespace, status string) {
	m.serviceHeartbeatTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordConfigOperation 记录配置操作
func (m *Metrics) RecordConfigOperation(operation, file, group, status string) {
	m.configOperationsTotal.WithLabelValues(operation, file, group, status).Inc()
}

// RecordConfigOperationDuration 记录配置操作耗时
func (m *Metrics) RecordConfigOperationDuration(operation, file, group string, duration float64) {
	m.configOperationsDuration.WithLabelValues(operation, file, group).Observe(duration)
}

// RecordConfigChange 记录配置变更
func (m *Metrics) RecordConfigChange(file, group string) {
	m.configChangesTotal.WithLabelValues(file, group).Inc()
}

// RecordRouteOperation 记录路由操作
func (m *Metrics) RecordRouteOperation(service, namespace, status string) {
	m.routeOperationsTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordRouteOperationDuration 记录路由操作耗时
func (m *Metrics) RecordRouteOperationDuration(service, namespace string, duration float64) {
	m.routeOperationsDuration.WithLabelValues(service, namespace).Observe(duration)
}

// RecordRateLimitRequest 记录限流请求
func (m *Metrics) RecordRateLimitRequest(service, namespace, status string) {
	m.rateLimitRequestsTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordRateLimitRejection 记录限流拒绝
func (m *Metrics) RecordRateLimitRejection(service, namespace string) {
	m.rateLimitRejectedTotal.WithLabelValues(service, namespace).Inc()
}

// SetRateLimitQuota 设置限流配额使用量
func (m *Metrics) SetRateLimitQuota(service, namespace string, quota float64) {
	m.rateLimitQuotaUsed.WithLabelValues(service, namespace).Set(quota)
}

// RecordHealthCheck 记录健康检查
func (m *Metrics) RecordHealthCheck(component, status string) {
	m.healthCheckTotal.WithLabelValues(component, status).Inc()
}

// RecordHealthCheckDuration 记录健康检查耗时
func (m *Metrics) RecordHealthCheckDuration(component string, duration float64) {
	m.healthCheckDuration.WithLabelValues(component).Observe(duration)
}

// RecordHealthCheckFailed 记录健康检查失败
func (m *Metrics) RecordHealthCheckFailed(component, errorType string) {
	m.healthCheckFailed.WithLabelValues(component, errorType).Inc()
}

// SetConnectionCount 设置连接数量
func (m *Metrics) SetConnectionCount(connType, status string, count float64) {
	m.connectionTotal.WithLabelValues(connType, status).Set(count)
}

// RecordConnectionError 记录连接错误
func (m *Metrics) RecordConnectionError(connType, errorType string) {
	m.connectionErrorsTotal.WithLabelValues(connType, errorType).Inc()
}
