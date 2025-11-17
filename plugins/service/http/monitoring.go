// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"encoding/json"
	"fmt"
	"net"
	nhttp "net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/observability/metrics"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/prometheus/client_golang/prometheus"
)

// initMonitoringDefaults initializes monitoring defaults.
func (h *ServiceHttp) initMonitoringDefaults() {
	// Set default monitoring configuration if not provided
	if h.conf.Monitoring == nil {
		h.conf.Monitoring = &conf.MonitoringConfig{}
	}

	// Enable metrics by default
	if !h.conf.Monitoring.EnableMetrics {
		h.conf.Monitoring.EnableMetrics = true
	}

	// Enable connection metrics by default
	if !h.conf.Monitoring.EnableConnectionMetrics {
		h.conf.Monitoring.EnableConnectionMetrics = true
	}

	// Enable queue metrics by default
	if !h.conf.Monitoring.EnableQueueMetrics {
		h.conf.Monitoring.EnableQueueMetrics = true
	}

	// Enable route metrics by default
	if !h.conf.Monitoring.EnableRouteMetrics {
		h.conf.Monitoring.EnableRouteMetrics = true
	}
}

// initMetrics initializes Prometheus metrics.
// Use global singletons to avoid duplicate registrations across instances.
var (
	metricsInitOnce          sync.Once
	httpRequestCounter       *prometheus.CounterVec
	httpRequestDuration      *prometheus.HistogramVec
	httpResponseSize         *prometheus.HistogramVec
	httpRequestSize          *prometheus.HistogramVec
	httpErrorCounter         *prometheus.CounterVec
	httpHealthCheckTot       *prometheus.CounterVec
	httpInflight             *prometheus.GaugeVec
	httpActiveConnections    *prometheus.GaugeVec
	httpConnectionPoolUsage  *prometheus.GaugeVec
	httpRequestQueueLength   *prometheus.GaugeVec
	httpRouteRequestCounter  *prometheus.CounterVec
	httpRouteRequestDuration *prometheus.HistogramVec
)

// ensureGlobalMetrics initializes metrics and registers them once in the unified registry.
func ensureGlobalMetrics() {
	metricsInitOnce.Do(func() {
		httpRequestCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		)

		httpRequestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"method", "path"},
		)

		httpResponseSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		)

		httpRequestSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "request_size_bytes",
				Help:      "HTTP request size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		)

		httpErrorCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "errors_total",
				Help:      "Total number of HTTP errors",
			},
			[]string{"method", "path", "error_type"},
		)

		httpHealthCheckTot = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "health_check_total",
				Help:      "Total number of health checks",
			},
			[]string{"status"},
		)

		httpInflight = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "inflight_requests",
				Help:      "Number of HTTP requests currently being served",
			},
			[]string{"path"},
		)

		// Additional metrics
		httpActiveConnections = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "active_connections",
				Help:      "Number of active HTTP connections",
			},
			[]string{"address"},
		)

		httpConnectionPoolUsage = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "connection_pool_usage",
				Help:      "Connection pool usage percentage",
			},
			[]string{"pool_name"},
		)

		httpRequestQueueLength = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "request_queue_length",
				Help:      "Number of requests in the queue",
			},
			[]string{"path"},
		)

		httpRouteRequestCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "route_requests_total",
				Help:      "Total number of requests per route",
			},
			[]string{"route", "method", "status"},
		)

		httpRouteRequestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lynx",
				Subsystem: "http",
				Name:      "route_request_duration_seconds",
				Help:      "HTTP request duration per route in seconds",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"route", "method"},
		)

		// Register with the unified registry to avoid duplicate registrations across instances.
		metrics.MustRegister(
			httpRequestCounter,
			httpRequestDuration,
			httpResponseSize,
			httpRequestSize,
			httpErrorCounter,
			httpHealthCheckTot,
			httpInflight,
			httpActiveConnections,
			httpConnectionPoolUsage,
			httpRequestQueueLength,
			httpRouteRequestCounter,
			httpRouteRequestDuration,
		)
	})
}

