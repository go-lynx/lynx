package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ProductionMetrics provides comprehensive production monitoring metrics
type ProductionMetrics struct {
	// Application metrics
	appStartTime prometheus.Gauge
	appUptime    prometheus.Gauge
	appVersion   *prometheus.GaugeVec

	// System metrics
	systemMemory     prometheus.Gauge
	systemGoroutines prometheus.Gauge
	systemThreads    prometheus.Gauge
	systemCPUs       prometheus.Gauge

	// Plugin metrics
	pluginCount   prometheus.Gauge
	pluginHealth  *prometheus.GaugeVec
	pluginErrors  *prometheus.CounterVec
	pluginLatency *prometheus.HistogramVec

	// Circuit breaker metrics
	circuitBreakerState     *prometheus.GaugeVec
	circuitBreakerFailures  *prometheus.CounterVec
	circuitBreakerSuccesses *prometheus.CounterVec

	// Health check metrics
	healthCheckStatus  *prometheus.GaugeVec
	healthCheckLatency *prometheus.HistogramVec
	healthCheckErrors  *prometheus.CounterVec

	// Event system metrics
	eventPublished *prometheus.CounterVec
	eventProcessed *prometheus.CounterVec
	eventDropped   *prometheus.CounterVec
	eventLatency   *prometheus.HistogramVec

	// HTTP/GRPC metrics
	requestTotal    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	errorRate       *prometheus.GaugeVec

	// Cache metrics
	cacheHits      *prometheus.CounterVec
	cacheMisses    *prometheus.CounterVec
	cacheSize      *prometheus.GaugeVec
	cacheEvictions *prometheus.CounterVec

	// Database metrics
	dbConnections   *prometheus.GaugeVec
	dbQueries       *prometheus.CounterVec
	dbQueryDuration *prometheus.HistogramVec
	dbErrors        *prometheus.CounterVec

	// Configuration metrics
	configChanges *prometheus.CounterVec
	configErrors  *prometheus.CounterVec
	configReloads *prometheus.CounterVec

	// Security metrics
	authFailures       *prometheus.CounterVec
	authSuccesses      *prometheus.CounterVec
	rateLimitHits      *prometheus.CounterVec
	securityViolations *prometheus.CounterVec

	// Resource metrics
	resourceUsage  *prometheus.GaugeVec
	resourceLimit  *prometheus.GaugeVec
	resourceErrors *prometheus.CounterVec

	// Added: Performance bottleneck detection metrics
	performanceBottlenecks *prometheus.GaugeVec
	slowOperations         *prometheus.CounterVec
	timeoutOperations      *prometheus.CounterVec
	memoryLeaks            *prometheus.GaugeVec
	goroutineLeaks         *prometheus.GaugeVec

	// Added: Business metrics
	businessTransactions *prometheus.CounterVec
	businessErrors       *prometheus.CounterVec
	businessLatency      *prometheus.HistogramVec
	userSessions         *prometheus.GaugeVec
	activeConnections    *prometheus.GaugeVec

	// Added: System health metrics
	systemHealthScore    prometheus.Gauge
	componentHealthScore *prometheus.GaugeVec
	overallHealthStatus  prometheus.Gauge

	// Internal state
	mu             sync.RWMutex
	startTime      time.Time
	lastUpdate     time.Time
	updateInterval time.Duration
	stopChan       chan struct{}
}

// NewProductionMetrics creates a new production metrics instance
func NewProductionMetrics() *ProductionMetrics {
	pm := &ProductionMetrics{
		startTime:      time.Now(),
		lastUpdate:     time.Now(),
		updateInterval: 30 * time.Second,
		stopChan:       make(chan struct{}),
	}

	pm.initializeMetrics()
	return pm
}

