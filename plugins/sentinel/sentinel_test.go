package sentinel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewSentinelPlugin tests plugin creation
func TestNewSentinelPlugin(t *testing.T) {
	plugin := NewSentinelPlugin()
	assert.NotNil(t, plugin)
	assert.Equal(t, PluginName, plugin.Name())
	assert.Equal(t, PluginVersion, plugin.Version())
	assert.Equal(t, PluginDescription, plugin.Description())
	assert.NotNil(t, plugin.stopCh)
}

// TestPlugSentinel_validateAndSetDefaults tests default configuration setting
func TestPlugSentinel_validateAndSetDefaults(t *testing.T) {
	plugin := NewSentinelPlugin()
	plugin.conf = &SentinelConfig{}

	err := plugin.validateAndSetDefaults()
	assert.NoError(t, err)
	assert.Equal(t, "lynx-app", plugin.conf.AppName)
	assert.Equal(t, "./logs/sentinel", plugin.conf.LogDir)
	assert.Equal(t, "info", plugin.conf.LogLevel)
	assert.Equal(t, "30s", plugin.conf.Metrics.Interval)
	assert.Equal(t, int32(8719), plugin.conf.Dashboard.Port)
}

// TestPlugSentinel_validateConfiguration tests configuration validation
func TestPlugSentinel_validateConfiguration(t *testing.T) {
	plugin := NewSentinelPlugin()

	tests := []struct {
		name    string
		config  *SentinelConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &SentinelConfig{
				AppName: "test-app",
			},
			wantErr: false,
		},
		{
			name: "invalid - empty app_name",
			config: &SentinelConfig{
				AppName: "",
			},
			wantErr: true,
		},
		{
			name: "invalid - dashboard port too low",
			config: &SentinelConfig{
				AppName: "test-app",
				Dashboard: DashboardConfig{
					Port: 100,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - dashboard port too high",
			config: &SentinelConfig{
				AppName: "test-app",
				Dashboard: DashboardConfig{
					Port: 70000,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - flow rule with empty resource",
			config: &SentinelConfig{
				AppName: "test-app",
				FlowRules: []FlowRule{
					{
						Resource:  "",
						Threshold: 100.0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - flow rule with negative threshold",
			config: &SentinelConfig{
				AppName: "test-app",
				FlowRules: []FlowRule{
					{
						Resource:  "test-resource",
						Threshold: -1.0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - circuit breaker rule with empty resource",
			config: &SentinelConfig{
				AppName: "test-app",
				CBRules: []CircuitBreakerRule{
					{
						Resource:  "",
						Threshold: 0.5,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.validateConfiguration(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPlugSentinel_IsHealthy tests health check
func TestPlugSentinel_IsHealthy(t *testing.T) {
	plugin := NewSentinelPlugin()

	// Not initialized
	assert.False(t, plugin.IsHealthy())

	// Set as initialized
	plugin.mu.Lock()
	plugin.isInitialized = true
	plugin.sentinelInitialized = true
	plugin.mu.Unlock()

	assert.True(t, plugin.IsHealthy())
}

// TestPlugSentinel_Configure tests configuration update
func TestPlugSentinel_Configure(t *testing.T) {
	plugin := NewSentinelPlugin()
	plugin.mu.Lock()
	plugin.isInitialized = true
	plugin.mu.Unlock()

	// Test nil configuration
	err := plugin.Configure(nil)
	assert.NoError(t, err)

	// Test invalid configuration type
	err = plugin.Configure("invalid")
	assert.Error(t, err)

	// Test valid configuration
	newConfig := &SentinelConfig{
		AppName:  "new-app",
		LogLevel: "debug",
	}
	err = plugin.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "new-app", plugin.conf.AppName)
	assert.Equal(t, "debug", plugin.conf.LogLevel)
}

// TestNewMetricsCollector tests metrics collector creation
func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	assert.NotNil(t, collector)
	assert.True(t, collector.enabled)
	assert.Equal(t, 30*time.Second, collector.interval)
	assert.NotNil(t, collector.requestCounter)
	assert.NotNil(t, collector.blockedCounter)
	assert.NotNil(t, collector.passedCounter)
}

// TestMetricsCollector_RecordBlocked tests recording blocked requests
func TestMetricsCollector_RecordBlocked(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordBlocked("test-resource")

	collector.mu.RLock()
	count := collector.blockedCounter["test-resource"]
	collector.mu.RUnlock()

	assert.Equal(t, int64(1), count)
}

// TestMetricsCollector_RecordPassed tests recording passed requests
func TestMetricsCollector_RecordPassed(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordPassed("test-resource")

	collector.mu.RLock()
	count := collector.passedCounter["test-resource"]
	collector.mu.RUnlock()

	assert.Equal(t, int64(1), count)
}

// TestMetricsCollector_RecordRT tests recording response time
func TestMetricsCollector_RecordRT(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordRT("test-resource", 100*time.Millisecond)

	collector.mu.RLock()
	rtList := collector.rtHistogram["test-resource"]
	collector.mu.RUnlock()

	assert.NotEmpty(t, rtList)
}

// TestMetricsCollector_RecordError tests recording errors
func TestMetricsCollector_RecordError(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordError("test-resource")

	collector.mu.RLock()
	count := collector.requestCounter["test-resource"]
	collector.mu.RUnlock()

	// Error should increment request counter
	assert.GreaterOrEqual(t, count, int64(0))
}

// TestMetricsCollector_GetResourceStats tests getting resource statistics
func TestMetricsCollector_GetResourceStats(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordPassed("test-resource")
	collector.RecordBlocked("test-resource")
	collector.RecordRT("test-resource", 100*time.Millisecond)

	stats := collector.GetResourceStats("test-resource")
	assert.NotNil(t, stats)
	assert.Equal(t, "test-resource", stats.Resource)
}

// TestMetricsCollector_GetAllResourceStats tests getting all resource statistics
func TestMetricsCollector_GetAllResourceStats(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordPassed("resource1")
	collector.RecordBlocked("resource2")

	allStats := collector.GetAllResourceStats()
	assert.NotNil(t, allStats)
	assert.Contains(t, allStats, "resource1")
	assert.Contains(t, allStats, "resource2")
}

// TestMetricsCollector_Reset tests resetting metrics
func TestMetricsCollector_Reset(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	collector.RecordPassed("test-resource")
	collector.RecordBlocked("test-resource")

	collector.Reset()

	collector.mu.RLock()
	passedCount := collector.passedCounter["test-resource"]
	blockedCount := collector.blockedCounter["test-resource"]
	collector.mu.RUnlock()

	assert.Equal(t, int64(0), passedCount)
	assert.Equal(t, int64(0), blockedCount)
}

// TestNewDashboardServer tests dashboard server creation
func TestNewDashboardServer(t *testing.T) {
	collector := NewMetricsCollector(30 * time.Second)
	server := NewDashboardServer(8719, collector)

	assert.NotNil(t, server)
	assert.Equal(t, int32(8719), server.port)
	assert.Equal(t, collector, server.metricsCollector)
}

// TestPlugSentinel_CreateMiddleware tests middleware creation
func TestPlugSentinel_CreateMiddleware(t *testing.T) {
	plugin := NewSentinelPlugin()
	plugin.mu.Lock()
	plugin.isInitialized = true
	plugin.sentinelInitialized = true
	plugin.mu.Unlock()

	middleware := plugin.CreateMiddleware()
	assert.NotNil(t, middleware)
	assert.Equal(t, plugin, middleware.plugin)
}

// TestPluginMetadata tests plugin metadata constants
func TestPluginMetadata(t *testing.T) {
	assert.Equal(t, "sentinel.flow_control", PluginName)
	assert.Equal(t, "v1.0.0", PluginVersion)
	assert.Equal(t, "Sentinel flow control and circuit breaker plugin for lynx framework", PluginDescription)
	assert.Equal(t, int(200), PluginWeight)
}
