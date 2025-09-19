package snowflake

import (
	"context"
	"fmt"
	"time"
	
	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// NewSnowflakeGeneratorWithConfig creates a new snowflake ID generator with protobuf config
func NewSnowflakeGeneratorWithConfig(config *pb.Snowflake) (*SnowflakeGenerator, error) {
	// Convert protobuf config to internal config
	internalConfig := &GeneratorConfig{
		CustomEpoch:                config.CustomEpoch,
		DatacenterIDBits:          5, // Fixed to 5 bits for datacenter ID (0-31)
		WorkerIDBits:              int(config.WorkerIdBits),
		SequenceBits:              int(config.SequenceBits),
		EnableClockDriftProtection: config.EnableClockDriftProtection,
		MaxClockDrift:             config.MaxClockDrift.AsDuration(),
		ClockDriftAction:          ClockDriftActionWait,
		EnableSequenceCache:       config.EnableSequenceCache,
		SequenceCacheSize:         int(config.SequenceCacheSize),
	}
	
	return NewSnowflakeGeneratorCore(int64(config.DatacenterId), int64(config.WorkerId), internalConfig)
}

// NewSnowflakeGenerator creates a new snowflake generator
func NewSnowflakeGeneratorCore(datacenterID, workerID int64, config *GeneratorConfig) (*SnowflakeGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid generator config: %w", err)
	}

	// Validate datacenter ID and worker ID
	maxDatacenterID := (1 << config.DatacenterIDBits) - 1
	maxWorkerID := (1 << config.WorkerIDBits) - 1

	if datacenterID < 0 || datacenterID > int64(maxDatacenterID) {
		return nil, fmt.Errorf("datacenter ID must be between 0 and %d", maxDatacenterID)
	}

	if workerID < 0 || workerID > int64(maxWorkerID) {
		return nil, fmt.Errorf("worker ID must be between 0 and %d", maxWorkerID)
	}

	// Validate custom epoch is not in the future
	currentTimestamp := time.Now().UnixMilli()
	if config.CustomEpoch > currentTimestamp {
		return nil, fmt.Errorf("custom epoch cannot be in the future")
	}

	// Calculate bit shifts
	timestampShift := config.DatacenterIDBits + config.WorkerIDBits + config.SequenceBits
	datacenterShift := config.WorkerIDBits + config.SequenceBits
	workerShift := config.SequenceBits

	generator := &SnowflakeGenerator{
		datacenterID:    datacenterID,
		workerID:        workerID,
		customEpoch:     config.CustomEpoch,
		workerIDBits:    int64(config.WorkerIDBits),
		sequenceBits:    int64(config.SequenceBits),
		timestampShift:  int64(timestampShift),
		datacenterShift: int64(datacenterShift),
		workerShift:     int64(workerShift),
		maxDatacenterID: int64(maxDatacenterID),
		maxWorkerID:     int64(maxWorkerID),
		maxSequence:     (1 << config.SequenceBits) - 1,
		lastTimestamp:   -1,
		sequence:        0,
		enableClockDriftProtection: config.EnableClockDriftProtection,
		maxClockDrift:              config.MaxClockDrift,
		clockDriftAction:           config.ClockDriftAction,
		enableSequenceCache:        config.EnableSequenceCache,
		cacheSize:                  config.SequenceCacheSize,
	}

	// Initialize sequence cache if enabled
	if generator.enableSequenceCache {
		generator.sequenceCache = make([]int64, generator.cacheSize)
		generator.cacheIndex = 0
	}

	return generator, nil
}

