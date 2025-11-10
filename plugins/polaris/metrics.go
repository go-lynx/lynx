package polaris

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics defines Polaris-related monitoring metrics
type Metrics struct {
	// SDK operation metrics
	sdkOperationsTotal    *prometheus.CounterVec
	sdkOperationsDuration *prometheus.HistogramVec
	sdkErrorsTotal        *prometheus.CounterVec

	// Service discovery metrics
	serviceDiscoveryTotal    *prometheus.CounterVec
	serviceDiscoveryDuration *prometheus.HistogramVec
	serviceInstancesTotal    *prometheus.GaugeVec

	// Service registration metrics
	serviceRegistrationTotal    *prometheus.CounterVec
	serviceRegistrationDuration *prometheus.HistogramVec
	serviceHeartbeatTotal       *prometheus.CounterVec

	// Configuration management metrics
	configOperationsTotal    *prometheus.CounterVec
	configOperationsDuration *prometheus.HistogramVec
	configChangesTotal       *prometheus.CounterVec

	// Routing metrics
	routeOperationsTotal    *prometheus.CounterVec
	routeOperationsDuration *prometheus.HistogramVec

	// Rate limiting metrics
	rateLimitRequestsTotal *prometheus.CounterVec
	rateLimitRejectedTotal *prometheus.CounterVec
	rateLimitQuotaUsed     *prometheus.GaugeVec

	// Health check metrics
	healthCheckTotal    *prometheus.CounterVec
	healthCheckDuration *prometheus.HistogramVec
	healthCheckFailed   *prometheus.CounterVec

	// Connection metrics
	connectionTotal       *prometheus.GaugeVec
	connectionErrorsTotal *prometheus.CounterVec
}

// NewPolarisMetrics creates new monitoring metrics instance
func NewPolarisMetrics() *Metrics {
	return &Metrics{
		// SDK operation metrics
		sdkOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "sdk_operations_total",
				Help: "Total number of SDK operations",
			},
			[]string{"operation", "status"},
		),
		sdkOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "sdk_operations_duration_seconds",
				Help:    "Duration of SDK operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		sdkErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "sdk_errors_total",
				Help: "Total number of SDK errors",
			},
			[]string{"operation", "error_type"},
		),

		// Service discovery metrics
		serviceDiscoveryTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "service_discovery_total",
				Help: "Total number of service discovery operations",
			},
			[]string{"service", "namespace", "status"},
		),
		serviceDiscoveryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "service_discovery_duration_seconds",
				Help:    "Duration of service discovery operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),
		serviceInstancesTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "service_instances_total",
				Help: "Total number of service instances",
			},
			[]string{"service", "namespace", "status"},
		),

		// Service registration metrics
		serviceRegistrationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "service_registration_total",
				Help: "Total number of service registration operations",
			},
			[]string{"service", "namespace", "status"},
		),
		serviceRegistrationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "service_registration_duration_seconds",
				Help:    "Duration of service registration operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),
		serviceHeartbeatTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "service_heartbeat_total",
				Help: "Total number of service heartbeat operations",
			},
			[]string{"service", "namespace", "status"},
		),

		// Configuration management metrics
		configOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "config_operations_total",
				Help: "Total number of config operations",
			},
			[]string{"operation", "file", "group", "status"},
		),
		configOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "config_operations_duration_seconds",
				Help:    "Duration of config operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "file", "group"},
		),
		configChangesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "config_changes_total",
				Help: "Total number of config changes",
			},
			[]string{"file", "group"},
		),

		// Routing metrics
		routeOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "route_operations_total",
				Help: "Total number of route operations",
			},
			[]string{"service", "namespace", "status"},
		),
		routeOperationsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "route_operations_duration_seconds",
				Help:    "Duration of route operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "namespace"},
		),

		// Rate limiting metrics
		rateLimitRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "rate_limit_requests_total",
				Help: "Total number of rate limit requests",
			},
			[]string{"service", "namespace", "status"},
		),
		rateLimitRejectedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "rate_limit_rejected_total",
				Help: "Total number of rate limit rejections",
			},
			[]string{"service", "namespace"},
		),
		rateLimitQuotaUsed: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "rate_limit_quota_used",
				Help: "Rate limit quota usage",
			},
			[]string{"service", "namespace"},
		),

		// Health check metrics
		healthCheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "health_check_total",
				Help: "Total number of health checks",
			},
			[]string{"component", "status"},
		),
		healthCheckDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name:    "health_check_duration_seconds",
				Help:    "Duration of health checks",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"component"},
		),
		healthCheckFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "health_check_failed_total",
				Help: "Total number of failed health checks",
			},
			[]string{"component", "error_type"},
		),

		// Connection metrics
		connectionTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "connection_total",
				Help: "Total number of connections",
			},
			[]string{"type", "status"},
		),
		connectionErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "polaris",
				Name: "connection_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"type", "error_type"},
		),
	}
}

