// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
package http

import (
	"encoding/json"
	"fmt"
	"net"
	nhttp "net/http"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	obsmetrics "github.com/go-lynx/lynx/app/observability/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// initMonitoringDefaults 初始化监控默认配置
func (h *ServiceHttp) initMonitoringDefaults() {
	// 监控配置可以通过环境变量或配置文件设置
	// 暂时使用默认配置
}

// initMetrics 初始化 Prometheus 监控指标
// 全局单例指标，避免多实例重复注册导致 panic
var (
	metricsInitOnce     sync.Once
	httpRequestCounter  *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpErrorCounter    *prometheus.CounterVec
	httpHealthCheckTot  *prometheus.CounterVec
	httpInflight        *prometheus.GaugeVec
)

// ensureGlobalMetrics 初始化并在统一注册表注册一次
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

		// 使用统一注册表注册，避免多实例重复注册
		obsmetrics.MustRegister(
			httpRequestCounter,
			httpRequestDuration,
			httpResponseSize,
			httpRequestSize,
			httpErrorCounter,
			httpHealthCheckTot,
			httpInflight,
		)
	})
}

func (h *ServiceHttp) initMetrics() {
	// 确保全局指标已初始化并注册一次
	ensureGlobalMetrics()

	// 各实例复用同一组 Collector
	h.requestCounter = httpRequestCounter
	h.requestDuration = httpRequestDuration
	h.responseSize = httpResponseSize
	h.requestSize = httpRequestSize
	h.errorCounter = httpErrorCounter
	h.healthCheckTotal = httpHealthCheckTot
	h.inflightRequests = httpInflight
}

// CheckHealth 对 HTTP 服务器进行全面的健康检查。
func (h *ServiceHttp) CheckHealth() error {
	if h.server == nil {
		return fmt.Errorf("HTTP server is not initialized")
	}

	// 检查监听端口
	if h.conf.Addr != "" {
		conn, err := net.DialTimeout("tcp", h.conf.Addr, 5*time.Second)
		if err != nil {
			return fmt.Errorf("HTTP server is not listening on %s: %w", h.conf.Addr, err)
		}
		conn.Close()
	}

	// 记录健康检查指标
	if h.healthCheckTotal != nil {
		h.healthCheckTotal.WithLabelValues("success").Inc()
	}

	log.Debugf("HTTP service health check passed")
	return nil
}

// healthCheckHandler 健康检查处理器
func (h *ServiceHttp) healthCheckHandler() nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 执行健康检查
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

		// 序列化并写入响应
		w.WriteHeader(statusCode)
		if data, err := json.Marshal(response); err == nil {
			w.Write(data)
		} else {
			log.Errorf("Failed to marshal health check response: %v", err)
			w.WriteHeader(nhttp.StatusInternalServerError)
			w.Write([]byte(`{"error": "Failed to serialize response"}`))
		}
	})
}