// initializeMetrics initializes all production metrics
func (pm *ProductionMetrics) initializeMetrics() {
	// Application metrics
	pm.appStartTime = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_app_start_time_seconds",
		Help: "Application start time as Unix timestamp",
	})

	pm.appUptime = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_app_uptime_seconds",
		Help: "Application uptime in seconds",
	})

	pm.appVersion = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_app_version_info",
		Help: "Application version information",
	}, []string{"version", "commit", "build_date"})

	// System metrics
	pm.systemMemory = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_system_memory_bytes",
		Help: "Current system memory usage in bytes",
	})

	pm.systemGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_system_goroutines",
		Help: "Current number of goroutines",
	})

	pm.systemThreads = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_system_threads",
		Help: "Current number of OS threads",
	})

	pm.systemCPUs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_system_cpus",
		Help: "Number of available CPUs",
	})

	// Plugin metrics
	pm.pluginCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_plugin_count",
		Help: "Total number of loaded plugins",
	})

	pm.pluginHealth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_plugin_health_status",
		Help: "Plugin health status (1=healthy, 0=unhealthy)",
	}, []string{"plugin_name", "plugin_id"})

	pm.pluginErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_plugin_errors_total",
		Help: "Total number of plugin errors",
	}, []string{"plugin_name", "plugin_id", "error_type"})

	pm.pluginLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_plugin_latency_seconds",
		Help:    "Plugin operation latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"plugin_name", "plugin_id", "operation"})

	// Circuit breaker metrics
	pm.circuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_circuit_breaker_state",
		Help: "Circuit breaker state (0=closed, 1=open, 2=half_open)",
	}, []string{"breaker_name"})

	pm.circuitBreakerFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_circuit_breaker_failures_total",
		Help: "Total number of circuit breaker failures",
	}, []string{"breaker_name"})

	pm.circuitBreakerSuccesses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_circuit_breaker_successes_total",
		Help: "Total number of circuit breaker successes",
	}, []string{"breaker_name"})

	// Health check metrics
	pm.healthCheckStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_health_check_status",
		Help: "Health check status (1=healthy, 0=unhealthy)",
	}, []string{"check_name"})

	pm.healthCheckLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_health_check_latency_seconds",
		Help:    "Health check latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"check_name"})

	pm.healthCheckErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_health_check_errors_total",
		Help: "Total number of health check errors",
	}, []string{"check_name", "error_type"})

	// Event system metrics
	pm.eventPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_events_published_total",
		Help: "Total number of events published",
	}, []string{"event_type", "bus_type"})

	pm.eventProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_events_processed_total",
		Help: "Total number of events processed",
	}, []string{"event_type", "bus_type"})

	pm.eventDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_events_dropped_total",
		Help: "Total number of events dropped",
	}, []string{"event_type", "bus_type", "reason"})

	pm.eventLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_event_latency_seconds",
		Help:    "Event processing latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type", "bus_type"})

	// HTTP/GRPC metrics
	pm.requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_requests_total",
		Help: "Total number of requests",
	}, []string{"method", "path", "status_code", "protocol"})

	pm.requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "protocol"})

	pm.requestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_request_size_bytes",
		Help:    "Request size in bytes",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	}, []string{"method", "path", "protocol"})

	pm.responseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_response_size_bytes",
		Help:    "Response size in bytes",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	}, []string{"method", "path", "protocol"})

	pm.errorRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_error_rate",
		Help: "Error rate percentage",
	}, []string{"service", "endpoint"})

	// Cache metrics
	pm.cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_cache_hits_total",
		Help: "Total number of cache hits",
	}, []string{"cache_name"})

	pm.cacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_cache_misses_total",
		Help: "Total number of cache misses",
	}, []string{"cache_name"})

	pm.cacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_cache_size",
		Help: "Current cache size",
	}, []string{"cache_name"})

	pm.cacheEvictions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_cache_evictions_total",
		Help: "Total number of cache evictions",
	}, []string{"cache_name"})

	// Database metrics
	pm.dbConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_db_connections",
		Help: "Current number of database connections",
	}, []string{"database", "type"})

	pm.dbQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_db_queries_total",
		Help: "Total number of database queries",
	}, []string{"database", "type", "operation"})

	pm.dbQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_db_query_duration_seconds",
		Help:    "Database query duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"database", "type", "operation"})

	pm.dbErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_db_errors_total",
		Help: "Total number of database errors",
	}, []string{"database", "type", "error_type"})

	// Configuration metrics
	pm.configChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_config_changes_total",
		Help: "Total number of configuration changes",
	}, []string{"config_type"})

	pm.configErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_config_errors_total",
		Help: "Total number of configuration errors",
	}, []string{"config_type", "error_type"})

	pm.configReloads = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_config_reloads_total",
		Help: "Total number of configuration reloads",
	}, []string{"config_type"})

	// Security metrics
	pm.authFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_auth_failures_total",
		Help: "Total number of authentication failures",
	}, []string{"auth_type", "reason"})

	pm.authSuccesses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_auth_successes_total",
		Help: "Total number of authentication successes",
	}, []string{"auth_type"})

	pm.rateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_rate_limit_hits_total",
		Help: "Total number of rate limit hits",
	}, []string{"limit_type", "resource"})

	pm.securityViolations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_security_violations_total",
		Help: "Total number of security violations",
	}, []string{"violation_type", "severity"})

	// Resource metrics
	pm.resourceUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_resource_usage",
		Help: "Current resource usage",
	}, []string{"resource_type", "unit"})

	pm.resourceLimit = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_resource_limit",
		Help: "Resource limit",
	}, []string{"resource_type", "unit"})

	pm.resourceErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_resource_errors_total",
		Help: "Total number of resource errors",
	}, []string{"resource_type", "error_type"})

	// Added: Performance bottleneck detection metrics
	pm.performanceBottlenecks = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_performance_bottlenecks",
		Help: "Performance bottlenecks detected (1=detected, 0=normal)",
	}, []string{"bottleneck_type", "component", "severity"})

	pm.slowOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_slow_operations_total",
		Help: "Total number of slow operations detected",
	}, []string{"operation_type", "component", "threshold"})

	pm.timeoutOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_timeout_operations_total",
		Help: "Total number of timeout operations",
	}, []string{"operation_type", "component", "timeout_duration"})

	pm.memoryLeaks = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_memory_leaks_detected",
		Help: "Memory leaks detected (1=detected, 0=normal)",
	}, []string{"component", "leak_type"})

	pm.goroutineLeaks = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_goroutine_leaks_detected",
		Help: "Goroutine leaks detected (1=detected, 0=normal)",
	}, []string{"component", "leak_type"})

	// Added: Business metrics
	pm.businessTransactions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_business_transactions_total",
		Help: "Total number of business transactions",
	}, []string{"transaction_type", "status", "user_type"})

	pm.businessErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_business_errors_total",
		Help: "Total number of business errors",
	}, []string{"error_type", "business_function", "severity"})

	pm.businessLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_business_latency_seconds",
		Help:    "Business operation latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"business_function", "operation_type"})

	pm.userSessions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_user_sessions_active",
		Help: "Number of active user sessions",
	}, []string{"session_type", "user_category"})

	pm.activeConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_active_connections",
		Help: "Number of active connections",
	}, []string{"connection_type", "protocol"})

	// Added: System health metrics
	pm.systemHealthScore = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_system_health_score",
		Help: "Overall system health score (0-100)",
	})

	pm.componentHealthScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lynx_component_health_score",
		Help: "Component health score (0-100)",
	}, []string{"component", "health_type"})

	pm.overallHealthStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lynx_overall_health_status",
		Help: "Overall health status (1=healthy, 0=unhealthy, 0.5=degraded)",
	})
}

