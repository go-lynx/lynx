package snowflake

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

func TestPlugSnowflake_GetHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupPlugin    func() *PlugSnowflake
		expectedStatus string
		checkDetails   func(t *testing.T, details map[string]any)
	}{
		{
			name: "healthy_plugin_with_generator",
			setupPlugin: func() *PlugSnowflake {
				plugin := NewSnowflakePlugin()

				// Initialize generator
				generator := &Generator{
					datacenterID:       1,
					workerID:           1,
					customEpoch:        1640995200000, // 2022-01-01
					generatedCount:     100,
					clockBackwardCount: 0,
					isShuttingDown:     false,
				}
				plugin.generator = generator

				// Initialize metrics
				plugin.metrics = &Metrics{
					IDsGenerated:       100,
					GenerationErrors:   0,
					IDGenerationRate:   1000.0,
					UptimeDuration:     time.Hour,
					LastGenerationTime: time.Now(),
				}

				return plugin
			},
			expectedStatus: "healthy",
			checkDetails: func(t *testing.T, details map[string]any) {
				assert.Equal(t, "initialized", details["generator_status"])
				assert.Equal(t, int64(1), details["worker_id"])
				assert.Equal(t, int64(1), details["datacenter_id"])
				assert.Equal(t, int64(100), details["generated_count"])
				assert.Equal(t, int64(0), details["clock_backward_count"])
				assert.Equal(t, false, details["is_shutting_down"])
				assert.Equal(t, "not_configured", details["redis_status"])

				// Check metrics
				metrics, ok := details["metrics"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, int64(100), metrics["ids_generated"])
				assert.Equal(t, int64(0), metrics["generation_errors"])
				assert.Equal(t, 1000.0, metrics["id_generation_rate"])
			},
		},
		{
			name: "unhealthy_plugin_no_generator",
			setupPlugin: func() *PlugSnowflake {
				plugin := NewSnowflakePlugin()
				// Don't initialize generator
				return plugin
			},
			expectedStatus: "unhealthy",
			checkDetails: func(t *testing.T, details map[string]any) {
				assert.Equal(t, "not_initialized", details["generator_status"])
				assert.Equal(t, "not_configured", details["redis_status"])
			},
		},
		{
			name: "degraded_plugin_with_clock_drift",
			setupPlugin: func() *PlugSnowflake {
				plugin := NewSnowflakePlugin()

				// Initialize generator with clock drift
				generator := &Generator{
					datacenterID:       1,
					workerID:           1,
					customEpoch:        1640995200000,
					generatedCount:     100,
					clockBackwardCount: 5, // Clock drift detected
					isShuttingDown:     false,
				}
				plugin.generator = generator

				return plugin
			},
			expectedStatus: "degraded",
			checkDetails: func(t *testing.T, details map[string]any) {
				assert.Equal(t, "initialized", details["generator_status"])
				assert.Equal(t, int64(5), details["clock_backward_count"])
			},
		},
		{
			name: "degraded_plugin_with_high_error_rate",
			setupPlugin: func() *PlugSnowflake {
				plugin := NewSnowflakePlugin()

				// Initialize generator
				generator := &Generator{
					datacenterID:       1,
					workerID:           1,
					customEpoch:        1640995200000,
					generatedCount:     100,
					clockBackwardCount: 0,
					isShuttingDown:     false,
				}
				plugin.generator = generator

				// Initialize metrics with high error rate
				plugin.metrics = &Metrics{
					IDsGenerated:       70, // 70 successful
					GenerationErrors:   30, // 30 errors = 30% error rate
					IDGenerationRate:   1000.0,
					UptimeDuration:     time.Hour,
					LastGenerationTime: time.Now(),
				}

				return plugin
			},
			expectedStatus: "degraded",
			checkDetails: func(t *testing.T, details map[string]any) {
				assert.Equal(t, "initialized", details["generator_status"])
				errorRate, ok := details["error_rate"].(float64)
				require.True(t, ok)
				assert.Greater(t, errorRate, 0.1) // More than 10%
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := tt.setupPlugin()

			health := plugin.GetHealth()

			assert.Equal(t, tt.expectedStatus, health.Status)
			assert.NotEmpty(t, health.Message)
			assert.Greater(t, health.Timestamp, int64(0))
			assert.NotNil(t, health.Details)

			if tt.checkDetails != nil {
				tt.checkDetails(t, health.Details)
			}
		})
	}
}

