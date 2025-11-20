package apollo

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/apollo/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestNewApolloConfigCenter tests plugin creation
func TestNewApolloConfigCenter(t *testing.T) {
	plugin := NewApolloConfigCenter()
	assert.NotNil(t, plugin)
	assert.Equal(t, pluginName, plugin.Name())
	assert.Equal(t, pluginVersion, plugin.Version())
	assert.Equal(t, pluginDescription, plugin.Description())
	assert.NotNil(t, plugin.healthCheckCh)
	assert.NotNil(t, plugin.configWatchers)
}

// TestPlugApollo_setDefaultConfig tests default configuration setting
func TestPlugApollo_setDefaultConfig(t *testing.T) {
	plugin := NewApolloConfigCenter()
	plugin.conf = &conf.Apollo{}

	plugin.setDefaultConfig()

	assert.Equal(t, conf.DefaultCluster, plugin.conf.Cluster)
	assert.Equal(t, conf.DefaultNamespace, plugin.conf.Namespace)
	assert.NotNil(t, plugin.conf.Timeout)
	assert.NotNil(t, plugin.conf.NotificationTimeout)
	assert.Equal(t, conf.DefaultCacheDir, plugin.conf.CacheDir)
}

// TestPlugApollo_validateConfig tests configuration validation
func TestPlugApollo_validateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Apollo
		wantErr bool
	}{
		{
			name: "valid config",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "invalid config - nil",
			config: nil,
			wantErr: true,
		},
		{
			name: "invalid config - missing app_id",
			config: &conf.Apollo{
				MetaServer: "http://localhost:8080",
			},
			wantErr: true,
		},
		{
			name: "invalid config - missing meta_server",
			config: &conf.Apollo{
				AppId: "test-app",
			},
			wantErr: true,
		},
		{
			name: "invalid config - invalid meta_server URL",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "invalid-url",
			},
			wantErr: true,
		},
		{
			name: "valid config with timeout",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
				Timeout:    durationpb.New(5 * time.Second),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := NewApolloConfigCenter()
			plugin.conf = tt.config

			err := plugin.validateConfig()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPlugApollo_checkInitialized tests initialization check
func TestPlugApollo_checkInitialized(t *testing.T) {
	plugin := NewApolloConfigCenter()

	// Test not initialized
	err := plugin.checkInitialized()
	assert.Error(t, err)

	// Test initialized
	plugin.setInitialized()
	err = plugin.checkInitialized()
	assert.NoError(t, err)

	// Test destroyed
	plugin.setDestroyed()
	err = plugin.checkInitialized()
	assert.Error(t, err)
}

// TestPlugApollo_IsInitialized tests initialization status check
func TestPlugApollo_IsInitialized(t *testing.T) {
	plugin := NewApolloConfigCenter()

	assert.False(t, plugin.IsInitialized())
	plugin.setInitialized()
	assert.True(t, plugin.IsInitialized())
}

// TestPlugApollo_IsDestroyed tests destruction status check
func TestPlugApollo_IsDestroyed(t *testing.T) {
	plugin := NewApolloConfigCenter()

	assert.False(t, plugin.IsDestroyed())
	plugin.setDestroyed()
	assert.True(t, plugin.IsDestroyed())
}

// TestPlugApollo_GetApolloConfig tests getting Apollo configuration
func TestPlugApollo_GetApolloConfig(t *testing.T) {
	plugin := NewApolloConfigCenter()
	plugin.conf = &conf.Apollo{
		AppId:      "test-app",
		MetaServer: "http://localhost:8080",
	}

	config := plugin.GetApolloConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "test-app", config.AppId)
	assert.Equal(t, "http://localhost:8080", config.MetaServer)
}

// TestPlugApollo_GetNamespace tests getting namespace
func TestPlugApollo_GetNamespace(t *testing.T) {
	plugin := NewApolloConfigCenter()

	// Test with nil config
	namespace := plugin.GetNamespace()
	assert.Equal(t, conf.DefaultNamespace, namespace)

	// Test with config
	plugin.conf = &conf.Apollo{
		Namespace: "test-namespace",
	}
	namespace = plugin.GetNamespace()
	assert.Equal(t, "test-namespace", namespace)
}

// TestNewApolloHTTPClient tests HTTP client creation
func TestNewApolloHTTPClient(t *testing.T) {
	client := NewApolloHTTPClient(
		"http://localhost:8080",
		"test-app",
		"default",
		"application",
		"test-token",
		10*time.Second,
	)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8080", client.metaServer)
	assert.Equal(t, "test-app", client.appId)
	assert.Equal(t, "default", client.cluster)
	assert.Equal(t, "application", client.namespace)
	assert.Equal(t, "test-token", client.token)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
}