// Start starts the metrics collection
func (pm *ProductionMetrics) Start() {
	go pm.collectMetrics()
}

// Stop stops the metrics collection
func (pm *ProductionMetrics) Stop() {
	close(pm.stopChan)
}

// collectMetrics periodically collects and updates metrics
func (pm *ProductionMetrics) collectMetrics() {
	ticker := time.NewTicker(pm.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.updateMetrics()
		case <-pm.stopChan:
			return
		}
	}
}

// updateMetrics updates all metrics with current values
func (pm *ProductionMetrics) updateMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()

	// Update application metrics
	pm.appStartTime.Set(float64(pm.startTime.Unix()))
	pm.appUptime.Set(now.Sub(pm.startTime).Seconds())

	// Update system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pm.systemMemory.Set(float64(m.Alloc))
	pm.systemGoroutines.Set(float64(runtime.NumGoroutine()))
	pm.systemThreads.Set(float64(runtime.GOMAXPROCS(0)))
	pm.systemCPUs.Set(float64(runtime.NumCPU()))

	pm.lastUpdate = now
}

// RecordPluginHealth records plugin health status
func (pm *ProductionMetrics) RecordPluginHealth(pluginName, pluginID string, healthy bool) {
	status := 0.0
	if healthy {
		status = 1.0
	}
	pm.pluginHealth.WithLabelValues(pluginName, pluginID).Set(status)
}

