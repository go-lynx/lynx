package kafka

import (
	"context"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// SASLMechanism SASL authentication mechanism
type SASLMechanism struct {
	config *conf.SASL
}

// NewSASLMechanism creates a new SASL authentication mechanism
func NewSASLMechanism(config *conf.SASL) *SASLMechanism {
	return &SASLMechanism{
		config: config,
	}
}

// getSASLMechanism gets SASL authentication mechanism
func (k *Client) getSASLMechanism() sasl.Mechanism {
	if k.conf.Sasl == nil || !k.conf.Sasl.Enabled {
		return nil
	}

	mechanism := NewSASLMechanism(k.conf.Sasl)
	return mechanism.getMechanism()
}

// getMechanism gets the corresponding SASL mechanism based on configuration
func (sm *SASLMechanism) getMechanism() sasl.Mechanism {
	if sm.config == nil {
		return nil
	}

	switch sm.config.Mechanism {
	case SASLPlain:
		return sm.getPlainMechanism()
	case SASLScramSHA256:
		return sm.getScramSHA256Mechanism()
	case SASLScramSHA512:
		return sm.getScramSHA512Mechanism()
	default:
		// For unsupported mechanisms, return nil instead of logging warnings
		return nil
	}
}

// getPlainMechanism gets PLAIN authentication mechanism
func (sm *SASLMechanism) getPlainMechanism() sasl.Mechanism {
	return plain.Plain(func(ctx context.Context) (plain.Auth, error) {
		return plain.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// getScramSHA256Mechanism gets SCRAM-SHA-256 authentication mechanism
func (sm *SASLMechanism) getScramSHA256Mechanism() sasl.Mechanism {
	return scram.Sha256(func(ctx context.Context) (scram.Auth, error) {
		return scram.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// getScramSHA512Mechanism gets SCRAM-SHA-512 authentication mechanism
func (sm *SASLMechanism) getScramSHA512Mechanism() sasl.Mechanism {
	return scram.Sha512(func(ctx context.Context) (scram.Auth, error) {
		return scram.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// SASLConfig SASL configuration structure
type SASLConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Mechanism string `json:"mechanism" yaml:"mechanism"`
	Username  string `json:"username" yaml:"username"`
	Password  string `json:"password" yaml:"password"`
}

// DefaultSASLConfig default SASL configuration
func DefaultSASLConfig() *SASLConfig {
	return &SASLConfig{
		Enabled:   false,
		Mechanism: SASLPlain,
		Username:  "",
		Password:  "",
	}
}

// IsValidMechanism checks whether the SASL mechanism is valid
func IsValidMechanism(mechanism string) bool {
	validMechanisms := map[string]bool{
		SASLPlain:       true,
		SASLScramSHA256: true,
		SASLScramSHA512: true,
	}
	return validMechanisms[mechanism]
}

// GetSupportedMechanisms returns the list of supported SASL mechanisms
func GetSupportedMechanisms() []string {
	return []string{
		SASLPlain,
		SASLScramSHA256,
		SASLScramSHA512,
	}
}
