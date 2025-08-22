package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestHTTPPluginIntegration tests the complete integration of the HTTP plugin
func TestHTTPPluginIntegration(t *testing.T) {
	// Create a simple test configuration
	testConfig := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Timeout: &durationpb.Duration{Seconds: 5},
		Monitoring: &conf.MonitoringConfig{
			EnableMetrics: true,
			MetricsPath:   "/metrics",
			HealthPath:    "/health",
		},
		Middleware: &conf.MiddlewareConfig{
			EnableTracing:    true,
			EnableLogging:    true,
			EnableMetrics:    true,
			EnableRecovery:   true,
			EnableValidation: true,
			EnableRateLimit:  true,
		},
		Security: &conf.SecurityConfig{
			MaxRequestSize: 10 * 1024 * 1024, // 10MB
		},
		Performance: &conf.PerformanceConfig{
			ReadTimeout:  &durationpb.Duration{Seconds: 30},
			WriteTimeout: &durationpb.Duration{Seconds: 30},
			IdleTimeout:  &durationpb.Duration{Seconds: 60},
		},
	}

	// Test plugin lifecycle
	t.Run("Plugin Lifecycle", func(t *testing.T) {
		// Create HTTP plugin
		httpPlugin := NewServiceHttp()

		// Test initialization with configuration
		httpPlugin.conf = testConfig
		err := httpPlugin.validateConfig()
		require.NoError(t, err)
		assert.NotNil(t, httpPlugin.conf)

		// Test configuration validation
		assert.Equal(t, "tcp", httpPlugin.conf.Network)
		assert.Equal(t, ":8080", httpPlugin.conf.Addr)
		assert.Equal(t, int64(10*1024*1024), httpPlugin.conf.Security.MaxRequestSize)
	})

	// Test configuration updates
	t.Run("Configuration Updates", func(t *testing.T) {
		httpPlugin := NewServiceHttp()

		// Test valid configuration update
		newConfig := &conf.Http{
			Network: "tcp",
			Addr:    ":8081",
			Timeout: &durationpb.Duration{Seconds: 10},
		}
		err := httpPlugin.Configure(newConfig)
		require.NoError(t, err)

		// Test invalid configuration update
		invalidConfig := &conf.Http{
			Network: "invalid",
			Addr:    "invalid-address",
		}
		err = httpPlugin.Configure(invalidConfig)
		assert.Error(t, err)
	})

	// Test error handling
	t.Run("Error Handling", func(t *testing.T) {
		// Test invalid address
		invalidPlugin := NewServiceHttp()
		invalidPlugin.conf = &conf.Http{
			Network: "tcp",
			Addr:    "invalid-address",
		}
		err := invalidPlugin.validateConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid address format")

		// Test invalid timeout
		invalidPlugin.conf = &conf.Http{
			Network: "tcp",
			Addr:    ":8080",
			Timeout: &durationpb.Duration{Seconds: -1},
		}
		err = invalidPlugin.validateConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be positive")
	})
}

// TestHTTPPluginWithMiddleware tests middleware integration
func TestHTTPPluginWithMiddleware(t *testing.T) {
	httpPlugin := NewServiceHttp()

	// Configure with all middlewares enabled
	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Middleware: &conf.MiddlewareConfig{
			EnableTracing:    true,
			EnableLogging:    true,
			EnableMetrics:    true,
			EnableRecovery:   true,
			EnableValidation: true,
			EnableRateLimit:  true,
		},
	}

	httpPlugin.conf = config

	// Test middleware chain building
	middlewares := httpPlugin.buildMiddlewares()
	assert.NotEmpty(t, middlewares)
	assert.GreaterOrEqual(t, len(middlewares), 5) // At least 5 middlewares should be present
}

// TestHTTPPluginMetrics tests metrics integration
func TestHTTPPluginMetrics(t *testing.T) {
	httpPlugin := NewServiceHttp()

	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Monitoring: &conf.MonitoringConfig{
			EnableMetrics:           true,
			EnableRouteMetrics:      true,
			EnableConnectionMetrics: true,
			EnableQueueMetrics:      true,
		},
	}

	httpPlugin.conf = config

	// Test metrics initialization
	httpPlugin.initMetrics()

	// Verify metrics are registered
	assert.NotNil(t, httpRequestCounter)
	assert.NotNil(t, httpRequestDuration)
	assert.NotNil(t, httpResponseSize)
	assert.NotNil(t, httpActiveConnections)
	assert.NotNil(t, httpRequestQueueLength)

	// Test metrics recording
	httpRequestCounter.WithLabelValues("GET", "/test", "200").Inc()
	httpRequestDuration.WithLabelValues("GET", "/test").Observe(0.1)
	httpActiveConnections.WithLabelValues("/test").Inc()
}

// TestHTTPPluginGracefulShutdown tests graceful shutdown functionality
func TestHTTPPluginGracefulShutdown(t *testing.T) {
	httpPlugin := NewServiceHttp()

	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		GracefulShutdown: &conf.GracefulShutdownConfig{
			ShutdownTimeout:        &durationpb.Duration{Seconds: 5},
			WaitForOngoingRequests: true,
		},
	}

	httpPlugin.conf = config

	// Test graceful shutdown configuration
	assert.Equal(t, 5*time.Second, config.GracefulShutdown.ShutdownTimeout.AsDuration())
	assert.True(t, config.GracefulShutdown.WaitForOngoingRequests)
}

// TestHTTPPluginTLS tests TLS configuration (without actual certificates)
func TestHTTPPluginTLS(t *testing.T) {
	httpPlugin := NewServiceHttp()

	// Test TLS configuration without certificate provider
	config := &conf.Http{
		Network:     "tcp",
		Addr:        ":8080",
		TlsEnable:   true,
		TlsAuthType: 0,
	}

	httpPlugin.conf = config

	// Test TLS loading without certificate provider (should fail gracefully)
	_, err := httpPlugin.tlsLoad()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate provider not configured")
}

// TestHTTPPluginHealthCheck tests health check functionality
func TestHTTPPluginHealthCheck(t *testing.T) {
	httpPlugin := NewServiceHttp()

	// Test health check handler creation
	handler := httpPlugin.healthCheckHandler()
	assert.NotNil(t, handler)

	// Test health check response
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	// Create a test response writer
	rr := &testResponseWriter{}
	handler.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rr.statusCode)
	assert.Contains(t, rr.body, "status")
}

// testResponseWriter is a simple response writer for testing
type testResponseWriter struct {
	statusCode int
	body       string
	headers    http.Header
}

func (w *testResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	w.body = string(data)
	return len(data), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// BenchmarkHTTPPlugin tests performance characteristics
func BenchmarkHTTPPlugin(b *testing.B) {
	httpPlugin := NewServiceHttp()

	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Performance: &conf.PerformanceConfig{
			ReadTimeout:  &durationpb.Duration{Seconds: 30},
			WriteTimeout: &durationpb.Duration{Seconds: 30},
			IdleTimeout:  &durationpb.Duration{Seconds: 60},
		},
	}

	httpPlugin.conf = config

	// Benchmark configuration validation
	b.Run("ConfigValidation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = httpPlugin.validateConfig()
		}
	})

	// Benchmark middleware building
	b.Run("MiddlewareBuilding", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = httpPlugin.buildMiddlewares()
		}
	})

	// Benchmark metrics recording
	b.Run("MetricsRecording", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			httpRequestCounter.WithLabelValues("GET", "/benchmark", "200").Inc()
			httpRequestDuration.WithLabelValues("GET", "/benchmark").Observe(0.1)
		}
	})
}