func (h *ServiceHttp) initMetrics() {
	// Ensure global metrics are initialized and registered once.
	ensureGlobalMetrics()

	// Reuse the same set of collectors for each instance.
	h.requestCounter = httpRequestCounter
	h.requestDuration = httpRequestDuration
	h.responseSize = httpResponseSize
	h.requestSize = httpRequestSize
	h.errorCounter = httpErrorCounter
	h.healthCheckTotal = httpHealthCheckTot
	h.inflightRequests = httpInflight
	h.activeConnections = httpActiveConnections
	h.connectionPoolUsage = httpConnectionPoolUsage
	h.requestQueueLength = httpRequestQueueLength
	h.routeRequestCounter = httpRouteRequestCounter
	h.routeRequestDuration = httpRouteRequestDuration

	// Initialize real connection pool metrics if enabled
	if h.conf.Monitoring != nil && h.conf.Monitoring.EnableConnectionMetrics {
		// Create context for metrics goroutine
		h.metricsCtx, h.metricsCancel = context.WithCancel(context.Background())
		// Initialize connection pool metrics with real values
		go h.updateConnectionPoolMetrics(h.metricsCtx)
	}
}

// CheckHealth performs a comprehensive health check for the HTTP server.
func (h *ServiceHttp) CheckHealth() error {
	if h.server == nil {
		return fmt.Errorf("HTTP server is not initialized")
	}

	// During startup, only check if server is configured properly
	// Skip port connection check as server may not be listening yet
	if h.conf != nil && h.conf.Addr != "" {
		log.Debugf("HTTP server configured for address: %s", h.conf.Addr)
	} else {
		return fmt.Errorf("HTTP server address not configured")
	}

	// Record health check metrics
	if h.healthCheckTotal != nil {
		h.healthCheckTotal.WithLabelValues("success").Inc()
	}

	log.Debugf("HTTP service health check passed")
	return nil
}

// CheckRuntimeHealth performs a comprehensive runtime health check including port connectivity.
// This is used by the health check endpoint when the service is running.
// It uses a cache to avoid frequent network dials when checks are successful,
// but always performs a real check on failures to ensure immediate issue detection.
func (h *ServiceHttp) CheckRuntimeHealth() error {
	if h.server == nil {
		return fmt.Errorf("HTTP server is not initialized")
	}

	// Check if the listen address is accepting connections.
	if h.conf.Addr != "" {
		// Use cached check result to avoid frequent network dials
		// Only cache failures briefly, never cache successes
		if err := h.checkPortAvailability(); err != nil {
			return fmt.Errorf("HTTP server is not listening on %s: %w", h.conf.Addr, err)
		}
	}

	// Record health check metrics
	if h.healthCheckTotal != nil {
		h.healthCheckTotal.WithLabelValues("success").Inc()
	}

	log.Debugf("HTTP service runtime health check passed")
	return nil
}

// updateConnectionPoolMetrics updates connection pool metrics with real values
// Runs in a background goroutine until context is cancelled
func (h *ServiceHttp) updateConnectionPoolMetrics(ctx context.Context) {
	poolName := "http-server-pool"
	if h.connectionPoolUsage == nil {
		return
	}

	// Update metrics periodically until context is cancelled
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial update
	h.updateConnectionPoolMetricsOnce(poolName)

	for {
		select {
		case <-ctx.Done():
			log.Debugf("Connection pool metrics goroutine stopped")
			return
		case <-ticker.C:
			h.updateConnectionPoolMetricsOnce(poolName)
		}
	}
}

// updateConnectionPoolMetricsOnce performs a single metrics update
func (h *ServiceHttp) updateConnectionPoolMetricsOnce(poolName string) {
	if h.connectionPoolUsage == nil {
		return
	}

	// Calculate real connection pool usage based on configuration
	// Note: Prometheus Set() operations don't return errors, so we don't need error handling here
	if h.maxConnections > 0 {
		current := atomic.LoadInt32(&h.activeConnectionsCount)
		usage := clampUsage(float64(current) / float64(h.maxConnections))
		h.connectionPoolUsage.WithLabelValues(poolName).Set(usage)
		log.Debugf("Connection pool metrics updated for pool: %s (current=%d, max=%d)", poolName, current, h.maxConnections)
	} else {
		// No connection limit configured, set to 0
		h.connectionPoolUsage.WithLabelValues(poolName).Set(0)
	}
}

