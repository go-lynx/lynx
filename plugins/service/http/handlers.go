// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
package http

import (
	nhttp "net/http"

	"github.com/go-lynx/lynx/app/log"
)

// ========== 处理器实现 ==========

// notFoundHandler 404 处理器
func (h *ServiceHttp) notFoundHandler() nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nhttp.StatusNotFound)

		_ = map[string]interface{}{
			"code":    404,
			"message": "Resource not found",
			"path":    r.URL.Path,
			"method":  r.Method,
		}

		// 记录 404 错误
		if h.errorCounter != nil {
			h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "not_found").Inc()
		}

		log.Warnf("404 not found: %s %s", r.Method, r.URL.Path)
	})
}

// methodNotAllowedHandler 405 处理器
func (h *ServiceHttp) methodNotAllowedHandler() nhttp.Handler {
	return nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nhttp.StatusMethodNotAllowed)

		_ = map[string]interface{}{
			"code":    405,
			"message": "Method not allowed",
			"path":    r.URL.Path,
			"method":  r.Method,
		}

		// 记录 405 错误
		if h.errorCounter != nil {
			h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "method_not_allowed").Inc()
		}

		log.Warnf("405 method not allowed: %s %s", r.Method, r.URL.Path)
	})
}

// enhancedErrorEncoder 增强的错误编码器
func (h *ServiceHttp) enhancedErrorEncoder(w nhttp.ResponseWriter, r *nhttp.Request, err error) {
	// 记录错误指标
	if h.errorCounter != nil {
		h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "server_error").Inc()
	}

	// 调用原始错误编码器
	EncodeErrorFunc(w, r, err)
}
