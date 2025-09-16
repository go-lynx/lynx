package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewGrpcClientPlugin(t *testing.T) {
	plugin := NewGrpcClientPlugin()
	assert.NotNil(t, plugin)
	assert.Equal(t, "grpc.client", plugin.Name())
	assert.Equal(t, "v2.0.0", plugin.Version())
	assert.Equal(t, "gRPC client plugin for Lynx framework", plugin.Description())
	assert.Equal(t, 20, plugin.Weight())
}

func TestClientConfig(t *testing.T) {
	config := ClientConfig{
		ServiceName:    "test-service",
		Endpoint:       "localhost:9090",
		TLS:            true,
		TLSAuthType:    4,
		Timeout:        10 * time.Second,
		KeepAlive:      30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   time.Second,
		MaxConnections: 10,
	}

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "localhost:9090", config.Endpoint)
	assert.True(t, config.TLS)
	assert.Equal(t, int32(4), config.TLSAuthType)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 30*time.Second, config.KeepAlive)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.RetryBackoff)
	assert.Equal(t, 10, config.MaxConnections)
}

func TestGrpcClientConfiguration(t *testing.T) {
	clientConf := &conf.GrpcClient{
		DefaultTimeout:      &durationpb.Duration{Seconds: 10},
		DefaultKeepAlive:    &durationpb.Duration{Seconds: 30},
		MaxRetries:          3,
		RetryBackoff:        &durationpb.Duration{Seconds: 1},
		MaxConnections:      10,
		TlsEnable:           true,
		TlsAuthType:         4,
		ConnectionPooling:   true,
		PoolSize:            5,
		IdleTimeout:         &durationpb.Duration{Seconds: 60},
		HealthCheckEnabled:  true,
		HealthCheckInterval: &durationpb.Duration{Seconds: 30},
		MetricsEnabled:      true,
		TracingEnabled:      true,
		LoggingEnabled:      true,
		MaxMessageSize:      4194304,
		CompressionEnabled:  false,
		CompressionType:     "gzip",
	}

	assert.NotNil(t, clientConf.GetDefaultTimeout())
	assert.Equal(t, int64(10), clientConf.GetDefaultTimeout().Seconds)
	
	assert.NotNil(t, clientConf.GetDefaultKeepAlive())
	assert.Equal(t, int64(30), clientConf.GetDefaultKeepAlive().Seconds)
	
	assert.Equal(t, int32(3), clientConf.MaxRetries)
	assert.Equal(t, int32(10), clientConf.MaxConnections)
	assert.True(t, clientConf.GetTlsEnable())
	assert.Equal(t, int32(4), clientConf.GetTlsAuthType())
	assert.True(t, clientConf.ConnectionPooling)
	assert.Equal(t, int32(5), clientConf.PoolSize)
	assert.True(t, clientConf.HealthCheckEnabled)
	assert.True(t, clientConf.MetricsEnabled)
	assert.True(t, clientConf.TracingEnabled)
	assert.True(t, clientConf.LoggingEnabled)
	assert.Equal(t, int32(4194304), clientConf.MaxMessageSize)
	assert.False(t, clientConf.CompressionEnabled)
	assert.Equal(t, "gzip", clientConf.CompressionType)
}

func TestRetryHandler(t *testing.T) {
	handler := NewRetryHandler()
	assert.NotNil(t, handler)

	// Test initialization
	handler.Initialize(5, 2*time.Second)
	
	config := handler.GetRetryConfig()
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryBackoff)
	assert.Equal(t, 30*time.Second, config.MaxBackoff)
	assert.Equal(t, 2.0, config.BackoffMultiplier)

	// Test retryable error detection
	assert.True(t, handler.isRetryableError(nil) == false)
	
	// Test exponential backoff calculation
	delay1 := ExponentialBackoff(0, time.Second, 30*time.Second, 2.0)
	assert.Equal(t, time.Second, delay1)
	
	delay2 := ExponentialBackoff(1, time.Second, 30*time.Second, 2.0)
	assert.Equal(t, 2*time.Second, delay2)
	
	delay3 := ExponentialBackoff(2, time.Second, 30*time.Second, 2.0)
	assert.Equal(t, 4*time.Second, delay3)

	// Test jitter
	jitteredDelay := Jitter(time.Second, 0.1)
	assert.True(t, jitteredDelay >= 900*time.Millisecond)
	assert.True(t, jitteredDelay <= 1100*time.Millisecond)
}

