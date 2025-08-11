package kafka

import (
	"fmt"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// validateConfiguration 验证配置
func (k *Client) validateConfiguration() error {
	if len(k.conf.Brokers) == 0 {
		return ErrNoBrokersConfigured
	}

	// 验证生产者配置（多实例）
	for _, p := range k.conf.Producers {
		if p != nil && p.Enabled {
			if err := k.validateProducerConfig(p); err != nil {
				return fmt.Errorf("producer config validation failed: %w", err)
			}
		}
	}

	// 验证消费者配置（多实例）
	for _, c := range k.conf.Consumers {
		if c != nil && c.Enabled {
			if err := k.validateConsumerConfig(c); err != nil {
				return fmt.Errorf("consumer config validation failed: %w", err)
			}
		}
	}

	// 验证 SASL 配置
	if k.conf.Sasl != nil && k.conf.Sasl.Enabled {
		if err := k.validateSASLConfig(); err != nil {
			return fmt.Errorf("SASL config validation failed: %w", err)
		}
	}

	// 验证 TLS 配置
	if k.conf.Tls != nil && k.conf.Tls.Enabled {
		if err := k.validateTLSConfig(); err != nil {
			return fmt.Errorf("TLS config validation failed: %w", err)
		}
	}

	return nil
}

// validateProducerConfig 验证生产者配置
func (k *Client) validateProducerConfig(p *conf.Producer) error {
	if p.Compression != "" {
		validCompressions := map[string]bool{
			CompressionNone:   true,
			CompressionGzip:   true,
			CompressionSnappy: true,
			CompressionLz4:    true,
			CompressionZstd:   true,
		}
		if !validCompressions[p.Compression] {
			return fmt.Errorf("%w: %s", ErrInvalidCompression, p.Compression)
		}
	}
	// 校验 RequiredAcks 取值范围：允许 -1, 0, 1
	if p.RequiredAcks != -1 && p.RequiredAcks != 0 && p.RequiredAcks != 1 {
		return fmt.Errorf("invalid required_acks: %d (allowed: -1,0,1)", p.RequiredAcks)
	}
	return nil
}

// validateConsumerConfig 验证消费者配置
func (k *Client) validateConsumerConfig(c *conf.Consumer) error {
	if c.GroupId == "" {
		return ErrNoGroupID
	}

	if c.StartOffset != "" {
		validOffsets := map[string]bool{
			StartOffsetEarliest: true,
			StartOffsetLatest:   true,
		}
		if !validOffsets[c.StartOffset] {
			return fmt.Errorf("%w: %s", ErrInvalidStartOffset, c.StartOffset)
		}
	}

	if c.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be greater than 0")
	}

	return nil
}

// validateSASLConfig 验证 SASL 配置
func (k *Client) validateSASLConfig() error {
	if k.conf.Sasl == nil {
		return fmt.Errorf("SASL configuration is nil")
	}

	if !k.conf.Sasl.Enabled {
		return nil // SASL 未启用，不需要验证
	}

	// 验证机制类型
	validMechanisms := map[string]bool{
		SASLPlain:       true,
		SASLScramSHA256: true,
		SASLScramSHA512: true,
	}

	if !validMechanisms[k.conf.Sasl.Mechanism] {
		return fmt.Errorf("%w: %s", ErrInvalidSASLMechanism, k.conf.Sasl.Mechanism)
	}

	// 验证用户名和密码
	if k.conf.Sasl.Username == "" {
		return fmt.Errorf("SASL username is required when SASL is enabled")
	}
	if k.conf.Sasl.Password == "" {
		return fmt.Errorf("SASL password is required when SASL is enabled")
	}

	return nil
}

// validateTLSConfig 验证 TLS 配置
func (k *Client) validateTLSConfig() error {
	if k.conf.Tls == nil {
		return fmt.Errorf("TLS configuration is nil")
	}

	if !k.conf.Tls.Enabled {
		return nil // TLS 未启用，不需要验证
	}

	// 验证证书和密钥
	if k.conf.Tls.CertFile == "" {
		return fmt.Errorf("TLS certificate is required when TLS is enabled")
	}
	if k.conf.Tls.KeyFile == "" {
		return fmt.Errorf("TLS key is required when TLS is enabled")
	}

	return nil
}

// setDefaultValues 设置默认值
func (k *Client) setDefaultValues() {
	defaultConf := &conf.Kafka{
		DialTimeout: &durationpb.Duration{Seconds: 10},
		Producers: []*conf.Producer{
			{
				MaxRetries:   3,
				RetryBackoff: &durationpb.Duration{Seconds: 1},
				BatchSize:    1000,
				BatchTimeout: &durationpb.Duration{Seconds: 1},
				Compression:  CompressionSnappy,
				RequiredAcks: 1, // 默认 leader ack，避免默认 0 导致 no-ack 不安全
			},
		},
		Consumers: []*conf.Consumer{
			{
				AutoCommitInterval: &durationpb.Duration{Seconds: 5},
				AutoCommit:         true,
				StartOffset:        StartOffsetLatest,
				MaxConcurrency:     10,
				MinBatchSize:       1,
				MaxBatchSize:       1000,
				MaxWaitTime:        &durationpb.Duration{Seconds: 1},
				RebalanceTimeout:   &durationpb.Duration{Seconds: 30},
			},
		},
	}

	// 应用默认值
	if k.conf.DialTimeout == nil {
		k.conf.DialTimeout = defaultConf.DialTimeout
	}
	// 多生产者默认
	for _, p := range k.conf.Producers {
		if p == nil {
			continue
		}
		if p.MaxRetries == 0 {
			p.MaxRetries = defaultConf.Producers[0].MaxRetries
		}
		if p.RetryBackoff == nil {
			p.RetryBackoff = defaultConf.Producers[0].RetryBackoff
		}
		if p.BatchSize == 0 {
			p.BatchSize = defaultConf.Producers[0].BatchSize
		}
		if p.BatchTimeout == nil {
			p.BatchTimeout = defaultConf.Producers[0].BatchTimeout
		}
		if p.Compression == "" {
			p.Compression = defaultConf.Producers[0].Compression
		}
		// required_acks: 不覆盖 0
	}
	// 多消费者默认
	for _, c := range k.conf.Consumers {
		if c == nil {
			continue
		}
		if c.AutoCommitInterval == nil {
			c.AutoCommitInterval = defaultConf.Consumers[0].AutoCommitInterval
		}
		if c.StartOffset == "" {
			c.StartOffset = defaultConf.Consumers[0].StartOffset
		}
		if c.MaxConcurrency == 0 {
			c.MaxConcurrency = defaultConf.Consumers[0].MaxConcurrency
		}
		if c.MinBatchSize == 0 {
			c.MinBatchSize = defaultConf.Consumers[0].MinBatchSize
		}
		if c.MaxBatchSize == 0 {
			c.MaxBatchSize = defaultConf.Consumers[0].MaxBatchSize
		}
		if c.MaxWaitTime == nil {
			c.MaxWaitTime = defaultConf.Consumers[0].MaxWaitTime
		}
		if c.RebalanceTimeout == nil {
			c.RebalanceTimeout = defaultConf.Consumers[0].RebalanceTimeout
		}
	}
}