// RecordPluginError records a plugin error
func (pm *ProductionMetrics) RecordPluginError(pluginName, pluginID, errorType string) {
	pm.pluginErrors.WithLabelValues(pluginName, pluginID, errorType).Inc()
}

// RecordPluginLatency records plugin operation latency
func (pm *ProductionMetrics) RecordPluginLatency(pluginName, pluginID, operation string, duration time.Duration) {
	pm.pluginLatency.WithLabelValues(pluginName, pluginID, operation).Observe(duration.Seconds())
}

// RecordCircuitBreakerState records circuit breaker state
func (pm *ProductionMetrics) RecordCircuitBreakerState(breakerName string, state int) {
	pm.circuitBreakerState.WithLabelValues(breakerName).Set(float64(state))
}

// RecordCircuitBreakerFailure records a circuit breaker failure
func (pm *ProductionMetrics) RecordCircuitBreakerFailure(breakerName string) {
	pm.circuitBreakerFailures.WithLabelValues(breakerName).Inc()
}

// RecordCircuitBreakerSuccess records a circuit breaker success
func (pm *ProductionMetrics) RecordCircuitBreakerSuccess(breakerName string) {
	pm.circuitBreakerSuccesses.WithLabelValues(breakerName).Inc()
}

// RecordHealthCheck records health check status
func (pm *ProductionMetrics) RecordHealthCheck(checkName string, healthy bool, duration time.Duration) {
	status := 0.0
	if healthy {
		status = 1.0
	}
	pm.healthCheckStatus.WithLabelValues(checkName).Set(status)
	pm.healthCheckLatency.WithLabelValues(checkName).Observe(duration.Seconds())
}

// RecordHealthCheckError records a health check error
func (pm *ProductionMetrics) RecordHealthCheckError(checkName, errorType string) {
	pm.healthCheckErrors.WithLabelValues(checkName, errorType).Inc()
}

// RecordEventPublished records an event publication
func (pm *ProductionMetrics) RecordEventPublished(eventType, busType string) {
	pm.eventPublished.WithLabelValues(eventType, busType).Inc()
}

// RecordEventProcessed records an event processing
func (pm *ProductionMetrics) RecordEventProcessed(eventType, busType string, duration time.Duration) {
	pm.eventProcessed.WithLabelValues(eventType, busType).Inc()
	pm.eventLatency.WithLabelValues(eventType, busType).Observe(duration.Seconds())
}

// RecordEventDropped records an event drop
func (pm *ProductionMetrics) RecordEventDropped(eventType, busType, reason string) {
	pm.eventDropped.WithLabelValues(eventType, busType, reason).Inc()
}

