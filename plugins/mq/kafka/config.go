package kafka

import (
	"fmt"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// validateConfiguration 验证配置
func (k *KafkaClient) validateConfiguration() error {
	if len(k.conf.Brokers) == 0 {
		return ErrNoBrokersConfigured
	}

	// 验证生产者配置
	if k.conf.Producer != nil && k.conf.Producer.Enabled {
		if err := k.validateProducerConfig(); err != nil {
			return fmt.Errorf("producer config validation failed: %w", err)
		}
	}

	// 验证消费者配置
	if k.conf.Consumer != nil && k.conf.Consumer.Enabled {
		if err := k.validateConsumerConfig(); err != nil {
			return fmt.Errorf("consumer config validation failed: %w", err)
		}
	}

	// 验证 SASL 配置
	if k.conf.Sasl != nil && k.conf.Sasl.Enabled {
		if err := k.validateSASLConfig(); err != nil {
			return fmt.Errorf("SASL config validation failed: %w", err)
		}
	}

	return nil
}

// validateProducerConfig 验证生产者配置
func (k *KafkaClient) validateProducerConfig() error {
	if k.conf.Producer.Compression != "" {
		validCompressions := map[string]bool{
			CompressionNone:   true,
			CompressionGzip:   true,
			CompressionSnappy: true,
			CompressionLz4:    true,
			CompressionZstd:   true,
		}
		if !validCompressions[k.conf.Producer.Compression] {
			return fmt.Errorf("%w: %s", ErrInvalidCompression, k.conf.Producer.Compression)
		}
	}
	return nil
}

// validateConsumerConfig 验证消费者配置
func (k *KafkaClient) validateConsumerConfig() error {
	if k.conf.Consumer.GroupId == "" {
		return ErrNoGroupID
	}

	if k.conf.Consumer.StartOffset != "" {
		validOffsets := map[string]bool{
			StartOffsetEarliest: true,
			StartOffsetLatest:   true,
		}
		if !validOffsets[k.conf.Consumer.StartOffset] {
			return fmt.Errorf("%w: %s", ErrInvalidStartOffset, k.conf.Consumer.StartOffset)
		}
	}

	if k.conf.Consumer.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be greater than 0")
	}

	return nil
}

// validateSASLConfig 验证 SASL 配置
func (k *KafkaClient) validateSASLConfig() error {
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

// setDefaultValues 设置默认值
func (k *KafkaClient) setDefaultValues() {
	defaultConf := &conf.Kafka{
		DialTimeout: &durationpb.Duration{Seconds: 10},
		Producer: &conf.Producer{
			MaxRetries:   3,
			RetryBackoff: &durationpb.Duration{Seconds: 1},
			BatchSize:    1000,
			BatchTimeout: &durationpb.Duration{Seconds: 1},
			Compression:  CompressionSnappy,
			RequiredAcks: true,
		},
		Consumer: &conf.Consumer{
			AutoCommitInterval: &durationpb.Duration{Seconds: 5},
			AutoCommit:         true,
			StartOffset:        StartOffsetLatest,
			MaxConcurrency:     10,
			MinBatchSize:       1,
			MaxBatchSize:       1000,
			MaxWaitTime:        &durationpb.Duration{Seconds: 1},
			RebalanceTimeout:   &durationpb.Duration{Seconds: 30},
		},
	}

	// 应用默认值
	if k.conf.DialTimeout == nil {
		k.conf.DialTimeout = defaultConf.DialTimeout
	}
	if k.conf.Producer != nil {
		if k.conf.Producer.MaxRetries == 0 {
			k.conf.Producer.MaxRetries = defaultConf.Producer.MaxRetries
		}
		if k.conf.Producer.RetryBackoff == nil {
			k.conf.Producer.RetryBackoff = defaultConf.Producer.RetryBackoff
		}
		if k.conf.Producer.BatchSize == 0 {
			k.conf.Producer.BatchSize = defaultConf.Producer.BatchSize
		}
		if k.conf.Producer.BatchTimeout == nil {
			k.conf.Producer.BatchTimeout = defaultConf.Producer.BatchTimeout
		}
		if k.conf.Producer.Compression == "" {
			k.conf.Producer.Compression = defaultConf.Producer.Compression
		}
	}
	if k.conf.Consumer != nil {
		if k.conf.Consumer.AutoCommitInterval == nil {
			k.conf.Consumer.AutoCommitInterval = defaultConf.Consumer.AutoCommitInterval
		}
		if k.conf.Consumer.StartOffset == "" {
			k.conf.Consumer.StartOffset = defaultConf.Consumer.StartOffset
		}
		if k.conf.Consumer.MaxConcurrency == 0 {
			k.conf.Consumer.MaxConcurrency = defaultConf.Consumer.MaxConcurrency
		}
		if k.conf.Consumer.MinBatchSize == 0 {
			k.conf.Consumer.MinBatchSize = defaultConf.Consumer.MinBatchSize
		}
		if k.conf.Consumer.MaxBatchSize == 0 {
			k.conf.Consumer.MaxBatchSize = defaultConf.Consumer.MaxBatchSize
		}
		if k.conf.Consumer.MaxWaitTime == nil {
			k.conf.Consumer.MaxWaitTime = defaultConf.Consumer.MaxWaitTime
		}
		if k.conf.Consumer.RebalanceTimeout == nil {
			k.conf.Consumer.RebalanceTimeout = defaultConf.Consumer.RebalanceTimeout
		}
	}
}
