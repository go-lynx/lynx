package kafka

import (
	"testing"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/stretchr/testify/assert"
)

func TestSASLMechanism(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.SASL
		expected bool
	}{
		{
			name: "PLAIN mechanism",
			config: &conf.SASL{
				Enabled:   true,
				Mechanism: SASLPlain,
				Username:  "testuser",
				Password:  "testpass",
			},
			expected: true,
		},
		{
			name: "SCRAM-SHA-256 mechanism",
			config: &conf.SASL{
				Enabled:   true,
				Mechanism: SASLScramSHA256,
				Username:  "testuser",
				Password:  "testpass",
			},
			expected: true,
		},
		{
			name: "SCRAM-SHA-512 mechanism",
			config: &conf.SASL{
				Enabled:   true,
				Mechanism: SASLScramSHA512,
				Username:  "testuser",
				Password:  "testpass",
			},
			expected: true,
		},
		{
			name: "SASL disabled",
			config: &conf.SASL{
				Enabled: false,
			},
			expected: false,
		},
		{
			name:     "SASL config nil",
			config:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mechanism := NewSASLMechanism(tt.config)
			if tt.config != nil && tt.config.Enabled {
				saslMech := mechanism.getMechanism()
				assert.NotNil(t, saslMech, "SASL mechanism should not be nil when enabled")
				if saslMech != nil {
					assert.Equal(t, tt.config.Mechanism, saslMech.Name(), "SASL mechanism name should match")
				}
			} else {
				saslMech := mechanism.getMechanism()
				assert.Nil(t, saslMech, "SASL mechanism should be nil when disabled or config is nil")
			}
		})
	}
}

func TestIsValidMechanism(t *testing.T) {
	tests := []struct {
		name      string
		mechanism string
		expected  bool
	}{
		{"PLAIN", SASLPlain, true},
		{"SCRAM-SHA-256", SASLScramSHA256, true},
		{"SCRAM-SHA-512", SASLScramSHA512, true},
		{"Invalid", "INVALID", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidMechanism(tt.mechanism)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSupportedMechanisms(t *testing.T) {
	mechanisms := GetSupportedMechanisms()
	expected := []string{SASLPlain, SASLScramSHA256, SASLScramSHA512}

	assert.Equal(t, len(expected), len(mechanisms))
	for _, mechanism := range expected {
		assert.Contains(t, mechanisms, mechanism)
	}
}

func TestDefaultSASLConfig(t *testing.T) {
	config := DefaultSASLConfig()

	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, SASLPlain, config.Mechanism)
	assert.Empty(t, config.Username)
	assert.Empty(t, config.Password)
}

func TestSASLConfigValidation(t *testing.T) {
	client := &Client{
		conf: &conf.Kafka{
			Sasl: &conf.SASL{
				Enabled:   true,
				Mechanism: SASLPlain,
				Username:  "testuser",
				Password:  "testpass",
			},
		},
	}

	// Test valid SASL configuration
	err := client.validateSASLConfig()
	assert.NoError(t, err)

	// Test invalid mechanism
	client.conf.Sasl.Mechanism = "INVALID"
	err = client.validateSASLConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SASL mechanism")

	// Test empty username
	client.conf.Sasl.Mechanism = SASLPlain
	client.conf.Sasl.Username = ""
	err = client.validateSASLConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SASL username is required")

	// Test empty password
	client.conf.Sasl.Username = "testuser"
	client.conf.Sasl.Password = ""
	err = client.validateSASLConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SASL password is required")

	// Test SASL disabled
	client.conf.Sasl.Enabled = false
	err = client.validateSASLConfig()
	assert.NoError(t, err)
}
