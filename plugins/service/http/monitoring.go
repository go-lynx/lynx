// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"encoding/json"
	"fmt"
	"net"
	nhttp "net/http"
	"sync"
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

	// Simulate connection pool usage if enabled
	if h.conf.Monitoring != nil && h.conf.Monitoring.EnableConnectionMetrics {
		go func() {
			// Pool name for identification
			poolName := "http-server-pool"

			// Simulate connection pool usage with a default of 30%
			defaultUsage := 0.3
			h.connectionPoolUsage.WithLabelValues(poolName).Set(defaultUsage)

			// In a real implementation, you would monitor the actual connection pool here
			// This is just a placeholder to demonstrate the metric usage
			// You could also read from configuration if connection pool settings are available
		}()
	}
}

// CheckHealth performs a comprehensive health check for the HTTP server.
func (h *ServiceHttp) CheckHealth() error {
	if h.server == nil {
		return fmt.Errorf("HTTP server is not initialized")
	}

	// Check if the listen address is accepting connections.
	if h.conf.Addr != "" {
		conn, err := net.DialTimeout("tcp", h.conf.Addr, 5*time.Second)
		if err != nil {
			return fmt.Errorf("HTTP server is not listening on %s: %w", h.conf.Addr, err)
		}
		conn.Close()
	}

	// Record health check metrics
	if h.healthCheckTotal != nil {
		h.healthCheckTotal.WithLabelValues("success").Inc()
	}

	log.Debugf("HTTP service health check passed")
	return nil
}

// healthCheckHandler returns the health check handler.
func (h *ServiceHttp) healthCheckHandler() nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Execute health check
		err := h.CheckHealth()

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
			_, err := w.Write(data)
			if err != nil {
				return
			}
		} else {
			log.Errorf("Failed to marshal health check response: %v", err)
			w.WriteHeader(nhttp.StatusInternalServerError)
			_, err := w.Write([]byte(`{"error": "Failed to serialize response"}`))
			if err != nil {
				return
			}
		}
	})
}