// RecordRequest records an HTTP/GRPC request
func (pm *ProductionMetrics) RecordRequest(method, path, statusCode, protocol string, duration time.Duration, requestSize, responseSize int64) {
	pm.requestTotal.WithLabelValues(method, path, statusCode, protocol).Inc()
	pm.requestDuration.WithLabelValues(method, path, protocol).Observe(duration.Seconds())
	pm.requestSize.WithLabelValues(method, path, protocol).Observe(float64(requestSize))
	pm.responseSize.WithLabelValues(method, path, protocol).Observe(float64(responseSize))
}

// RecordErrorRate records error rate
func (pm *ProductionMetrics) RecordErrorRate(service, endpoint string, rate float64) {
	pm.errorRate.WithLabelValues(service, endpoint).Set(rate)
}

// RecordCacheHit records a cache hit
func (pm *ProductionMetrics) RecordCacheHit(cacheName string) {
	pm.cacheHits.WithLabelValues(cacheName).Inc()
}

// RecordCacheMiss records a cache miss
func (pm *ProductionMetrics) RecordCacheMiss(cacheName string) {
	pm.cacheMisses.WithLabelValues(cacheName).Inc()
}

// RecordCacheSize records cache size
func (pm *ProductionMetrics) RecordCacheSize(cacheName string, size int64) {
	pm.cacheSize.WithLabelValues(cacheName).Set(float64(size))
}

// RecordCacheEviction records a cache eviction
func (pm *ProductionMetrics) RecordCacheEviction(cacheName string) {
	pm.cacheEvictions.WithLabelValues(cacheName).Inc()
}

// RecordDBConnection records database connection count
func (pm *ProductionMetrics) RecordDBConnection(database, connType string, count int64) {
	pm.dbConnections.WithLabelValues(database, connType).Set(float64(count))
}

// RecordDBQuery records a database query
func (pm *ProductionMetrics) RecordDBQuery(database, queryType, operation string, duration time.Duration) {
	pm.dbQueries.WithLabelValues(database, queryType, operation).Inc()
	pm.dbQueryDuration.WithLabelValues(database, queryType, operation).Observe(duration.Seconds())
}

// RecordDBError records a database error
func (pm *ProductionMetrics) RecordDBError(database, queryType, errorType string) {
	pm.dbErrors.WithLabelValues(database, queryType, errorType).Inc()
}

// RecordConfigChange records a configuration change
func (pm *ProductionMetrics) RecordConfigChange(configType string) {
	pm.configChanges.WithLabelValues(configType).Inc()
}

// RecordConfigError records a configuration error
func (pm *ProductionMetrics) RecordConfigError(configType, errorType string) {
	pm.configErrors.WithLabelValues(configType, errorType).Inc()
}

// RecordConfigReload records a configuration reload
func (pm *ProductionMetrics) RecordConfigReload(configType string) {
	pm.configReloads.WithLabelValues(configType).Inc()
}

// RecordAuthFailure records an authentication failure
func (pm *ProductionMetrics) RecordAuthFailure(authType, reason string) {
	pm.authFailures.WithLabelValues(authType, reason).Inc()
}

// RecordAuthSuccess records an authentication success
func (pm *ProductionMetrics) RecordAuthSuccess(authType string) {
	pm.authSuccesses.WithLabelValues(authType).Inc()
}

// RecordRateLimitHit records a rate limit hit
func (pm *ProductionMetrics) RecordRateLimitHit(limitType, resource string) {
	pm.rateLimitHits.WithLabelValues(limitType, resource).Inc()
}

// RecordSecurityViolation records a security violation
func (pm *ProductionMetrics) RecordSecurityViolation(violationType, severity string) {
	pm.securityViolations.WithLabelValues(violationType, severity).Inc()
}

// RecordResourceUsage records resource usage
func (pm *ProductionMetrics) RecordResourceUsage(resourceType, unit string, usage float64) {
	pm.resourceUsage.WithLabelValues(resourceType, unit).Set(usage)
}

// RecordResourceLimit records resource limit
func (pm *ProductionMetrics) RecordResourceLimit(resourceType, unit string, limit float64) {
	pm.resourceLimit.WithLabelValues(resourceType, unit).Set(limit)
}

