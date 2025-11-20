package snowflake

import (
	"context"
	"fmt"
	"time"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// NewSnowflakeGeneratorWithConfig creates a new snowflake ID generator with protobuf config
func NewSnowflakeGeneratorWithConfig(config *pb.Snowflake) (*Generator, error) {
	// Convert protobuf config to internal config
	internalConfig := &GeneratorConfig{
		CustomEpoch:                config.CustomEpoch,
		DatacenterIDBits:           5, // Fixed to 5 bits for datacenter ID (0-31)
		WorkerIDBits:               int(config.WorkerIdBits),
		SequenceBits:               int(config.SequenceBits),
		EnableClockDriftProtection: config.EnableClockDriftProtection,
		MaxClockDrift:              config.MaxClockDrift.AsDuration(),
		ClockDriftAction:           ClockDriftActionWait,
		EnableSequenceCache:        config.EnableSequenceCache,
		SequenceCacheSize:          int(config.SequenceCacheSize),
	}

	return NewSnowflakeGeneratorCore(int64(config.DatacenterId), int64(config.WorkerId), internalConfig)
}

// NewSnowflakeGeneratorCore  creates a new snowflake generator
func NewSnowflakeGeneratorCore(datacenterID, workerID int64, config *GeneratorConfig) (*Generator, error) {
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

	generator := &Generator{
		datacenterID:               datacenterID,
		workerID:                   workerID,
		customEpoch:                config.CustomEpoch,
		workerIDBits:               int64(config.WorkerIDBits),
		sequenceBits:               int64(config.SequenceBits),
		timestampShift:             int64(timestampShift),
		datacenterShift:            int64(datacenterShift),
		workerShift:                int64(workerShift),
		maxDatacenterID:            int64(maxDatacenterID),
		maxWorkerID:                int64(maxWorkerID),
		maxSequence:                (1 << config.SequenceBits) - 1,
		lastTimestamp:              -1,
		sequence:                   0,
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

	// Initialize metrics
	generator.metrics = NewSnowflakeMetrics()

	return generator, nil
}

// GenerateID generates a new snowflake ID
func (g *Generator) GenerateID() (int64, error) {
	startTime := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if generator is shutting down
	if g.isShuttingDown {
		g.metrics.RecordError("generation")
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
		newTimestamp, err := g.handleClockBackward(timestamp)
		if err != nil {
			return 0, err
		}
		// Update timestamp after handling clock backward
		timestamp = newTimestamp
	}

	// If same millisecond, increment sequence
	if timestamp == g.lastTimestamp {
		if g.enableSequenceCache && g.cacheIndex < len(g.sequenceCache) && g.cacheIndex >= 0 {
			// Use cached sequence if available and valid
			cachedSeq := g.sequenceCache[g.cacheIndex]
			g.cacheIndex++
			// Validate cached sequence is within bounds before using
			if cachedSeq > 0 && cachedSeq <= g.maxSequence {
				g.sequence = cachedSeq
			} else {
				// Invalid cached sequence, fall back to normal increment
				g.sequence = (g.sequence + 1) & g.maxSequence
				if g.sequence == 0 {
					// Sequence overflow, wait for next millisecond
					timestamp = g.waitForNextMillisecond(timestamp)
				}
			}
		} else {
			// Cache exhausted or disabled, use normal increment
			g.sequence = (g.sequence + 1) & g.maxSequence
			if g.sequence == 0 {
				// Sequence overflow, wait for next millisecond
				timestamp = g.waitForNextMillisecond(timestamp)
			}
		}
	} else {
		// New millisecond, reset sequence and refill cache if enabled
		g.sequence = 0
		if g.enableSequenceCache {
			g.refillSequenceCache()
		}
	}

	g.lastTimestamp = timestamp

	// Generate the ID
	id := ((timestamp - g.customEpoch) << g.timestampShift) |
		(g.datacenterID << g.datacenterShift) |
		(g.workerID << g.workerShift) |
		g.sequence

	// Update statistics
	g.generatedCount++

	// Record metrics
	latency := time.Since(startTime)
	cacheHit := g.enableSequenceCache && g.cacheIndex > 0
	g.metrics.RecordIDGeneration(latency, cacheHit)

	return id, nil
}

// GenerateIDWithMetadata generates a snowflake ID with metadata
func (g *Generator) GenerateIDWithMetadata() (int64, *SID, error) {
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
func (g *Generator) GetStats() *GeneratorStats {
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

// GetMetrics returns detailed metrics about the generator
func (g *Generator) GetMetrics() *Metrics {
	if g.metrics == nil {
		return nil
	}
	return g.metrics.GetSnapshot()
}

// IsHealthy returns whether the generator is healthy
func (g *Generator) IsHealthy() bool {
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
func (g *Generator) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.isShuttingDown {
		return nil // Already shutting down
	}

	g.isShuttingDown = true
	return nil
}

// ParseID parses a snowflake ID and returns its components
func (g *Generator) ParseID(id int64) (*SID, error) {
	if id < 0 {
		return nil, fmt.Errorf("invalid snowflake ID: %d", id)
	}

	// Extract components
	sequence := id & g.maxSequence
	workerID := (id >> g.workerShift) & g.maxWorkerID
	datacenterID := (id >> g.datacenterShift) & g.maxDatacenterID
	timestamp := (id >> g.timestampShift) + g.customEpoch

	return &SID{
		ID:           id,
		Timestamp:    time.Unix(timestamp/1000, (timestamp%1000)*1000000),
		DatacenterID: datacenterID,
		WorkerID:     workerID,
		Sequence:     sequence,
	}, nil
}

// getCurrentTimestamp returns current timestamp in milliseconds
func (g *Generator) getCurrentTimestamp() int64 {
	return time.Now().UnixNano() / 1000000
}

// checkClockDrift checks for clock drift and handles it according to configuration
func (g *Generator) checkClockDrift(currentTimestamp int64) error {
	if g.lastTimestamp == -1 {
		return nil // First call, no drift to check
	}

	now := time.Now()
	if now.Sub(g.lastClockCheck) < time.Second {
		return nil // Skip check if checked recently
	}
	g.lastClockCheck = now

	// Calculate drift correctly: positive means clock moved forward too fast
	// Negative means clock went backward (handled separately in handleClockBackward)
	driftMs := currentTimestamp - g.lastTimestamp
	if driftMs < 0 {
		// Clock went backward, this is handled in handleClockBackward
		return nil
	}

	drift := time.Duration(driftMs) * time.Millisecond
	if drift > g.maxClockDrift {
		switch g.clockDriftAction {
		case ClockDriftActionError:
			return &ClockDriftError{
				CurrentTime:   time.Unix(currentTimestamp/1000, (currentTimestamp%1000)*1000000),
				LastTimestamp: time.Unix(g.lastTimestamp/1000, (g.lastTimestamp%1000)*1000000),
				Drift:         drift,
			}
		case ClockDriftActionWait:
			// Wait for clock to stabilize (not wait for drift, but wait a bit)
			time.Sleep(10 * time.Millisecond)
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
// Returns the new timestamp to use and any error
func (g *Generator) handleClockBackward(currentTimestamp int64) (int64, error) {
	driftMs := g.lastTimestamp - currentTimestamp
	drift := time.Duration(driftMs) * time.Millisecond

	// Update statistics
	g.clockBackwardCount++

	switch g.clockDriftAction {
	case ClockDriftActionError:
		return 0, &ClockDriftError{
			CurrentTime:   time.Unix(currentTimestamp/1000, (currentTimestamp%1000)*1000000),
			LastTimestamp: time.Unix(g.lastTimestamp/1000, (g.lastTimestamp%1000)*1000000),
			Drift:         drift,
		}
	case ClockDriftActionWait:
		// Wait for clock to catch up, with maximum wait time limit
		maxWaitTime := 5 * time.Second
		waitTime := drift + time.Millisecond // Add 1ms buffer
		if waitTime > maxWaitTime {
			waitTime = maxWaitTime
		}
		time.Sleep(waitTime)
		// Re-check timestamp after wait
		newTimestamp := g.getCurrentTimestamp()
		if newTimestamp < g.lastTimestamp {
			// Still backward after wait, use last timestamp + 1 to avoid blocking
			// Reset sequence when forcing timestamp increment
			g.sequence = 0
			return g.lastTimestamp + 1, nil
		}
		// Clock caught up, use the new timestamp
		return newTimestamp, nil
	case ClockDriftActionIgnore:
		// Use last timestamp + 1 to ensure monotonicity
		// Reset sequence to avoid potential ID collisions
		g.sequence = 0
		return g.lastTimestamp + 1, nil
	default:
		return 0, fmt.Errorf("unknown clock drift action: %s", g.clockDriftAction)
	}
}

// waitForNextMillisecond waits until the next millisecond with improved precision
func (g *Generator) waitForNextMillisecond(lastTimestamp int64) int64 {
	timestamp := g.getCurrentTimestamp()
	maxWaitIterations := 100 // Prevent infinite loop
	iterations := 0
	
	for timestamp <= lastTimestamp && iterations < maxWaitIterations {
		// Calculate remaining time more precisely
		remainingNs := (lastTimestamp+1)*1000000 - time.Now().UnixNano()
		if remainingNs > 0 {
			// Sleep for the remaining time, but not more than 1ms
			if remainingNs > 1000000 {
				remainingNs = 1000000
			}
			time.Sleep(time.Duration(remainingNs))
		} else {
			// Small sleep to avoid busy waiting
			time.Sleep(100 * time.Microsecond)
		}
		timestamp = g.getCurrentTimestamp()
		iterations++
	}
	
	// If still not advanced after max iterations, force increment
	if timestamp <= lastTimestamp {
		timestamp = lastTimestamp + 1
	}
	
	return timestamp
}

// refillSequenceCache pre-generates sequence numbers for better performance
func (g *Generator) refillSequenceCache() {
	if !g.enableSequenceCache || len(g.sequenceCache) == 0 {
		return
	}

	// Calculate how many valid sequences we can cache
	// Start from sequence 1 (since 0 is already used)
	validCount := 0
	maxValidCount := int(g.maxSequence) // Maximum sequences per millisecond
	
	// Limit cache size to available sequence space
	cacheSize := len(g.sequenceCache)
	if cacheSize > maxValidCount {
		cacheSize = maxValidCount
	}

	// Fill cache with sequential numbers starting from 1
	// Note: sequence is already 0 at this point (new millisecond)
	// Limit to actual available sequences
	actualCacheSize := cacheSize
	if actualCacheSize > maxValidCount {
		actualCacheSize = maxValidCount
	}
	
	for i := 0; i < actualCacheSize; i++ {
		seq := int64(i + 1) // Start from 1, not 0
		if seq > g.maxSequence {
			// Should not happen if cacheSize is validated, but check anyway
			break
		}
		g.sequenceCache[i] = seq
		validCount = i + 1
	}

	// Note: We keep the original slice size, but only use validCount entries
	// The cacheIndex check in GenerateID ensures we don't access beyond validCount

	// Reset cache index
	g.cacheIndex = 0

	// Record cache refill metrics
	if g.metrics != nil {
		g.metrics.RecordCacheRefill()
	}
}

// GeneratorConfig holds configuration for the snowflake generator
type GeneratorConfig struct {
	CustomEpoch                int64
	DatacenterIDBits           int
	WorkerIDBits               int
	SequenceBits               int
	EnableClockDriftProtection bool
	MaxClockDrift              time.Duration
	ClockDriftAction           string
	EnableSequenceCache        bool
	SequenceCacheSize          int
}

// DefaultGeneratorConfig returns default generator configuration
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:           5,  // 0-31
		WorkerIDBits:               5,  // 0-31 (reduced from 10 to fit 22-bit limit)
		SequenceBits:               12, // 0-4095
		EnableClockDriftProtection: true,
		MaxClockDrift:              DefaultMaxClockDrift,
		ClockDriftAction:           ClockDriftActionWait,
		EnableSequenceCache:        false,
		SequenceCacheSize:          DefaultSequenceCacheSize,
	}
}

// Validate validates the generator configuration with enhanced checks
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

	// Enhanced epoch validation
	if err := c.validateEpoch(); err != nil {
		return err
	}

	// Enhanced clock drift validation
	if err := c.validateClockDrift(); err != nil {
		return err
	}

	// Enhanced cache validation
	if err := c.validateCache(); err != nil {
		return err
	}

	// Validate bit allocation efficiency
	if err := c.validateBitAllocationEfficiency(); err != nil {
		return err
	}

	return nil
}

// validateEpoch validates the custom epoch configuration
func (c *GeneratorConfig) validateEpoch() error {
	// Check if epoch is not in the future
	currentTimestamp := time.Now().UnixMilli()
	if c.CustomEpoch > currentTimestamp {
		return fmt.Errorf("custom epoch cannot be in the future: epoch=%d, current=%d",
			c.CustomEpoch, currentTimestamp)
	}

	// Check if epoch is not too old (more than 50 years ago)
	fiftyYearsAgo := time.Now().AddDate(-50, 0, 0).UnixMilli()
	if c.CustomEpoch < fiftyYearsAgo {
		return fmt.Errorf("custom epoch is too old (more than 50 years ago): epoch=%d, limit=%d",
			c.CustomEpoch, fiftyYearsAgo)
	}

	// Check if epoch allows for reasonable future timestamps
	// With 41 bits for timestamp, we can represent ~69 years from epoch
	maxFutureTime := c.CustomEpoch + (1<<41 - 1)
	if maxFutureTime < time.Now().AddDate(10, 0, 0).UnixMilli() {
		return fmt.Errorf("custom epoch doesn't allow for sufficient future timestamps: max_future=%d",
			maxFutureTime)
	}

	return nil
}

// validateClockDrift validates clock drift protection settings
func (c *GeneratorConfig) validateClockDrift() error {
	// Check clock drift action
	switch c.ClockDriftAction {
	case ClockDriftActionWait, ClockDriftActionError, ClockDriftActionIgnore:
		// Valid actions
	default:
		return fmt.Errorf("invalid clock drift action: %s", c.ClockDriftAction)
	}

	// Validate max clock drift duration
	if c.EnableClockDriftProtection {
		if c.MaxClockDrift <= 0 {
			return fmt.Errorf("max clock drift must be positive when clock drift protection is enabled")
		}

		if c.MaxClockDrift > 1*time.Hour {
			return fmt.Errorf("max clock drift is too large (>1 hour): %v", c.MaxClockDrift)
		}

		if c.MaxClockDrift < 100*time.Millisecond {
			return fmt.Errorf("max clock drift is too small (<100ms): %v", c.MaxClockDrift)
		}
	}

	return nil
}

// validateCache validates sequence cache settings
func (c *GeneratorConfig) validateCache() error {
	if c.EnableSequenceCache {
		if c.SequenceCacheSize <= 0 {
			return fmt.Errorf("sequence cache size must be positive when cache is enabled")
		}

		// Check cache size limits
		maxSequence := 1 << c.SequenceBits
		if c.SequenceCacheSize > maxSequence {
			return fmt.Errorf("sequence cache size (%d) cannot exceed max sequence (%d)",
				c.SequenceCacheSize, maxSequence)
		}

		// Warn if cache size is too large (more than 50% of sequence space)
		if c.SequenceCacheSize > maxSequence/2 {
			// This is a warning, not an error, but we could log it
			// For now, we'll allow it but could add logging here
		}

		// Check minimum cache size for efficiency
		if c.SequenceCacheSize < 10 {
			return fmt.Errorf("sequence cache size is too small (<10): %d", c.SequenceCacheSize)
		}
	}

	return nil
}

// validateBitAllocationEfficiency validates the efficiency of bit allocation
func (c *GeneratorConfig) validateBitAllocationEfficiency() error {
	// Calculate maximum values for each component
	maxDatacenters := 1 << c.DatacenterIDBits
	maxWorkers := 1 << c.WorkerIDBits
	maxSequence := 1 << c.SequenceBits

	// Check for reasonable bit allocation
	if c.DatacenterIDBits > 0 && maxDatacenters > 1024 {
		return fmt.Errorf("datacenter ID bits allocation is excessive (>1024 datacenters): %d bits = %d max",
			c.DatacenterIDBits, maxDatacenters)
	}

	if maxWorkers > 65536 {
		return fmt.Errorf("worker ID bits allocation is excessive (>65536 workers): %d bits = %d max",
			c.WorkerIDBits, maxWorkers)
	}

	if maxSequence < 100 {
		return fmt.Errorf("sequence bits allocation is too small (<100 sequences per ms): %d bits = %d max",
			c.SequenceBits, maxSequence)
	}

	// Check for balanced allocation
	totalBits := c.DatacenterIDBits + c.WorkerIDBits + c.SequenceBits
	if c.SequenceBits < totalBits/3 {
		return fmt.Errorf("sequence bits allocation is disproportionately small: %d/%d total bits",
			c.SequenceBits, totalBits)
	}

	return nil
}