func TestClientMetrics(t *testing.T) {
	metrics := NewClientMetrics()
	assert.NotNil(t, metrics)
	assert.False(t, metrics.IsInitialized())

	// Initialize metrics
	metrics.Initialize()
	assert.True(t, metrics.IsInitialized())

	// Test recording metrics
	metrics.RecordConnectionCreated("test-service")
	metrics.RecordConnectionClosed("test-service")
	metrics.RecordConnectionFailed("test-service")
	metrics.RecordRequest("test-service", "TestMethod", "success", 100*time.Millisecond)
	metrics.RecordRequestWithDetails("test-service", "TestMethod", 200*time.Millisecond, "error")
	metrics.RecordRequestError("test-service", "TestMethod", "timeout")
	metrics.RecordRetry("test-service", "TestMethod", "timeout")
	metrics.RecordHealthCheck("test-service", 10*time.Millisecond, "healthy")
	metrics.RecordPoolSize("test-service", 5)
	metrics.RecordPoolActive("test-service", 3)
	metrics.RecordPoolIdle("test-service", 2)
	metrics.RecordMessageSize("test-service", "TestMethod", "request", 1024)
	metrics.RecordCircuitBreakerState("test-service", "open")
	metrics.RecordCircuitBreakerTrip("test-service")

	// Test getters
	assert.Equal(t, float64(0), metrics.GetConnectionCount())
	assert.Equal(t, float64(0), metrics.GetActiveConnectionCount())
}

func TestClientPluginHealthCheck(t *testing.T) {
	plugin := NewGrpcClientPlugin()
	
	// Test health check with no connections
	err := plugin.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no gRPC client connections available")

	// Test health check with nil connections
	plugin.connections = map[string]*grpc.ClientConn{
		"test-service": nil,
	}
	err = plugin.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is nil")
}

func TestClientPluginConfiguration(t *testing.T) {
	plugin := NewGrpcClientPlugin()
	
	// Test nil configuration
	err := plugin.Configure(nil)
	assert.NoError(t, err)

	// Test valid configuration
	config := &conf.GrpcClient{
		MaxRetries: 5,
		TlsEnable:  true,
	}
	err = plugin.Configure(config)
	assert.NoError(t, err)
	assert.Equal(t, config, plugin.conf)

	// Test invalid configuration type
	err = plugin.Configure("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration type")
}

func TestClientPluginConnectionManagement(t *testing.T) {
	plugin := NewGrpcClientPlugin()
	
	// Test connection count
	count := plugin.GetConnectionCount()
	assert.Equal(t, 0, count)

	// Test connection status
	status := plugin.GetConnectionStatus()
	assert.NotNil(t, status)
	assert.Empty(t, status)

	// Test close non-existent connection
	err := plugin.connectionPool.CloseConnection("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Mock context for testing
type mockContext struct{}

func (m mockContext) Deadline() (deadline time.Time, ok bool) {
	return time.Now().Add(time.Hour), true
}

func (m mockContext) Done() <-chan struct{} {
	return make(chan struct{})
}

func (m mockContext) Err() error {
	return nil
}

func (m mockContext) Value(key interface{}) interface{} {
	return nil
}

func TestRetryHandlerExecuteWithRetry(t *testing.T) {
	handler := NewRetryHandler()
	handler.Initialize(2, 100*time.Millisecond)

	// Test successful execution
	successHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	}

	ctx := context.Background()
	result, err := handler.ExecuteWithRetry(ctx, successHandler, "test")
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Test retryable error
	attemptCount := 0
	retryableErrorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, context.DeadlineExceeded // This is retryable
		}
		return "success", nil
	}

	attemptCount = 0
	result, err = handler.ExecuteWithRetry(ctx, retryableErrorHandler, "test")
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attemptCount)

	// Test non-retryable error
	nonRetryableErrorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, context.Canceled // This is not retryable
	}

	result, err = handler.ExecuteWithRetry(ctx, nonRetryableErrorHandler, "test")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

