package snowflake

import (
	"time"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// TestConfig provides test configuration utilities
type TestConfig struct {
	DatacenterID int64
	WorkerID     int64
	Config       *pb.Snowflake
}

// NewTestConfig creates a new test configuration
func NewTestConfig(datacenterID, workerID int64) *TestConfig {
	return &TestConfig{
		DatacenterID: datacenterID,
		WorkerID:     workerID,
		Config:       createDefaultTestConfig(datacenterID, workerID),
	}
}

// createDefaultTestConfig creates a default test configuration
func createDefaultTestConfig(datacenterID, workerID int64) *pb.Snowflake {
	return &pb.Snowflake{
		DatacenterId:               int32(datacenterID),
		WorkerId:                   int32(workerID),
		CustomEpoch:                1640995200000, // 2022-01-01 00:00:00 UTC
		WorkerIdBits:               5,             // 5-bit worker node ID (0-31) - combined with 5-bit datacenter ID
		SequenceBits:               12,            // 12-bit sequence number (0-4095)
		EnableClockDriftProtection: false,         // Disable clock drift protection to simplify testing
		ClockDriftAction:           ClockDriftActionWait,
		EnableSequenceCache:        false,
		SequenceCacheSize:          0,
		AutoRegisterWorkerId:       false, // Disable auto-registration to simplify testing
		RedisKeyPrefix:             "test_worker:",
		RedisPluginName:            "default",
		RedisDb:                    0,
	}
}

// WithRedisConfig sets Redis configuration for testing
func (tc *TestConfig) WithRedisConfig(pluginName string, db int32) *TestConfig {
	tc.Config.RedisPluginName = pluginName
	tc.Config.RedisDb = db
	return tc
}

// WithCustomEpoch sets custom epoch for testing
func (tc *TestConfig) WithCustomEpoch(epoch int64) *TestConfig {
	tc.Config.CustomEpoch = epoch
	return tc
}

// WithClockDriftProtection enables/disables clock drift protection
func (tc *TestConfig) WithClockDriftProtection(enabled bool, maxDrift time.Duration, action string) *TestConfig {
	tc.Config.EnableClockDriftProtection = enabled
	tc.Config.ClockDriftAction = action
	return tc
}

// WithSequenceCache enables/disables sequence cache
func (tc *TestConfig) WithSequenceCache(enabled bool, cacheSize int32) *TestConfig {
	tc.Config.EnableSequenceCache = enabled
	tc.Config.SequenceCacheSize = cacheSize
	return tc
}

// WithWorkerConfig sets worker configuration
func (tc *TestConfig) WithWorkerConfig(autoRegister bool) *TestConfig {
	tc.Config.AutoRegisterWorkerId = autoRegister
	return tc
}

// WithKeyPrefixes sets key prefixes for Redis
func (tc *TestConfig) WithKeyPrefixes(redisKeyPrefix string) *TestConfig {
	tc.Config.RedisKeyPrefix = redisKeyPrefix
	return tc
}

// Build returns the final configuration
func (tc *TestConfig) Build() *pb.Snowflake {
	return tc.Config
}

// CreateTestGenerator creates a generator for testing
func (tc *TestConfig) CreateTestGenerator() (*Generator, error) {
	genConfig := &GeneratorConfig{
		CustomEpoch:                tc.Config.CustomEpoch,
		DatacenterIDBits:           5, // Fixed to 5-bit datacenter ID (0-31)
		WorkerIDBits:               int(tc.Config.WorkerIdBits),
		SequenceBits:               int(tc.Config.SequenceBits),
		EnableClockDriftProtection: tc.Config.EnableClockDriftProtection,
		ClockDriftAction:           tc.Config.ClockDriftAction,
		EnableSequenceCache:        tc.Config.EnableSequenceCache,
		SequenceCacheSize:          int(tc.Config.SequenceCacheSize),
	}

	return NewSnowflakeGeneratorCore(tc.DatacenterID, tc.WorkerID, genConfig)
}

// CreateTestPlugin creates a plugin for testing
func (tc *TestConfig) CreateTestPlugin() *PlugSnowflake {
	return NewSnowflakePlugin()
}

// MinimalConfig creates a minimal configuration for testing
func MinimalConfig(datacenterID, workerID int64) *pb.Snowflake {
	return &pb.Snowflake{
		DatacenterId: int32(datacenterID),
		WorkerId:     int32(workerID),
		CustomEpoch:  1640995200000, // 2022-01-01 00:00:00 UTC
		WorkerIdBits: 5,
		SequenceBits: 12,
	}
}

// RedisTestConfig creates a configuration with Redis for testing
func RedisTestConfig(datacenterID, workerID int64, redisPluginName string) *pb.Snowflake {
	config := MinimalConfig(datacenterID, workerID)
	config.AutoRegisterWorkerId = true
	config.RedisPluginName = redisPluginName
	config.RedisDb = 0
	config.RedisKeyPrefix = "test_worker:"

	return config
}

// ClockDriftTestConfig creates a configuration with clock drift protection
func ClockDriftTestConfig(datacenterID, workerID int64) *pb.Snowflake {
	config := MinimalConfig(datacenterID, workerID)
	config.EnableClockDriftProtection = true
	config.ClockDriftAction = ClockDriftActionWait

	return config
}

// SequenceCacheTestConfig creates a configuration with sequence cache
func SequenceCacheTestConfig(datacenterID, workerID int64, cacheSize int32) *pb.Snowflake {
	config := MinimalConfig(datacenterID, workerID)
	config.EnableSequenceCache = true
	config.SequenceCacheSize = cacheSize

	return config
}
