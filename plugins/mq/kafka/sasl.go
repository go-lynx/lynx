package kafka

import (
	"context"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// SASLMechanism SASL 认证机制
type SASLMechanism struct {
	config *conf.SASL
}

// NewSASLMechanism 创建新的 SASL 认证机制
func NewSASLMechanism(config *conf.SASL) *SASLMechanism {
	return &SASLMechanism{
		config: config,
	}
}

// getSASLMechanism 获取 SASL 认证机制
func (k *Client) getSASLMechanism() sasl.Mechanism {
	if k.conf.Sasl == nil || !k.conf.Sasl.Enabled {
		return nil
	}

	mechanism := NewSASLMechanism(k.conf.Sasl)
	return mechanism.getMechanism()
}

// getMechanism 根据配置获取对应的 SASL 机制
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
		// 对于不支持的机制，返回 nil 而不是记录警告
		return nil
	}
}

// getPlainMechanism 获取 PLAIN 认证机制
func (sm *SASLMechanism) getPlainMechanism() sasl.Mechanism {
	return plain.Plain(func(ctx context.Context) (plain.Auth, error) {
		return plain.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// getScramSHA256Mechanism 获取 SCRAM-SHA-256 认证机制
func (sm *SASLMechanism) getScramSHA256Mechanism() sasl.Mechanism {
	return scram.Sha256(func(ctx context.Context) (scram.Auth, error) {
		return scram.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// getScramSHA512Mechanism 获取 SCRAM-SHA-512 认证机制
func (sm *SASLMechanism) getScramSHA512Mechanism() sasl.Mechanism {
	return scram.Sha512(func(ctx context.Context) (scram.Auth, error) {
		return scram.Auth{
			User: sm.config.Username,
			Pass: sm.config.Password,
		}, nil
	})
}

// SASLConfig SASL 配置结构
type SASLConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Mechanism string `json:"mechanism" yaml:"mechanism"`
	Username  string `json:"username" yaml:"username"`
	Password  string `json:"password" yaml:"password"`
}

// DefaultSASLConfig 默认 SASL 配置
func DefaultSASLConfig() *SASLConfig {
	return &SASLConfig{
		Enabled:   false,
		Mechanism: SASLPlain,
		Username:  "",
		Password:  "",
	}
}

// IsValidMechanism 检查 SASL 机制是否有效
func IsValidMechanism(mechanism string) bool {
	validMechanisms := map[string]bool{
		SASLPlain:       true,
		SASLScramSHA256: true,
		SASLScramSHA512: true,
	}
	return validMechanisms[mechanism]
}

// GetSupportedMechanisms 获取支持的 SASL 机制列表
func GetSupportedMechanisms() []string {
	return []string{
		SASLPlain,
		SASLScramSHA256,
		SASLScramSHA512,
	}
}
