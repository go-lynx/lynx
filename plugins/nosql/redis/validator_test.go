package redis

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestValidateRedisConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.Redis
		expected bool
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: false,
		},
		{
			name: "valid single node config",
			config: &conf.Redis{
				Network:        "tcp",
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
				DialTimeout:    durationpb.New(5 * time.Second),
				ReadTimeout:    durationpb.New(5 * time.Second),
				WriteTimeout:   durationpb.New(5 * time.Second),
			},
			expected: true,
		},
		{
			name: "valid cluster config",
			config: &conf.Redis{
				Addrs:          []string{"node1:6379", "node2:6379", "node3:6379"},
				MinIdleConns:   20,
				MaxActiveConns: 100,
				PoolTimeout:    durationpb.New(2 * time.Second),
			},
			expected: true,
		},
		{
			name: "valid sentinel config",
			config: &conf.Redis{
				Addrs: []string{"sentinel1:26379", "sentinel2:26379"},
				Sentinel: &conf.Redis_Sentinel{
					MasterName: "mymaster",
					Addrs:      []string{"sentinel1:26379", "sentinel2:26379"},
				},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: true,
		},
		{
			name: "valid TLS config",
			config: &conf.Redis{
				Addrs:          []string{"rediss://localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
				Tls: &conf.Redis_TLS{
					Enabled:            true,
					InsecureSkipVerify: true,
				},
			},
			expected: true,
		},
		{
			name: "empty addrs",
			config: &conf.Redis{
				Addrs:          []string{},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "invalid address format",
			config: &conf.Redis{
				Addrs:          []string{"invalid-address"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "negative min_idle_conns",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   -1,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "min_idle_conns greater than max_active_conns",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   30,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "negative max_active_conns",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: -1,
			},
			expected: false,
		},
		{
			name: "invalid database number",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				Db:             20,
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "invalid client name",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				ClientName:     "invalid@name",
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "invalid network type",
			config: &conf.Redis{
				Network:        "invalid",
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "sentinel without master name",
			config: &conf.Redis{
				Addrs: []string{"sentinel1:26379"},
				Sentinel: &conf.Redis_Sentinel{
					MasterName: "",
					Addrs:      []string{"sentinel1:26379"},
				},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expected: false,
		},
		{
			name: "invalid timeout values",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
				DialTimeout:    durationpb.New(70 * time.Second), // Exceeds 60 second limit
				ReadTimeout:    durationpb.New(5 * time.Second),
			},
			expected: false,
		},
		{
			name: "dial_timeout greater than read_timeout",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
				DialTimeout:    durationpb.New(10 * time.Second),
				ReadTimeout:    durationpb.New(5 * time.Second),
			},
			expected: false,
		},
		{
			name: "invalid retry config",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
				MaxRetries:     15, // Exceeds 10 retry limit
			},
			expected: false,
		},
		{
			name: "min_retry_backoff greater than max_retry_backoff",
			config: &conf.Redis{
				Addrs:           []string{"localhost:6379"},
				MinIdleConns:    10,
				MaxActiveConns:  20,
				MinRetryBackoff: durationpb.New(1 * time.Second),
				MaxRetryBackoff: durationpb.New(500 * time.Millisecond),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRedisConfig(tt.config)
			if result.IsValid != tt.expected {
				t.Errorf("ValidateRedisConfig() = %v, expected %v", result.IsValid, tt.expected)
				if !result.IsValid {
					t.Logf("Validation errors: %s", result.Error())
				}
			}
		})
	}
}

func TestValidateAndSetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		config      *conf.Redis
		expectError bool
	}{
		{
			name: "valid config with defaults",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   10,
				MaxActiveConns: 20,
			},
			expectError: false,
		},
		{
			name: "config with missing required fields",
			config: &conf.Redis{
				Addrs: []string{},
			},
			expectError: true,
		},
		{
			name: "config with invalid values",
			config: &conf.Redis{
				Addrs:          []string{"localhost:6379"},
				MinIdleConns:   30,
				MaxActiveConns: 20,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAndSetDefaults(tt.config)
			if tt.expectError && err == nil {
				t.Errorf("ValidateAndSetDefaults() expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateAndSetDefaults() unexpected error: %v", err)
			}
		})
	}
}

func TestSetDefaultValues(t *testing.T) {
	config := &conf.Redis{
		Addrs: []string{"localhost:6379"},
	}

	setDefaultValues(config)

	// Verify that default values are correctly set
	if config.Network != "tcp" {
		t.Errorf("Expected Network to be 'tcp', got %s", config.Network)
	}
	if config.MinIdleConns != 10 {
		t.Errorf("Expected MinIdleConns to be 10, got %d", config.MinIdleConns)
	}
	if config.MaxIdleConns != 20 {
		t.Errorf("Expected MaxIdleConns to be 20, got %d", config.MaxIdleConns)
	}
	if config.MaxActiveConns != 20 {
		t.Errorf("Expected MaxActiveConns to be 20, got %d", config.MaxActiveConns)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}
	if config.DialTimeout == nil {
		t.Error("Expected DialTimeout to be set")
	}
	if config.ReadTimeout == nil {
		t.Error("Expected ReadTimeout to be set")
	}
	if config.WriteTimeout == nil {
		t.Error("Expected WriteTimeout to be set")
	}
	if config.PoolTimeout == nil {
		t.Error("Expected PoolTimeout to be set")
	}
	if config.ConnMaxIdleTime == nil {
		t.Error("Expected ConnMaxIdleTime to be set")
	}
	if config.MaxConnAge == nil {
		t.Error("Expected MaxConnAge to be set")
	}
	if config.MinRetryBackoff == nil {
		t.Error("Expected MinRetryBackoff to be set")
	}
	if config.MaxRetryBackoff == nil {
		t.Error("Expected MaxRetryBackoff to be set")
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "test_field",
		Message: "test message",
	}

	expected := "validation error in field 'test_field': test message"
	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %s, expected %s", err.Error(), expected)
	}
}

func TestValidationResult(t *testing.T) {
	result := &ValidationResult{IsValid: true}

	// Test adding error
	result.AddError("field1", "error1")
	if result.IsValid {
		t.Error("Expected IsValid to be false after adding error")
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	// Test adding multiple errors
	result.AddError("field2", "error2")
	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}

	// Test error message format
	errorMsg := result.Error()
	expected := "validation error in field 'field1': error1; validation error in field 'field2': error2"
	if errorMsg != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, errorMsg)
	}
}