// GenerateID generates a new snowflake ID
func (g *SnowflakeGenerator) GenerateID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if generator is shutting down
	if g.isShuttingDown {
		return 0, fmt.Errorf("generator is shutting down")
	}

	timestamp := g.getCurrentTimestamp()

	// Check for clock drift
	if g.enableClockDriftProtection {
		if err := g.checkClockDrift(timestamp); err != nil {
			return 0, err
		}
	}

	// Handle clock going backwards
	if timestamp < g.lastTimestamp {
		return 0, g.handleClockBackward(timestamp)
	}

	// If same millisecond, increment sequence
	if timestamp == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & g.maxSequence
		if g.sequence == 0 {
			// Sequence overflow, wait for next millisecond
			timestamp = g.waitForNextMillisecond(timestamp)
		}
	} else {
		// New millisecond, reset sequence
		g.sequence = 0
	}

	g.lastTimestamp = timestamp

	// Generate the ID
	id := ((timestamp - g.customEpoch) << g.timestampShift) |
		(g.datacenterID << g.datacenterShift) |
		(g.workerID << g.workerShift) |
		g.sequence

	// Update statistics
	g.generatedCount++

	return id, nil
}

// GenerateIDWithMetadata generates a snowflake ID with metadata
func (g *SnowflakeGenerator) GenerateIDWithMetadata() (int64, *SnowflakeID, error) {
	id, err := g.GenerateID()
	if err != nil {
		return 0, nil, err
	}

	metadata, err := g.ParseID(id)
	if err != nil {
		return 0, nil, err
	}

	return id, metadata, nil
}

// GetStats returns statistics about the generator
func (g *SnowflakeGenerator) GetStats() *GeneratorStats {
	g.mu.Lock()
	defer g.mu.Unlock()

	return &GeneratorStats{
		WorkerID:           g.workerID,
		DatacenterID:       g.datacenterID,
		GeneratedCount:     g.generatedCount,
		ClockBackwardCount: g.clockBackwardCount,
		LastGeneratedTime:  g.lastTimestamp,
	}
}

// IsHealthy returns whether the generator is healthy
func (g *SnowflakeGenerator) IsHealthy() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if generator is shutting down
	if g.isShuttingDown {
		return false
	}

	// Check for recent clock drift issues
	if g.enableClockDriftProtection {
		now := time.Now()
		if now.Sub(g.lastClockCheck) < time.Minute {
			// If we've checked recently and no errors, we're healthy
			return true
		}
	}

	return true
}

// Shutdown gracefully shuts down the generator
func (g *SnowflakeGenerator) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.isShuttingDown {
		return nil // Already shutting down
	}

	g.isShuttingDown = true
	return nil
}

// ParseID parses a snowflake ID and returns its components
func (g *SnowflakeGenerator) ParseID(id int64) (*SnowflakeID, error) {
	if id < 0 {
		return nil, fmt.Errorf("invalid snowflake ID: %d", id)
	}

	// Extract components
	sequence := id & g.maxSequence
	workerID := (id >> g.workerShift) & g.maxWorkerID
	datacenterID := (id >> g.datacenterShift) & g.maxDatacenterID
	timestamp := (id >> g.timestampShift) + g.customEpoch

	return &SnowflakeID{
		ID:           id,
		Timestamp:    time.Unix(timestamp/1000, (timestamp%1000)*1000000),
		DatacenterID: datacenterID,
		WorkerID:     workerID,
		Sequence:     sequence,
	}, nil
}

// getCurrentTimestamp returns current timestamp in milliseconds
func (g *SnowflakeGenerator) getCurrentTimestamp() int64 {
	return time.Now().UnixNano() / 1000000
}

// checkClockDrift checks for clock drift and handles it according to configuration
func (g *SnowflakeGenerator) checkClockDrift(currentTimestamp int64) error {
	if g.lastTimestamp == -1 {
		return nil // First call, no drift to check
	}

	now := time.Now()
	if now.Sub(g.lastClockCheck) < time.Second {
		return nil // Skip check if checked recently
	}
	g.lastClockCheck = now

	drift := time.Duration(g.lastTimestamp-currentTimestamp) * time.Millisecond
	if drift > g.maxClockDrift {
		switch g.clockDriftAction {
		case ClockDriftActionError:
			return &ClockDriftError{
				CurrentTime:   time.Unix(currentTimestamp/1000, (currentTimestamp%1000)*1000000),
				LastTimestamp: time.Unix(g.lastTimestamp/1000, (g.lastTimestamp%1000)*1000000),
				Drift:         drift,
			}
		case ClockDriftActionWait:
			// Wait for clock to catch up
			waitTime := drift
			time.Sleep(waitTime)
			return nil
		case ClockDriftActionIgnore:
			// Do nothing, just log
			return nil
		default:
			return fmt.Errorf("unknown clock drift action: %s", g.clockDriftAction)
		}
	}

	return nil
}

