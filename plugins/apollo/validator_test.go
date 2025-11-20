package apollo

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/apollo/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestNewValidator tests validator creation
func TestNewValidator(t *testing.T) {
	config := &conf.Apollo{
		AppId:      "test-app",
		MetaServer: "http://localhost:8080",
	}

	validator := NewValidator(config)
	assert.NotNil(t, validator)
	assert.Equal(t, config, validator.config)
}

// TestValidator_Validate tests configuration validation
func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Apollo
		wantErr bool
	}{
		{
			name: "valid basic config",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "invalid - missing app_id",
			config: &conf.Apollo{
				MetaServer: "http://localhost:8080",
			},
			wantErr: true,
		},
		{
			name: "invalid - missing meta_server",
			config: &conf.Apollo{
				AppId: "test-app",
			},
			wantErr: true,
		},
		{
			name: "invalid - app_id too long",
			config: &conf.Apollo{
				AppId:      string(make([]byte, 129)),
				MetaServer: "http://localhost:8080",
			},
			wantErr: true,
		},
		{
			name: "invalid - invalid meta_server URL",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "invalid-url",
			},
			wantErr: true,
		},
		{
			name: "invalid - token too short",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
				Token:      "short",
			},
			wantErr: true,
		},
		{
			name: "invalid - max_retry_times out of range",
			config: &conf.Apollo{
				AppId:        "test-app",
				MetaServer:   "http://localhost:8080",
				MaxRetryTimes: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid - circuit_breaker_threshold out of range",
			config: &conf.Apollo{
				AppId:                 "test-app",
				MetaServer:            "http://localhost:8080",
				CircuitBreakerThreshold: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid - invalid log_level",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
				LogLevel:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid - timeout too small",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
				Timeout:    durationpb.New(50 * time.Millisecond),
			},
			wantErr: true,
		},
		{
			name: "invalid - timeout too large",
			config: &conf.Apollo{
				AppId:      "test-app",
				MetaServer: "http://localhost:8080",
				Timeout:    durationpb.New(40 * time.Second),
			},
			wantErr: true,
		},
		{
			name: "invalid - timeout >= notification_timeout",
			config: &conf.Apollo{
				AppId:             "test-app",
				MetaServer:        "http://localhost:8080",
				Timeout:           durationpb.New(30 * time.Second),
				NotificationTimeout: durationpb.New(10 * time.Second),
			},
			wantErr: true,
		},
		{
			name: "invalid - enable_cache without cache_dir",
			config: &conf.Apollo{
				AppId:       "test-app",
				MetaServer:  "http://localhost:8080",
				EnableCache: true,
				CacheDir:    "",
			},
			wantErr: true,
		},
		{
			name: "valid - with all optional fields",
			config: &conf.Apollo{
				AppId:             "test-app",
				MetaServer:        "http://localhost:8080",
				Cluster:           "default",
				Namespace:         "application",
				Token:             "valid-token-123",
				Timeout:           durationpb.New(10 * time.Second),
				NotificationTimeout: durationpb.New(30 * time.Second),
				EnableCache:        true,
				CacheDir:           "/tmp/cache",
				MaxRetryTimes:      3,
				CircuitBreakerThreshold: 0.5,
				LogLevel:           "info",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(tt.config)
			result := validator.Validate()

			if tt.wantErr {
				assert.False(t, result.IsValid)
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.True(t, result.IsValid)
				assert.Empty(t, result.Errors)
			}
		})
	}
}

// TestValidationResult tests validation result
func TestValidationResult(t *testing.T) {
	result := NewValidationResult()
	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)

	// Add error
	result.AddError("field1", "error message", "value1")
	assert.False(t, result.IsValid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Error(), "field1")
	assert.Contains(t, result.Error(), "error message")
}

// TestValidateConfig tests convenient validation function
func TestValidateConfig(t *testing.T) {
	validConfig := &conf.Apollo{
		AppId:      "test-app",
		MetaServer: "http://localhost:8080",
	}

	err := ValidateConfig(validConfig)
	assert.NoError(t, err)

	invalidConfig := &conf.Apollo{
		AppId: "",
	}

	err = ValidateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