func TestPlugSnowflake_GetHealth_WithConfiguration(t *testing.T) {
	plugin := NewSnowflakePlugin()

	// Set up configuration
	config := &pb.Snowflake{
		DatacenterId:               1,
		WorkerId:                   2,
		CustomEpoch:                1640995200000,
		AutoRegisterWorkerId:       true,
		RedisPluginName:            "redis",
		RedisKeyPrefix:             "snowflake:",
		WorkerIdTtl:                durationpb.New(300 * time.Second),
		HeartbeatInterval:          durationpb.New(30 * time.Second),
		EnableMetrics:              true,
		EnableClockDriftProtection: true,
		EnableSequenceCache:        true,
		SequenceCacheSize:          1000,
		RedisDb:                    0,
		WorkerIdBits:               5,
		SequenceBits:               12,
	}
	plugin.conf = config

	health := plugin.GetHealth()

	// Check configuration details
	configDetails, ok := health.Details["configuration"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, int32(1), configDetails["datacenter_id"])
	assert.Equal(t, int32(2), configDetails["worker_id"])
	assert.Equal(t, int64(1640995200000), configDetails["custom_epoch"])
	assert.Equal(t, true, configDetails["auto_register_worker_id"])
	assert.Equal(t, "redis", configDetails["redis_plugin_name"])
	assert.Equal(t, "snowflake:", configDetails["redis_key_prefix"])
	assert.NotNil(t, configDetails["worker_id_ttl"])
	assert.NotNil(t, configDetails["heartbeat_interval"])
	assert.Equal(t, true, configDetails["enable_metrics"])
	assert.Equal(t, true, configDetails["clock_drift_protection"])
	assert.Equal(t, true, configDetails["sequence_cache"])
	assert.Equal(t, int32(1000), configDetails["sequence_cache_size"])
	assert.Equal(t, int32(0), configDetails["redis_db"])
	assert.Equal(t, int32(5), configDetails["worker_id_bits"])
	assert.Equal(t, int32(12), configDetails["sequence_bits"])
}

func TestPlugSnowflake_GetHealth_WithWorkerManager(t *testing.T) {
	plugin := NewSnowflakePlugin()

	// Set up worker manager
	workerManager := &WorkerIDManager{
		workerID:          1,
		datacenterID:      2,
		keyPrefix:         "snowflake:",
		ttl:               5 * time.Minute,
		heartbeatInterval: 30 * time.Second,
	}
	plugin.workerManager = workerManager

	health := plugin.GetHealth()

	assert.Equal(t, "active", health.Details["worker_manager_status"])
	assert.Equal(t, int64(1), health.Details["worker_manager_worker_id"])
	assert.Equal(t, int64(2), health.Details["worker_manager_datacenter_id"])
	assert.Equal(t, "snowflake:", health.Details["worker_manager_key_prefix"])
	assert.Equal(t, "5m0s", health.Details["worker_manager_ttl"])
	assert.Equal(t, "30s", health.Details["worker_manager_heartbeat_interval"])
}

func TestPlugSnowflake_GetHealth_ThreadSafety(t *testing.T) {
	plugin := NewSnowflakePlugin()

	// Initialize generator
	generator := &Generator{
		datacenterID:       1,
		workerID:           1,
		customEpoch:        1640995200000,
		generatedCount:     100,
		clockBackwardCount: 0,
		isShuttingDown:     false,
	}
	plugin.generator = generator

	// Initialize metrics
	plugin.metrics = &Metrics{
		IDsGenerated:       100,
		GenerationErrors:   0,
		IDGenerationRate:   1000.0,
		UptimeDuration:     time.Hour,
		LastGenerationTime: time.Now(),
	}

	// Run multiple goroutines calling GetHealth concurrently
	const numGoroutines = 10
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			health := plugin.GetHealth()
			results <- health.Status
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		status := <-results
		assert.Equal(t, "healthy", status)
	}
}