// TestNewApolloHTTPClient_DefaultTimeout tests default timeout
func TestNewApolloHTTPClient_DefaultTimeout(t *testing.T) {
	client := NewApolloHTTPClient(
		"http://localhost:8080",
		"test-app",
		"default",
		"application",
		"",
		0,
	)

	assert.NotNil(t, client)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
}

// TestApolloHTTPClient_Close tests client close
func TestApolloHTTPClient_Close(t *testing.T) {
	client := NewApolloHTTPClient(
		"http://localhost:8080",
		"test-app",
		"default",
		"application",
		"",
		10*time.Second,
	)

	// Set cached config server
	client.mu.Lock()
	client.configServer = "http://config-server:8080"
	client.mu.Unlock()

	client.Close()

	// Verify config server is cleared
	client.mu.RLock()
	assert.Empty(t, client.configServer)
	client.mu.RUnlock()
}

// TestNewRetryManager tests retry manager creation
func TestNewRetryManager(t *testing.T) {
	rm := NewRetryManager(3, time.Second)
	assert.NotNil(t, rm)
	assert.Equal(t, 3, rm.maxRetries)
	assert.Equal(t, time.Second, rm.retryInterval)
	assert.Equal(t, 2.0, rm.backoffFactor)
}

// TestRetryManager_DoWithRetry tests retry functionality
func TestRetryManager_DoWithRetry(t *testing.T) {
	rm := NewRetryManager(2, 10*time.Millisecond)

	// Test successful operation
	successCount := 0
	err := rm.DoWithRetry(func() error {
		successCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, successCount)

	// Test retry on failure
	attemptCount := 0
	err = rm.DoWithRetry(func() error {
		attemptCount++
		if attemptCount < 2 {
			return assert.AnError
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, attemptCount)

	// Test max retries exhausted
	attemptCount = 0
	err = rm.DoWithRetry(func() error {
		attemptCount++
		return assert.AnError
	})
	assert.Error(t, err)
	assert.Equal(t, 3, attemptCount) // Initial attempt + 2 retries
}

// TestRetryManager_calculateBackoff tests backoff calculation
func TestRetryManager_calculateBackoff(t *testing.T) {
	rm := NewRetryManager(3, 100*time.Millisecond)

	// Test exponential backoff
	backoff1 := rm.calculateBackoff(0)
	backoff2 := rm.calculateBackoff(1)
	backoff3 := rm.calculateBackoff(2)

	assert.True(t, backoff2 > backoff1)
	assert.True(t, backoff3 > backoff2)
	assert.True(t, backoff3 <= 30*time.Second) // Max backoff limit
}

// TestNewCircuitBreaker tests circuit breaker creation
func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(0.5)
	assert.NotNil(t, cb)
	assert.Equal(t, 0.5, cb.threshold)
	assert.Equal(t, CircuitStateClosed, cb.state)
}

// TestCircuitBreaker_Do tests circuit breaker functionality
func TestCircuitBreaker_Do(t *testing.T) {
	cb := NewCircuitBreaker(0.5)

	// Test successful operation
	err := cb.Do(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, CircuitStateClosed, cb.GetState())

	// Test failure recording
	err = cb.Do(func() error {
		return assert.AnError
	})
	assert.Error(t, err)
}

// TestCircuitBreaker_GetState tests getting circuit breaker state
func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker(0.5)
	assert.Equal(t, CircuitStateClosed, cb.GetState())

	cb.ForceOpen()
	assert.Equal(t, CircuitStateOpen, cb.GetState())

	cb.ForceClose()
	assert.Equal(t, CircuitStateClosed, cb.GetState())
}

// TestCircuitBreaker_GetFailureRate tests getting failure rate
func TestCircuitBreaker_GetFailureRate(t *testing.T) {
	cb := NewCircuitBreaker(0.5)

	// No operations yet
	rate := cb.GetFailureRate()
	assert.Equal(t, 0.0, rate)

	// Record some failures and successes
	cb.Do(func() error { return nil })
	cb.Do(func() error { return assert.AnError })
	cb.Do(func() error { return assert.AnError })

	rate = cb.GetFailureRate()
	assert.Equal(t, 2.0/3.0, rate)
}

// TestCircuitBreaker_ForceOpen tests forcing circuit breaker open
func TestCircuitBreaker_ForceOpen(t *testing.T) {
	cb := NewCircuitBreaker(0.5)
	cb.ForceOpen()

	assert.Equal(t, CircuitStateOpen, cb.GetState())

	// Should fail immediately
	err := cb.Do(func() error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

// TestCircuitBreaker_ForceClose tests forcing circuit breaker close
func TestCircuitBreaker_ForceClose(t *testing.T) {
	cb := NewCircuitBreaker(0.5)
	cb.ForceOpen()
	cb.ForceClose()

	assert.Equal(t, CircuitStateClosed, cb.GetState())

	// Should work normally
	err := cb.Do(func() error {
		return nil
	})
	assert.NoError(t, err)
}

// TestNewConfigWatcher tests config watcher creation
func TestNewConfigWatcher(t *testing.T) {
	watcher := NewConfigWatcher("test-namespace")
	assert.NotNil(t, watcher)
	assert.Equal(t, "test-namespace", watcher.namespace)
	assert.NotNil(t, watcher.stopCh)
}

// TestConfigWatcher_Stop tests watcher stop
func TestConfigWatcher_Stop(t *testing.T) {
	watcher := NewConfigWatcher("test-namespace")

	// Test stop
	watcher.Stop()
	
	// Test stop again (should be idempotent)
	watcher.Stop()
}

// TestErrorCreation tests error creation functions
func TestErrorCreation(t *testing.T) {
	// Test NewConfigError
	err := NewConfigError("test config error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test config error")
	assert.Equal(t, ErrCodeConfigInvalid, err.Code)

	// Test NewInitError
	err = NewInitError("test init error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test init error")
	assert.Equal(t, ErrCodeInitFailed, err.Code)

	// Test NewClientError
	err = NewClientError(ErrCodeClientFailed, "test client error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test client error")
	assert.Equal(t, ErrCodeClientFailed, err.Code)
}

// TestErrorWrapping tests error wrapping functions
func TestErrorWrapping(t *testing.T) {
	baseErr := assert.AnError

	// Test WrapInitError
	wrapped := WrapInitError(baseErr, "test message")
	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "test message")
	assert.Equal(t, ErrCodeInitFailed, wrapped.Code)

	// Test WrapConfigError
	wrapped = WrapConfigError(baseErr, "test config message")
	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "test config message")
	assert.Equal(t, ErrCodeConfigInvalid, wrapped.Code)

	// Test WrapClientError
	wrapped = WrapClientError(baseErr, ErrCodeClientFailed, "test client message")
	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "test client message")
	assert.Equal(t, ErrCodeClientFailed, wrapped.Code)
}

