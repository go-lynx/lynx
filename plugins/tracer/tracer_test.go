package tracer

import (
	"testing"

	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestPlugTracer_ValidateConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Tracer
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
			wantErr: false,
		},
		{
			name: "invalid ratio too high",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid ratio too low",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  -0.1,
			},
			wantErr: true,
		},
		{
			name: "enabled but no address",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "",
				Ratio:  0.5,
			},
			wantErr: true,
		},
		{
			name: "disabled configuration",
			config: &conf.Tracer{
				Enable: false,
				Addr:   "",
				Ratio:  0.5,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with nested config",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
				Config: &conf.Config{
					Batch: &conf.Batch{
						Enabled:      true,
						MaxQueueSize: 1000,
						MaxBatchSize: 100,
					},
					Retry: &conf.Retry{
						Enabled:         true,
						MaxAttempts:     3,
						InitialInterval: durationpb.New(100000000),  // 100ms
						MaxInterval:     durationpb.New(1000000000), // 1s
					},
					Sampler: &conf.Sampler{
						Type:  conf.Sampler_PARENT_BASED_TRACEID_RATIO,
						Ratio: 0.1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid batch configuration",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
				Config: &conf.Config{
					Batch: &conf.Batch{
						Enabled:      true,
						MaxQueueSize: -100,
						MaxBatchSize: 100,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid retry configuration",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
				Config: &conf.Config{
					Retry: &conf.Retry{
						Enabled:     true,
						MaxAttempts: 0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid sampler configuration",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
				Config: &conf.Config{
					Sampler: &conf.Sampler{
						Type:  conf.Sampler_TRACEID_RATIO,
						Ratio: 1.5,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &PlugTracer{
				conf: tt.config,
			}
			err := tracer.validateConfiguration()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPlugTracer_ValidateConfigFields(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Config
		wantErr bool
	}{
		{
			name: "valid batch config",
			config: &conf.Config{
				Batch: &conf.Batch{
					Enabled:      true,
					MaxQueueSize: 1000,
					MaxBatchSize: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid batch config - negative queue size",
			config: &conf.Config{
				Batch: &conf.Batch{
					Enabled:      true,
					MaxQueueSize: -100,
					MaxBatchSize: 100,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid batch config - batch size exceeds queue size",
			config: &conf.Config{
				Batch: &conf.Batch{
					Enabled:      true,
					MaxQueueSize: 100,
					MaxBatchSize: 200,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid batch config - no limits configured",
			config: &conf.Config{
				Batch: &conf.Batch{
					Enabled:      true,
					MaxQueueSize: 0,
					MaxBatchSize: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "valid retry config",
			config: &conf.Config{
				Retry: &conf.Retry{
					Enabled:         true,
					MaxAttempts:     3,
					InitialInterval: durationpb.New(100000000),
					MaxInterval:     durationpb.New(1000000000),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid retry config - zero max attempts",
			config: &conf.Config{
				Retry: &conf.Retry{
					Enabled:     true,
					MaxAttempts: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid retry config - initial interval exceeds max interval",
			config: &conf.Config{
				Retry: &conf.Retry{
					Enabled:         true,
					MaxAttempts:     3,
					InitialInterval: durationpb.New(2000000000), // 2s
					MaxInterval:     durationpb.New(1000000000), // 1s
				},
			},
			wantErr: true,
		},
		{
			name: "valid sampler config",
			config: &conf.Config{
				Sampler: &conf.Sampler{
					Type:  conf.Sampler_TRACEID_RATIO,
					Ratio: 0.5,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid sampler config - ratio out of range",
			config: &conf.Config{
				Sampler: &conf.Sampler{
					Type:  conf.Sampler_TRACEID_RATIO,
					Ratio: 1.5,
				},
			},
			wantErr: true,
		},
		{
			name: "valid connection config",
			config: &conf.Config{
				Connection: &conf.Connection{
					ConnectTimeout:     durationpb.New(10000000000),  // 10s
					ReconnectionPeriod: durationpb.New(5000000000),   // 5s
					MaxConnAge:         durationpb.New(600000000000), // 10m
					MaxConnIdleTime:    durationpb.New(300000000000), // 5m
					MaxConnAgeGrace:    durationpb.New(10000000000),  // 10s
				},
			},
			wantErr: false,
		},
		{
			name: "invalid connection config - negative timeout",
			config: &conf.Config{
				Connection: &conf.Connection{
					ConnectTimeout: durationpb.New(-1000000000), // -1s
				},
			},
			wantErr: true,
		},
		{
			name: "invalid connection config - max_conn_age less than max_conn_idle_time",
			config: &conf.Config{
				Connection: &conf.Connection{
					MaxConnAge:      durationpb.New(300000000000), // 5m
					MaxConnIdleTime: durationpb.New(600000000000), // 10m
				},
			},
			wantErr: true,
		},
		{
			name: "valid load balancing config",
			config: &conf.Config{
				LoadBalancing: &conf.LoadBalancing{
					Policy:      "round_robin",
					HealthCheck: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid load balancing config - invalid policy",
			config: &conf.Config{
				LoadBalancing: &conf.LoadBalancing{
					Policy:      "invalid_policy",
					HealthCheck: false,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &PlugTracer{
				conf: &conf.Tracer{Config: tt.config},
			}
			err := tracer.validateConfigFields()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSamplerHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		ratio    float32
		expected bool
	}{
		{"valid ratio 0.0", 0.0, true},
		{"valid ratio 0.5", 0.5, true},
		{"valid ratio 1.0", 1.0, true},
		{"invalid ratio -0.1", -0.1, false},
		{"invalid ratio 1.1", 1.1, false},
		{"edge case -epsilon", -1e-10, true},       // Within epsilon
		{"edge case 1+epsilon", 1.0 + 1e-10, true}, // Within epsilon
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRatio(tt.ratio)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClampRatio(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		expected float32
	}{
		{"normal value", 0.5, 0.5},
		{"negative value", -0.5, 0.0},
		{"value > 1", 1.5, 1.0},
		{"edge case 0", 0.0, 0.0},
		{"edge case 1", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampRatio(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlugTracer_SetDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.Tracer
		expected *conf.Tracer
	}{
		{
			name: "set default addr",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "",
				Ratio:  0.5,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
		},
		{
			name: "set default ratio",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  1.0,
			},
		},
		{
			name: "no defaults needed",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &PlugTracer{
				conf: tt.config,
			}
			tracer.setDefaultValues()

			assert.Equal(t, tt.expected.Addr, tracer.conf.Addr)
			assert.Equal(t, tt.expected.Ratio, tracer.conf.Ratio)
		})
	}
}

func TestNewPlugTracer(t *testing.T) {
	tracer := NewPlugTracer()

	assert.NotNil(t, tracer)
	assert.NotNil(t, tracer.BasePlugin)
	assert.NotNil(t, tracer.conf)
}