// RecordResourceError records a resource error
func (pm *ProductionMetrics) RecordResourceError(resourceType, errorType string) {
	pm.resourceErrors.WithLabelValues(resourceType, errorType).Inc()
}

// RecordPerformanceBottleneck records a performance bottleneck
func (pm *ProductionMetrics) RecordPerformanceBottleneck(bottleneckType, component, severity string) {
	if pm.performanceBottlenecks != nil {
		pm.performanceBottlenecks.WithLabelValues(bottleneckType, component, severity).Set(1)
	}
}

// RecordSlowOperation records a slow operation
func (pm *ProductionMetrics) RecordSlowOperation(operationType, component, threshold string) {
	if pm.slowOperations != nil {
		pm.slowOperations.WithLabelValues(operationType, component, threshold).Inc()
	}
}

// RecordTimeoutOperation records a timeout operation
func (pm *ProductionMetrics) RecordTimeoutOperation(operationType, component, timeoutDuration string) {
	if pm.timeoutOperations != nil {
		pm.timeoutOperations.WithLabelValues(operationType, component, timeoutDuration).Inc()
	}
}

// RecordMemoryLeak records a memory leak detection
func (pm *ProductionMetrics) RecordMemoryLeak(component, leakType string) {
	if pm.memoryLeaks != nil {
		pm.memoryLeaks.WithLabelValues(component, leakType).Set(1)
	}
}

// RecordGoroutineLeak records a goroutine leak detection
func (pm *ProductionMetrics) RecordGoroutineLeak(component, leakType string) {
	if pm.goroutineLeaks != nil {
		pm.goroutineLeaks.WithLabelValues(component, leakType).Set(1)
	}
}

// RecordBusinessTransaction records a business transaction
func (pm *ProductionMetrics) RecordBusinessTransaction(transactionType, status, userType string) {
	if pm.businessTransactions != nil {
		pm.businessTransactions.WithLabelValues(transactionType, status, userType).Inc()
	}
}

// RecordBusinessError records a business error
func (pm *ProductionMetrics) RecordBusinessError(errorType, businessFunction, severity string) {
	if pm.businessErrors != nil {
		pm.businessErrors.WithLabelValues(errorType, businessFunction, severity).Inc()
	}
}

// RecordBusinessLatency records business operation latency
func (pm *ProductionMetrics) RecordBusinessLatency(businessFunction, operationType string, duration time.Duration) {
	if pm.businessLatency != nil {
		pm.businessLatency.WithLabelValues(businessFunction, operationType).Observe(duration.Seconds())
	}
}

// UpdateSystemHealthScore updates the overall system health score
func (pm *ProductionMetrics) UpdateSystemHealthScore(score float64) {
	if pm.systemHealthScore != nil {
		pm.systemHealthScore.Set(score)
	}
}

// UpdateComponentHealthScore updates a component's health score
func (pm *ProductionMetrics) UpdateComponentHealthScore(component, healthType string, score float64) {
	if pm.componentHealthScore != nil {
		pm.componentHealthScore.WithLabelValues(component, healthType).Set(score)
	}
}

// UpdateOverallHealthStatus updates the overall health status
func (pm *ProductionMetrics) UpdateOverallHealthStatus(status float64) {
	if pm.overallHealthStatus != nil {
		pm.overallHealthStatus.Set(status)
	}
}

// GetMetrics returns current metrics snapshot
func (pm *ProductionMetrics) GetMetrics() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"app_start_time":      pm.startTime.Unix(),
		"app_uptime_seconds":  time.Since(pm.startTime).Seconds(),
		"system_memory_bytes": m.Alloc,
		"system_goroutines":   runtime.NumGoroutine(),
		"system_threads":      runtime.GOMAXPROCS(0),
		"system_cpus":         runtime.NumCPU(),
		"last_update":         pm.lastUpdate.Unix(),
	}
}