// TestErrorChecking tests error checking functions
func TestErrorChecking(t *testing.T) {
	// Test IsConfigError
	configErr := NewConfigError("config error")
	assert.True(t, IsConfigError(configErr))
	assert.False(t, IsInitError(configErr))

	// Test IsInitError
	initErr := NewInitError("init error")
	assert.True(t, IsInitError(initErr))
	assert.False(t, IsConfigError(initErr))

	// Test IsClientError
	clientErr := NewClientError(ErrCodeClientFailed, "client error")
	assert.True(t, IsClientError(clientErr))
	assert.False(t, IsNetworkError(clientErr))

	// Test IsNetworkError
	networkErr := NewNetworkError("network error")
	assert.True(t, IsNetworkError(networkErr))
}

// TestApolloError_WithCause tests error with cause
func TestApolloError_WithCause(t *testing.T) {
	baseErr := assert.AnError
	err := NewConfigError("test error").WithCause(baseErr)

	assert.Error(t, err)
	assert.Equal(t, baseErr, err.Cause)
	assert.Contains(t, err.Error(), "caused by")
}

// TestApolloError_WithContext tests error with context
func TestApolloError_WithContext(t *testing.T) {
	err := NewConfigError("test error").
		WithContext("key1", "value1").
		WithContext("key2", "value2")

	assert.Error(t, err)
	assert.Equal(t, "value1", err.Context["key1"])
	assert.Equal(t, "value2", err.Context["key2"])
	assert.Contains(t, err.Error(), "context")
}

// TestApolloError_Unwrap tests error unwrap
func TestApolloError_Unwrap(t *testing.T) {
	baseErr := assert.AnError
	err := NewConfigError("test error").WithCause(baseErr)

	assert.Equal(t, baseErr, err.Unwrap())
}

// TestApolloError_Is tests error type checking
func TestApolloError_Is(t *testing.T) {
	err1 := NewConfigError("error 1")
	err2 := NewConfigError("error 2")
	err3 := NewInitError("error 3")

	assert.True(t, err1.Is(err2))
	assert.False(t, err1.Is(err3))
}

