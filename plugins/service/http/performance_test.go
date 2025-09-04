package http

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestPerformanceConfig(t *testing.T) {
	// Create an HTTP plugin instance
	service := NewServiceHttp()

	// Set custom performance configurations
	service.idleTimeout = 120 * time.Second
	service.keepAliveTimeout = 60 * time.Second
	service.readHeaderTimeout = 30 * time.Second
	service.maxRequestSize = 20 * 1024 * 1024 // 20MB

	// Verify the configurations are set correctly
	assert.Equal(t, 120*time.Second, service.idleTimeout)
	assert.Equal(t, 60*time.Second, service.keepAliveTimeout)
	assert.Equal(t, 30*time.Second, service.readHeaderTimeout)
	assert.Equal(t, int64(20*1024*1024), service.maxRequestSize)
}

func TestMonitoringMetrics(t *testing.T) {
	// Create an HTTP plugin instance
	service := &ServiceHttp{}
	service.conf = &conf.Http{
		Monitoring: &conf.MonitoringConfig{},
	}

	// Initialize monitoring defaults
	service.initMonitoringDefaults()

	// Verify monitoring configuration defaults
	assert.True(t, service.conf.Monitoring.EnableMetrics)
	// Note: Other monitoring flags are not defined in the proto and are removed

	// Initialize metrics
	service.initMetrics()

	// Verify metrics are initialized
	assert.NotNil(t, httpRequestDuration)
	assert.NotNil(t, httpRequestCounter)
	assert.NotNil(t, httpResponseSize)
	assert.NotNil(t, httpActiveConnections)
	assert.NotNil(t, httpRequestQueueLength)
	assert.NotNil(t, httpConnectionPoolUsage)
	assert.NotNil(t, httpRouteRequestDuration)
	assert.NotNil(t, httpRouteRequestCounter)

	// Test metrics behavior
	// Simulate active connection
	if service.conf.Monitoring.EnableMetrics {
		httpActiveConnections.WithLabelValues("test-route").Inc()
		assert.Equal(t, float64(1), testutil.ToFloat64(httpActiveConnections.WithLabelValues("test-route")))
		httpActiveConnections.WithLabelValues("test-route").Dec()
		assert.Equal(t, float64(0), testutil.ToFloat64(httpActiveConnections.WithLabelValues("test-route")))
	}

	// Simulate request queue
	if service.conf.Monitoring.EnableMetrics {
		httpRequestQueueLength.WithLabelValues("test-route").Inc()
		assert.Equal(t, float64(1), testutil.ToFloat64(httpRequestQueueLength.WithLabelValues("test-route")))
		httpRequestQueueLength.WithLabelValues("test-route").Dec()
		assert.Equal(t, float64(0), testutil.ToFloat64(httpRequestQueueLength.WithLabelValues("test-route")))
	}

	// Simulate connection pool usage
	if service.conf.Monitoring.EnableMetrics {
		// Create a new registry to avoid conflicts with global registry
		registry := prometheus.NewRegistry()

		// Create a test gauge
		poolUsage := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_connection_pool_usage",
				Help: "Current HTTP connection pool usage percentage",
			},
			[]string{"pool"},
		)
		registry.MustRegister(poolUsage)

		// Set a value and verify
		poolUsage.WithLabelValues("http-server-pool").Set(30.0)
		assert.Equal(t, float64(30.0), testutil.ToFloat64(poolUsage.WithLabelValues("http-server-pool")))
	}
}

func TestPerformanceDefaults(t *testing.T) {
	// Create an HTTP plugin instance
	service := NewServiceHttp()

	// Apply default configuration settings
	service.initPerformanceDefaults()

	// Verify default values
	assert.Equal(t, 60*time.Second, service.idleTimeout)
	assert.Equal(t, 30*time.Second, service.keepAliveTimeout)
	assert.Equal(t, 20*time.Second, service.readHeaderTimeout)
}

func TestSecurityDefaults(t *testing.T) {
	// Create an HTTP plugin instance
	service := NewServiceHttp()

	// Apply security default configuration settings
	service.initSecurityDefaults()

	// Verify default values
	assert.Equal(t, int64(10*1024*1024), service.maxRequestSize) // 10MB
	assert.NotNil(t, service.rateLimiter)
	assert.Equal(t, rate.Limit(100), service.rateLimiter.Limit()) // 100 req/s
	assert.Equal(t, 200, service.rateLimiter.Burst())             // burst: 200
}

func TestGracefulShutdownDefaults(t *testing.T) {
	// Create an HTTP plugin instance
	service := NewServiceHttp()

	// Apply graceful shutdown default configuration settings
	service.initGracefulShutdownDefaults()

	// Verify default values
	assert.Equal(t, 30*time.Second, service.shutdownTimeout)
}

func TestConfigurationValidation(t *testing.T) {
	// Create an HTTP plugin instance
	service := NewServiceHttp()

	// Set valid configuration
	service.conf = &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Timeout: &durationpb.Duration{Seconds: 10},
	}
	service.maxRequestSize = 10 * 1024 * 1024
	service.idleTimeout = 60 * time.Second
	service.keepAliveTimeout = 30 * time.Second
	service.readHeaderTimeout = 20 * time.Second
	service.shutdownTimeout = 30 * time.Second

	// Validate configuration
	err := service.validateConfig()
	assert.NoError(t, err)

	// Test invalid address
	service.conf.Addr = "invalid-address"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid address format")

	// Test invalid port
	service.conf.Addr = ":99999"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port number")

	// Test invalid network protocol
	service.conf.Addr = ":8080" // restore valid address
	service.conf.Network = "invalid"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid network protocol")

	// Test negative request size
	service.conf.Network = "tcp" // restore valid network
	service.maxRequestSize = -1
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max request size cannot be negative")

	// Test excessively large request size
	service.maxRequestSize = 200 * 1024 * 1024 // 200MB
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max request size cannot exceed 100MB")

	// Test invalid timeout
	service.maxRequestSize = 10 * 1024 * 1024 // restore valid size
	service.conf.Timeout = &durationpb.Duration{Seconds: -1}
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be positive")

	// Test overly long timeout
	service.conf.Timeout = &durationpb.Duration{Seconds: 400} // 6.67 minutes
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout cannot exceed 5 minutes")

	// Test invalid performance configuration
	service.conf.Timeout = &durationpb.Duration{Seconds: 10} // restore valid timeout
	service.idleTimeout = -1 * time.Second
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idle timeout cannot be negative")

	// Test excessively long idle timeout
	service.idleTimeout = 700 * time.Second // 11.67 minutes
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idle timeout cannot exceed 10 minutes")

	// Test invalid rate limit configuration
	service.idleTimeout = 60 * time.Second        // restore valid idle timeout
	service.rateLimiter = rate.NewLimiter(0, 100) // 0 req/s
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit must be positive")

	// Test overly high rate limit
	service.rateLimiter = rate.NewLimiter(15000, 100) // 15k req/s
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit cannot exceed 10,000 requests per second")
}
