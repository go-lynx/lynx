// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// notFoundHandler returns a 404 handler.
func (h *ServiceHttp) notFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		response := map[string]interface{}{
			"code":    404,
			"message": "Resource not found",
			"path":    r.URL.Path,
			"method":  r.Method,
			"time":    time.Now().Format(time.RFC3339),
		}

		// Serialize and write the response
		if data, err := json.Marshal(response); err == nil {
			w.Write(data)
		} else {
			log.Errorf("Failed to marshal 404 response: %v", err)
			w.Write([]byte(`{"error": "Failed to serialize response"}`))
		}

		// Record 404 errors
		if h.errorCounter != nil {
			h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "not_found").Inc()
		}

		log.Warnf("404 not found: %s %s", r.Method, r.URL.Path)
	})
}

// methodNotAllowedHandler returns a 405 handler.
func (h *ServiceHttp) methodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)

		response := map[string]interface{}{
			"code":    405,
			"message": "Method not allowed",
			"path":    r.URL.Path,
			"method":  r.Method,
			"time":    time.Now().Format(time.RFC3339),
		}

		// Serialize and write the response
		if data, err := json.Marshal(response); err == nil {
			w.Write(data)
		} else {
			log.Errorf("Failed to marshal 405 response: %v", err)
			w.Write([]byte(`{"error": "Failed to serialize response"}`))
		}

		// Record 405 errors
		if h.errorCounter != nil {
			h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "method_not_allowed").Inc()
		}

		log.Warnf("405 method not allowed: %s %s", r.Method, r.URL.Path)
	})
}

// enhancedErrorEncoder is an enhanced error encoder.
func (h *ServiceHttp) enhancedErrorEncoder(w http.ResponseWriter, r *http.Request, err error) {
	// Record error metrics
	if h.errorCounter != nil {
		h.errorCounter.WithLabelValues(r.Method, r.URL.Path, "server_error").Inc()
	}

	// Delegate to the original error encoder
	EncodeErrorFunc(w, r, err)
}
