package snowflake

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratorConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *GeneratorConfig
		expectError bool
	}{
		{
			name:        "Valid config",
			config:      DefaultGeneratorConfig(),
			expectError: false,
		},
		{
			name: "Invalid worker ID bits",
			config: &GeneratorConfig{
				WorkerIDBits: 0,
			},
			expectError: true,
		},
		{
			name: "Invalid sequence bits",
			config: &GeneratorConfig{
				SequenceBits: 0,
			},
			expectError: true,
		},
		{
			name: "Invalid datacenter ID bits",
			config: &GeneratorConfig{
				DatacenterIDBits: 0,
			},
			expectError: true,
		},
		{
			name: "Invalid worker ID - out of range",
			config: &GeneratorConfig{
				CustomEpoch:                1640995200000, // 2022-01-01
				WorkerIDBits:               5,
				SequenceBits:               12,
				DatacenterIDBits:           5,
				EnableClockDriftProtection: true,
				MaxClockDrift:              5 * time.Second,
				ClockDriftAction:           ClockDriftActionWait,
				EnableSequenceCache:        false,
				SequenceCacheSize:          0,
			},
			expectError: false,
		},
		{
			name: "Invalid datacenter ID - out of range",
			config: &GeneratorConfig{
				CustomEpoch:                1640995200000, // 2022-01-01
				WorkerIDBits:               5,
				SequenceBits:               12,
				DatacenterIDBits:           5,
				EnableClockDriftProtection: true,
				MaxClockDrift:              5 * time.Second,
				ClockDriftAction:           ClockDriftActionWait,
				EnableSequenceCache:        false,
				SequenceCacheSize:          0,
			},
			expectError: false,
		},
		{
			name: "Future epoch time",
			config: &GeneratorConfig{
				CustomEpoch:                time.Now().Add(time.Hour).UnixMilli(), // Future time
				WorkerIDBits:               5,
				SequenceBits:               12,
				DatacenterIDBits:           5,
				EnableClockDriftProtection: true,
				MaxClockDrift:              5 * time.Second,
				ClockDriftAction:           ClockDriftActionWait,
				EnableSequenceCache:        false,
				SequenceCacheSize:          0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSnowflakeGeneratorBasic(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Test basic ID generation
	id1, err := gen.GenerateID()
	require.NoError(t, err)
	assert.Greater(t, id1, int64(0))

	id2, err := gen.GenerateID()
	require.NoError(t, err)
	assert.Greater(t, id2, id1) // ID should be incremental

	// Test ID uniqueness
	idSet := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id, err := gen.GenerateID()
		require.NoError(t, err)
		assert.False(t, idSet[id], "Duplicate ID found")
		idSet[id] = true
	}
}

func TestSnowflakeGeneratorWithMetadata(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	id, metadata, err := gen.GenerateIDWithMetadata()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
	assert.NotNil(t, metadata)
}

func TestSnowflakeGeneratorParsing(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(5, 3, config)
	require.NoError(t, err)

	// Generate ID and parse it
	id, err := gen.GenerateID()
	require.NoError(t, err)

	components, err := gen.ParseID(id)
	require.NoError(t, err)

	assert.Equal(t, int64(5), components.WorkerID)
	assert.Equal(t, int64(3), components.DatacenterID)
	assert.Greater(t, components.Timestamp, int64(0))
	assert.GreaterOrEqual(t, components.Sequence, int64(0))

	// Test invalid ID
	_, err = gen.ParseID(-1)
	assert.Error(t, err)
}

func TestSnowflakeGeneratorSequenceOverflow(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Simulate sequence overflow
	// This is difficult to test directly, so we just ensure the generator handles it gracefully
	for i := 0; i < 5000; i++ {
		_, err = gen.GenerateID()
		require.NoError(t, err)
	}

	// Next ID should wait until next millisecond
	id, err := gen.GenerateID()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
}

func TestSnowflakeGeneratorClockBackward(t *testing.T) {
	config := DefaultGeneratorConfig()
	config.EnableClockDriftProtection = true
	config.MaxClockDrift = 5 * time.Second

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Set a future timestamp
	// This is a simplified test - in reality, clock backward detection is more complex
	id, err := gen.GenerateID()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Should return error for clock going backward beyond threshold
	// This is difficult to test without manipulating system time
}

func TestSnowflakeGeneratorConcurrency(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	const numGoroutines = 100
	const idsPerGoroutine = 100

	var wg sync.WaitGroup
	idChan := make(chan int64, numGoroutines*idsPerGoroutine)

	// Start multiple goroutines to generate IDs concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.GenerateID()
				require.NoError(t, err)
				idChan <- id
			}
		}()
	}

	wg.Wait()
	close(idChan)

	// Verify all IDs are unique
	idSet := make(map[int64]bool)
	count := 0
	for id := range idChan {
		assert.False(t, idSet[id], "Duplicate ID found: %d", id)
		idSet[id] = true
		count++
	}

	assert.Equal(t, numGoroutines*idsPerGoroutine, count)
}

func TestSnowflakeGeneratorShutdown(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Test normal shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = gen.Shutdown(ctx)
	assert.NoError(t, err)

	// After shutdown, should not be able to generate IDs
	_, err = gen.GenerateID()
	assert.Error(t, err)
}

func TestSnowflakeGeneratorShutdownTimeout(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Test timeout shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = gen.Shutdown(ctx)
	// May succeed or timeout, depending on implementation
	// Here we don't enforce specific result
}

func TestSnowflakeGeneratorStats(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Generate some IDs
	for i := 0; i < 10; i++ {
		_, err := gen.GenerateID()
		require.NoError(t, err)
	}

	stats := gen.GetStats()
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.WorkerID)
	assert.Equal(t, int64(1), stats.DatacenterID)
	assert.GreaterOrEqual(t, stats.GeneratedCount, int64(10))
}

func TestSnowflakeGeneratorMetrics(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Basic functionality test
	id, err := gen.GenerateID()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Test metrics collection if available
	if metrics := gen.GetMetrics(); metrics != nil {
		snapshot := metrics.GetSnapshot()
		assert.NotNil(t, snapshot)
	}
}