// healthCheckHandler returns the health check handler.
func (h *ServiceHttp) healthCheckHandler() nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Execute runtime health check (includes port connectivity)
		err := h.CheckRuntimeHealth()

		var response map[string]interface{}
		var statusCode int

		if err != nil {
			statusCode = nhttp.StatusServiceUnavailable
			response = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
				"time":   time.Now().Format(time.RFC3339),
			}

			if h.healthCheckTotal != nil {
				h.healthCheckTotal.WithLabelValues("failure").Inc()
			}
		} else {
			statusCode = nhttp.StatusOK
			response = map[string]interface{}{
				"status": "healthy",
				"time":   time.Now().Format(time.RFC3339),
			}

			if h.healthCheckTotal != nil {
				h.healthCheckTotal.WithLabelValues("success").Inc()
			}
		}

		// Serialize and write the response
		w.WriteHeader(statusCode)
		if data, err := json.Marshal(response); err == nil {
			_, writeErr := w.Write(data)
			if writeErr != nil {
				return
			}
		} else {
			log.Errorf("Failed to marshal health check response: %v", err)
			w.WriteHeader(nhttp.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"error": "Failed to serialize response"}`))
			if writeErr != nil {
				return
			}
		}
	})
}

// UpdateConnectionPoolUsage updates the connection pool usage metric
func (h *ServiceHttp) UpdateConnectionPoolUsage(activeConnections, maxConnections int32) {
	if h.connectionPoolUsage != nil && maxConnections > 0 {
		usage := clampUsage(float64(activeConnections) / float64(maxConnections))
		h.connectionPoolUsage.WithLabelValues("http-server-pool").Set(usage)
	}
}

// GetConnectionPoolUsage returns the current connection pool usage
func (h *ServiceHttp) GetConnectionPoolUsage() float64 {
	if h.maxConnections > 0 {
		current := atomic.LoadInt32(&h.activeConnectionsCount)
		return clampUsage(float64(current) / float64(h.maxConnections))
	}
	return 0.0
}

func clampUsage(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// checkPortAvailability checks if the configured port is available for binding.
// It always performs a real network check to ensure real-time failure detection.
// Failed checks are cached briefly to avoid hammering unreachable ports.
func (h *ServiceHttp) checkPortAvailability() error {
	if h.conf == nil || h.conf.Addr == "" {
		return fmt.Errorf("server address not configured")
	}

	// Parse address and normalize host for dial
	addr := h.conf.Addr
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %v", addr, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	norm := net.JoinHostPort(host, port)

	// Only check TCP network here
	if h.conf.Network != "" && h.conf.Network != "tcp" && h.conf.Network != "tcp4" && h.conf.Network != "tcp6" {
		return nil
	}

	// Check if we recently failed and should skip retry to avoid hammering
	h.portCheckCache.mu.RLock()
	lastFailure := h.portCheckCache.lastFailure
	lastError := h.portCheckCache.lastError
	retryWindow := h.portCheckCache.retryWindow
	h.portCheckCache.mu.RUnlock()

	now := time.Now()
	// If we have a recent failure within retry window, return cached error
	// This avoids hammering unreachable ports while still checking frequently
	if lastError != nil && !lastFailure.IsZero() && now.Sub(lastFailure) < retryWindow {
		return fmt.Errorf("port %s is not reachable (cached): %v", norm, lastError)
	}

	// Always perform actual network check for real-time detection
	// This ensures we detect server crashes immediately, not after cache expiry
	conn, err := net.DialTimeout("tcp", norm, 2*time.Second)
	if err != nil {
		// Cache failure to avoid frequent retries
		h.portCheckCache.mu.Lock()
		h.portCheckCache.lastFailure = now
		h.portCheckCache.lastError = err
		h.portCheckCache.mu.Unlock()
		return fmt.Errorf("port %s is not reachable: %v", norm, err)
	}
	if err := conn.Close(); err != nil {
		log.Errorf("Failed to close health check connection: %v", err)
		return fmt.Errorf("failed to close health check connection: %w", err)
	}

	// Clear cache on success - we never cache successes to ensure real-time detection
	h.portCheckCache.mu.Lock()
	h.portCheckCache.lastFailure = time.Time{} // Clear failure cache
	h.portCheckCache.lastError = nil
	h.portCheckCache.mu.Unlock()

	return nil
}
