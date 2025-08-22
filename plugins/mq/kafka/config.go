package kafka

import (
	"fmt"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// validateConfiguration validates the configuration
func (k *Client) validateConfiguration() error {
	if len(k.conf.Brokers) == 0 {
		return ErrNoBrokersConfigured
	}

	// Validate producer configurations (multiple instances)
	for _, p := range k.conf.Producers {
		if p != nil && p.Enabled {
			if err := k.validateProducerConfig(p); err != nil {
				return fmt.Errorf("producer config validation failed: %w", err)
			}
		}
	}

	// Validate consumer configurations (multiple instances)
	for _, c := range k.conf.Consumers {
		if c != nil && c.Enabled {
			if err := k.validateConsumerConfig(c); err != nil {
				return fmt.Errorf("consumer config validation failed: %w", err)
			}
		}
	}

	// Validate SASL configuration
	if k.conf.Sasl != nil && k.conf.Sasl.Enabled {
		if err := k.validateSASLConfig(); err != nil {
			return fmt.Errorf("SASL config validation failed: %w", err)
		}
	}

	// Validate TLS configuration
	if k.conf.Tls != nil && k.conf.Tls.Enabled {
		if err := k.validateTLSConfig(); err != nil {
			return fmt.Errorf("TLS config validation failed: %w", err)
		}
	}

	return nil
}

// validateProducerConfig validates producer configuration
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
	// Validate RequiredAcks value range: allowed -1, 0, 1
	if p.RequiredAcks != -1 && p.RequiredAcks != 0 && p.RequiredAcks != 1 {
		return fmt.Errorf("invalid required_acks: %d (allowed: -1,0,1)", p.RequiredAcks)
	}
	return nil
}

// validateConsumerConfig validates consumer configuration
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

// validateSASLConfig validates SASL configuration
func (k *Client) validateSASLConfig() error {
	if k.conf.Sasl == nil {
		return fmt.Errorf("SASL configuration is nil")
	}

	if !k.conf.Sasl.Enabled {
		return nil // SASL not enabled, no validation needed
	}

	// Validate mechanism type
	validMechanisms := map[string]bool{
		SASLPlain:       true,
		SASLScramSHA256: true,
		SASLScramSHA512: true,
	}

	if !validMechanisms[k.conf.Sasl.Mechanism] {
		return fmt.Errorf("%w: %s", ErrInvalidSASLMechanism, k.conf.Sasl.Mechanism)
	}

	// Validate username and password
	if k.conf.Sasl.Username == "" {
		return fmt.Errorf("SASL username is required when SASL is enabled")
	}
	if k.conf.Sasl.Password == "" {
		return fmt.Errorf("SASL password is required when SASL is enabled")
	}

	return nil
}

// validateTLSConfig validates TLS configuration
func (k *Client) validateTLSConfig() error {
	if k.conf.Tls == nil {
		return fmt.Errorf("TLS configuration is nil")
	}

	if !k.conf.Tls.Enabled {
		return nil // TLS not enabled, no validation needed
	}

	// Validate certificate and key
	if k.conf.Tls.CertFile == "" {
		return fmt.Errorf("TLS certificate is required when TLS is enabled")
	}
	if k.conf.Tls.KeyFile == "" {
		return fmt.Errorf("TLS key is required when TLS is enabled")
	}

	return nil
}

// setDefaultValues sets default values
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
				RequiredAcks: 1, // Default to leader ack to avoid unsafe no-ack when defaulting to 0
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

	// Apply default values
	if k.conf.DialTimeout == nil {
		k.conf.DialTimeout = defaultConf.DialTimeout
	}
	// Multiple producer defaults
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
		// required_acks: don't override 0
	}
	// Multiple consumer defaults
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
