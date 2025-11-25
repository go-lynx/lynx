package snowflake

import (
	"fmt"
	"time"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// ValidateSnowflakeConfig validates the complete snowflake configuration
func ValidateSnowflakeConfig(config *pb.Snowflake) error {
	if config == nil {
		return fmt.Errorf("snowflake configuration cannot be nil")
	}

	// Validate basic configuration
	if err := validateBasicConfig(config); err != nil {
		return fmt.Errorf("basic configuration validation failed: %w", err)
	}

	// Validate Redis integration configuration
	if err := validateRedisIntegrationConfig(config); err != nil {
		return fmt.Errorf("Redis integration validation failed: %w", err)
	}

	// Validate clock drift protection configuration
	if err := validateClockDriftConfig(config); err != nil {
		return fmt.Errorf("clock drift protection validation failed: %w", err)
	}

	// Validate performance configuration
	if err := validatePerformanceConfig(config); err != nil {
		return fmt.Errorf("performance configuration validation failed: %w", err)
	}

	// Validate advanced configuration
	if err := validateAdvancedConfig(config); err != nil {
		return fmt.Errorf("advanced configuration validation failed: %w", err)
	}

	// Cross-validation between different configuration sections
	if err := validateConfigConsistency(config); err != nil {
		return fmt.Errorf("configuration consistency validation failed: %w", err)
	}

	return nil
}

// validateBasicConfig validates basic snowflake configuration
func validateBasicConfig(config *pb.Snowflake) error {
	// Validate datacenter ID
	if config.DatacenterId < 0 || config.DatacenterId > 31 {
		return fmt.Errorf("datacenter ID must be between 0 and 31, got %d", config.DatacenterId)
	}

	// Validate worker ID if not using auto-registration
	if !config.AutoRegisterWorkerId {
		maxWorkerID := int32((1 << 10) - 1) // Default 10 bits
		if config.WorkerIdBits > 0 {
			maxWorkerID = int32((1 << config.WorkerIdBits) - 1)
		}

		if config.WorkerId < 0 || config.WorkerId > maxWorkerID {
			return fmt.Errorf("worker ID must be between 0 and %d, got %d", maxWorkerID, config.WorkerId)
		}
	}

	return nil
}

// validateRedisIntegrationConfig validates Redis integration configuration
func validateRedisIntegrationConfig(config *pb.Snowflake) error {
	// If auto-registration is enabled, Redis configuration is required
	if config.AutoRegisterWorkerId {
		if config.RedisPluginName == "" {
			return fmt.Errorf("Redis plugin name is required when auto worker ID registration is enabled")
		}

		if config.RedisKeyPrefix == "" {
			return fmt.Errorf("Redis key prefix is required when auto worker ID registration is enabled")
		}

		// Validate Redis database number
		if config.RedisDb < 0 || config.RedisDb > 15 {
			return fmt.Errorf("Redis database number must be between 0 and 15, got %d", config.RedisDb)
		}

		// Validate TTL settings
		if config.WorkerIdTtl != nil {
			ttl := config.WorkerIdTtl.AsDuration()
			if ttl <= 0 {
				return fmt.Errorf("worker ID TTL must be positive")
			}
			if ttl < 10*time.Second {
				return fmt.Errorf("worker ID TTL is too small (<10 seconds): %v", ttl)
			}
			if ttl > 24*time.Hour {
				return fmt.Errorf("worker ID TTL is too large (>24 hours): %v", ttl)
			}
		}

		// Validate heartbeat interval
		if config.HeartbeatInterval != nil {
			interval := config.HeartbeatInterval.AsDuration()
			if interval <= 0 {
				return fmt.Errorf("heartbeat interval must be positive")
			}
			if interval < 1*time.Second {
				return fmt.Errorf("heartbeat interval is too small (<1 second): %v", interval)
			}
			if interval > 1*time.Hour {
				return fmt.Errorf("heartbeat interval is too large (>1 hour): %v", interval)
			}

			// Validate relationship between TTL and heartbeat
			if config.WorkerIdTtl != nil {
				ttl := config.WorkerIdTtl.AsDuration()
				if interval >= ttl {
					return fmt.Errorf("heartbeat interval (%v) must be less than worker ID TTL (%v)", interval, ttl)
				}
				if ttl < interval*3 {
					return fmt.Errorf("worker ID TTL (%v) should be at least 3x heartbeat interval (%v) for reliability", ttl, interval)
				}
			}
		}
	}

	return nil
}

// validateClockDriftConfig validates clock drift protection configuration
func validateClockDriftConfig(config *pb.Snowflake) error {
	if config.EnableClockDriftProtection {
		// Validate max clock drift
		if config.MaxClockDrift != nil {
			drift := config.MaxClockDrift.AsDuration()
			if drift <= 0 {
				return fmt.Errorf("max clock drift must be positive when clock drift protection is enabled")
			}
			if drift < 100*time.Millisecond {
				return fmt.Errorf("max clock drift is too small (<100ms): %v", drift)
			}
			if drift > 1*time.Hour {
				return fmt.Errorf("max clock drift is too large (>1 hour): %v", drift)
			}
		}

		// Validate clock check interval
		if config.ClockCheckInterval != nil {
			interval := config.ClockCheckInterval.AsDuration()
			if interval <= 0 {
				return fmt.Errorf("clock check interval must be positive")
			}
			if interval < 100*time.Millisecond {
				return fmt.Errorf("clock check interval is too small (<100ms): %v", interval)
			}
			if interval > 10*time.Minute {
				return fmt.Errorf("clock check interval is too large (>10 minutes): %v", interval)
			}
		}

		// Validate clock drift action
		if config.ClockDriftAction != "" {
			switch config.ClockDriftAction {
			case ClockDriftActionWait, ClockDriftActionError, ClockDriftActionIgnore:
				// Valid actions
			default:
				return fmt.Errorf("invalid clock drift action: %s (valid: wait, error, ignore)", config.ClockDriftAction)
			}
		}
	}

	return nil
}

// validatePerformanceConfig validates performance configuration
func validatePerformanceConfig(config *pb.Snowflake) error {
	if config.EnableSequenceCache {
		if config.SequenceCacheSize <= 0 {
			return fmt.Errorf("sequence cache size must be positive when cache is enabled")
		}

		if config.SequenceCacheSize < 10 {
			return fmt.Errorf("sequence cache size is too small (<10): %d", config.SequenceCacheSize)
		}

		// Calculate max sequence based on sequence bits
		sequenceBits := int32(12) // Default
		if config.SequenceBits > 0 {
			sequenceBits = config.SequenceBits
		}
		maxSequence := int32(1 << sequenceBits)

		if config.SequenceCacheSize > maxSequence {
			return fmt.Errorf("sequence cache size (%d) cannot exceed max sequence (%d)",
				config.SequenceCacheSize, maxSequence)
		}

		if config.SequenceCacheSize > maxSequence/2 {
			// This could be a warning in production, but for validation we'll allow it
			// Could add logging here in the future
		}
	}

	return nil
}

// validateAdvancedConfig validates advanced configuration
func validateAdvancedConfig(config *pb.Snowflake) error {
	// Validate custom epoch
	if config.CustomEpoch != 0 {
		currentTimestamp := time.Now().UnixMilli()
		if config.CustomEpoch > currentTimestamp {
			return fmt.Errorf("custom epoch cannot be in the future: epoch=%d, current=%d",
				config.CustomEpoch, currentTimestamp)
		}

		// Check if epoch is not too old (more than 50 years ago)
		fiftyYearsAgo := time.Now().AddDate(-50, 0, 0).UnixMilli()
		if config.CustomEpoch < fiftyYearsAgo {
			return fmt.Errorf("custom epoch is too old (more than 50 years ago): epoch=%d, limit=%d",
				config.CustomEpoch, fiftyYearsAgo)
		}

		// Check if epoch allows for reasonable future timestamps
		maxFutureTime := config.CustomEpoch + (1<<41 - 1)
		if maxFutureTime < time.Now().AddDate(10, 0, 0).UnixMilli() {
			return fmt.Errorf("custom epoch doesn't allow for sufficient future timestamps: max_future=%d",
				maxFutureTime)
		}
	}

	// Validate bit allocation
	if err := validateBitAllocation(config); err != nil {
		return err
	}

	return nil
}

// validateBitAllocation validates bit allocation configuration
func validateBitAllocation(config *pb.Snowflake) error {
	// Use defaults if not specified
	datacenterBits := int32(5) // Default
	workerBits := config.WorkerIdBits
	if workerBits == 0 {
		workerBits = 10 // Default
	}
	sequenceBits := config.SequenceBits
	if sequenceBits == 0 {
		sequenceBits = 12 // Default
	}

	// Validate individual bit ranges
	if datacenterBits < 0 || datacenterBits > 10 {
		return fmt.Errorf("datacenter ID bits must be between 0 and 10, got %d", datacenterBits)
	}

	if workerBits < 1 || workerBits > 20 {
		return fmt.Errorf("worker ID bits must be between 1 and 20, got %d", workerBits)
	}

	if sequenceBits < 1 || sequenceBits > 20 {
		return fmt.Errorf("sequence bits must be between 1 and 20, got %d", sequenceBits)
	}

	// Validate total bit allocation
	totalBits := datacenterBits + workerBits + sequenceBits
	if totalBits > 22 { // 64 - 41 (timestamp) - 1 (sign) = 22
		return fmt.Errorf("total bits for datacenter, worker, and sequence cannot exceed 22, got %d", totalBits)
	}

	// Validate efficiency
	maxDatacenters := 1 << datacenterBits
	maxWorkers := 1 << workerBits
	maxSequence := 1 << sequenceBits

	if datacenterBits > 0 && maxDatacenters > 1024 {
		return fmt.Errorf("datacenter ID bits allocation is excessive (>1024 datacenters): %d bits = %d max",
			datacenterBits, maxDatacenters)
	}

	if maxWorkers > 65536 {
		return fmt.Errorf("worker ID bits allocation is excessive (>65536 workers): %d bits = %d max",
			workerBits, maxWorkers)
	}

	if maxSequence < 100 {
		return fmt.Errorf("sequence bits allocation is too small (<100 sequences per ms): %d bits = %d max",
			sequenceBits, maxSequence)
	}

	return nil
}

// validateConfigConsistency validates consistency between different configuration sections
func validateConfigConsistency(config *pb.Snowflake) error {
	// If metrics are enabled but sequence cache is disabled, warn about potential performance impact
	if config.EnableMetrics && !config.EnableSequenceCache {
		// This is more of a warning than an error, but we could log it
		// For now, we'll allow this configuration
	}

	// Note: WorkerId=0 is a valid value (first available worker ID)
	// We only need to ensure that when auto-registration is disabled,
	// the worker ID is explicitly configured (which it always is, even if 0)
	// The validation in validateBasicConfig already ensures worker ID is within valid range

	// If clock drift protection is enabled but no action is specified, use default
	if config.EnableClockDriftProtection && config.ClockDriftAction == "" {
		// This will be handled by setting defaults, not an error
	}

	return nil
}