// handleClockBackward handles the case when clock goes backward
func (g *SnowflakeGenerator) handleClockBackward(currentTimestamp int64) error {
	drift := time.Duration(g.lastTimestamp-currentTimestamp) * time.Millisecond
	
	// Update statistics
	g.clockBackwardCount++

	switch g.clockDriftAction {
	case ClockDriftActionError:
		return &ClockDriftError{
			CurrentTime:   time.Unix(currentTimestamp/1000, (currentTimestamp%1000)*1000000),
			LastTimestamp: time.Unix(g.lastTimestamp/1000, (g.lastTimestamp%1000)*1000000),
			Drift:         drift,
		}
	case ClockDriftActionWait:
		// Wait for clock to catch up
		waitTime := drift + time.Millisecond // Add 1ms buffer
		time.Sleep(waitTime)
		return nil
	case ClockDriftActionIgnore:
		// Use last timestamp + 1
		g.lastTimestamp++
		return nil
	default:
		return fmt.Errorf("unknown clock drift action: %s", g.clockDriftAction)
	}
}

// waitForNextMillisecond waits until the next millisecond
func (g *SnowflakeGenerator) waitForNextMillisecond(lastTimestamp int64) int64 {
	timestamp := g.getCurrentTimestamp()
	for timestamp <= lastTimestamp {
		time.Sleep(time.Millisecond)
		timestamp = g.getCurrentTimestamp()
	}
	return timestamp
}

// GeneratorConfig holds configuration for the snowflake generator
type GeneratorConfig struct {
	CustomEpoch                int64
	DatacenterIDBits          int
	WorkerIDBits              int
	SequenceBits              int
	EnableClockDriftProtection bool
	MaxClockDrift             time.Duration
	ClockDriftAction          string
	EnableSequenceCache       bool
	SequenceCacheSize         int
}

// DefaultGeneratorConfig returns default generator configuration
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,  // 0-31
		WorkerIDBits:              DefaultWorkerIDBits, // 0-1023
		SequenceBits:              DefaultSequenceBits, // 0-4095
		EnableClockDriftProtection: true,
		MaxClockDrift:             DefaultMaxClockDrift,
		ClockDriftAction:          ClockDriftActionWait,
		EnableSequenceCache:       false,
		SequenceCacheSize:         DefaultSequenceCacheSize,
	}
}

// Validate validates the generator configuration
func (c *GeneratorConfig) Validate() error {
	// Check bit allocation
	totalBits := c.DatacenterIDBits + c.WorkerIDBits + c.SequenceBits
	if totalBits > 22 { // 64 - 41 (timestamp) - 1 (sign) = 22
		return fmt.Errorf("total bits for datacenter, worker, and sequence cannot exceed 22, got %d", totalBits)
	}

	if c.DatacenterIDBits < 0 || c.DatacenterIDBits > 10 {
		return fmt.Errorf("datacenter ID bits must be between 0 and 10, got %d", c.DatacenterIDBits)
	}

	if c.WorkerIDBits < 1 || c.WorkerIDBits > 20 {
		return fmt.Errorf("worker ID bits must be between 1 and 20, got %d", c.WorkerIDBits)
	}

	if c.SequenceBits < 1 || c.SequenceBits > 20 {
		return fmt.Errorf("sequence bits must be between 1 and 20, got %d", c.SequenceBits)
	}

	// Check clock drift action
	switch c.ClockDriftAction {
	case ClockDriftActionWait, ClockDriftActionError, ClockDriftActionIgnore:
		// Valid actions
	default:
		return fmt.Errorf("invalid clock drift action: %s", c.ClockDriftAction)
	}

	// Check cache size
	if c.EnableSequenceCache && c.SequenceCacheSize <= 0 {
		return fmt.Errorf("sequence cache size must be positive when cache is enabled")
	}

	return nil
}