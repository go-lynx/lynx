// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
package http

import (
	"encoding/json"
	"fmt"
	"net"
	nhttp "net/http"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/prometheus/client_golang/prometheus"
)

// initMonitoringDefaults 初始化监控默认配置
func (h *ServiceHttp) initMonitoringDefaults() {
	// 监控配置可以通过环境变量或配置文件设置
	// 暂时使用默认配置
}

// initMetrics 初始化 Prometheus 监控指标
func (h *ServiceHttp) initMetrics() {
	// 请求计数器
	h.requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// 请求持续时间
	h.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "lynx",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// 响应大小
	h.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "lynx",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	// 错误计数器
	h.errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "http",
			Name:      "errors_total",
			Help:      "Total number of HTTP errors",
		},
		[]string{"method", "path", "error_type"},
	)

	// 健康检查计数器
	h.healthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "http",
			Name:      "health_check_total",
			Help:      "Total number of health checks",
		},
		[]string{"status"},
	)

	// 注册指标（默认注册表）
	prometheus.MustRegister(
		h.requestCounter,
		h.requestDuration,
		h.responseSize,
		h.errorCounter,
		h.healthCheckTotal,
	)
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