// RecordSDKOperation records SDK operation
func (m *Metrics) RecordSDKOperation(operation, status string) {
	m.sdkOperationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordSDKOperationDuration records SDK operation duration
func (m *Metrics) RecordSDKOperationDuration(operation string, duration float64) {
	m.sdkOperationsDuration.WithLabelValues(operation).Observe(duration)
}

// RecordSDKError records SDK error
func (m *Metrics) RecordSDKError(operation, errorType string) {
	m.sdkErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// RecordServiceDiscovery records service discovery operation
func (m *Metrics) RecordServiceDiscovery(service, namespace, status string) {
	m.serviceDiscoveryTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordServiceDiscoveryDuration records service discovery duration
func (m *Metrics) RecordServiceDiscoveryDuration(service, namespace string, duration float64) {
	m.serviceDiscoveryDuration.WithLabelValues(service, namespace).Observe(duration)
}

// SetServiceInstances sets service instance count
func (m *Metrics) SetServiceInstances(service, namespace, status string, count float64) {
	m.serviceInstancesTotal.WithLabelValues(service, namespace, status).Set(count)
}

// RecordServiceRegistration records service registration operation
func (m *Metrics) RecordServiceRegistration(service, namespace, status string) {
	m.serviceRegistrationTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordServiceRegistrationDuration records service registration duration
func (m *Metrics) RecordServiceRegistrationDuration(service, namespace string, duration float64) {
	m.serviceRegistrationDuration.WithLabelValues(service, namespace).Observe(duration)
}

// RecordServiceHeartbeat records service heartbeat
func (m *Metrics) RecordServiceHeartbeat(service, namespace, status string) {
	m.serviceHeartbeatTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordConfigOperation records configuration operation
func (m *Metrics) RecordConfigOperation(operation, file, group, status string) {
	m.configOperationsTotal.WithLabelValues(operation, file, group, status).Inc()
}

// RecordConfigOperationDuration records configuration operation duration
func (m *Metrics) RecordConfigOperationDuration(operation, file, group string, duration float64) {
	m.configOperationsDuration.WithLabelValues(operation, file, group).Observe(duration)
}

// RecordConfigChange records configuration change
func (m *Metrics) RecordConfigChange(file, group string) {
	m.configChangesTotal.WithLabelValues(file, group).Inc()
}

// RecordRouteOperation records route operation
func (m *Metrics) RecordRouteOperation(service, namespace, status string) {
	m.routeOperationsTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordRouteOperationDuration records route operation duration
func (m *Metrics) RecordRouteOperationDuration(service, namespace string, duration float64) {
	m.routeOperationsDuration.WithLabelValues(service, namespace).Observe(duration)
}

// RecordRateLimitRequest records rate limit request
func (m *Metrics) RecordRateLimitRequest(service, namespace, status string) {
	m.rateLimitRequestsTotal.WithLabelValues(service, namespace, status).Inc()
}

// RecordRateLimitRejection records rate limit rejection
func (m *Metrics) RecordRateLimitRejection(service, namespace string) {
	m.rateLimitRejectedTotal.WithLabelValues(service, namespace).Inc()
}

// SetRateLimitQuota sets rate limit quota usage
func (m *Metrics) SetRateLimitQuota(service, namespace string, quota float64) {
	m.rateLimitQuotaUsed.WithLabelValues(service, namespace).Set(quota)
}

// RecordHealthCheck records health check
func (m *Metrics) RecordHealthCheck(component, status string) {
	m.healthCheckTotal.WithLabelValues(component, status).Inc()
}

// RecordHealthCheckDuration records health check duration
func (m *Metrics) RecordHealthCheckDuration(component string, duration float64) {
	m.healthCheckDuration.WithLabelValues(component).Observe(duration)
}

// RecordHealthCheckFailed records health check failure
func (m *Metrics) RecordHealthCheckFailed(component, errorType string) {
	m.healthCheckFailed.WithLabelValues(component, errorType).Inc()
}

// SetConnectionCount sets connection count
func (m *Metrics) SetConnectionCount(connType, status string, count float64) {
	m.connectionTotal.WithLabelValues(connType, status).Set(count)
}

// RecordConnectionError records connection error
func (m *Metrics) RecordConnectionError(connType, errorType string) {
	m.connectionErrorsTotal.WithLabelValues(connType, errorType).Inc()
}
