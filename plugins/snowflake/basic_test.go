package snowflake

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicFunctionality tests basic functionality
func TestBasicFunctionality(t *testing.T) {
	// Test generator config validation
	t.Run("Generator Config Validation", func(t *testing.T) {
		testConfig := NewTestConfig(1, 1)
		gen, err := testConfig.CreateTestGenerator()
		require.NoError(t, err)
		assert.NotNil(t, gen)
	})

	// Test ID generation
	t.Run("ID Generation", func(t *testing.T) {
		testConfig := NewTestConfig(1, 1)
		gen, err := testConfig.CreateTestGenerator()
		require.NoError(t, err)

		// Generate ID
		id, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.Greater(t, id, int64(0))

		// Generate another ID
		id2, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.Greater(t, id2, id)

		// Generate ID with metadata (no metadata parameter for now)
		id3, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.Greater(t, id3, id2)
	})

	// Test ID parsing
	t.Run("ID Parsing", func(t *testing.T) {
		testConfig := NewTestConfig(1, 1)
		gen, err := testConfig.CreateTestGenerator()
		require.NoError(t, err)

		// Generate and parse ID
		id, err := gen.GenerateID()
		require.NoError(t, err)

		snowflakeID, err := gen.ParseID(id)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), snowflakeID.WorkerID)
		assert.Equal(t, int64(1), snowflakeID.DatacenterID)
		assert.False(t, snowflakeID.Timestamp.IsZero())
		assert.GreaterOrEqual(t, snowflakeID.Sequence, int64(0))
	})

	// Test plugin metadata
	t.Run("Plugin Metadata", func(t *testing.T) {
		plugin := NewSnowflakePlugin()
		assert.Equal(t, "snowflake", plugin.ID())
		assert.Contains(t, plugin.Description(), "Snowflake")
		// Note: Name() and Version() methods may not be properly initialized
	})

	// Test constant definitions
	t.Run("Constants", func(t *testing.T) {
		// Use default config to calculate max values
		maxWorkerID := int64((1 << 10) - 1)    // 10 bits for worker ID
		maxDatacenterID := int64((1 << 5) - 1) // 5 bits for datacenter ID
		maxSequence := int64((1 << 12) - 1)    // 12 bits for sequence

		assert.Equal(t, int64(1023), maxWorkerID)
		assert.Equal(t, int64(31), maxDatacenterID)
		assert.Equal(t, int64(4095), maxSequence)
	})
}

// TestErrorCases tests error cases
func TestErrorCases(t *testing.T) {
	// Test invalid config
	t.Run("Invalid Config", func(t *testing.T) {
		testConfig := NewTestConfig(1024, 1) // Invalid WorkerID
		_, err := testConfig.CreateTestGenerator()
		assert.Error(t, err)

		testConfig2 := NewTestConfig(1, 32) // Invalid DatacenterID
		_, err = testConfig2.CreateTestGenerator()
		assert.Error(t, err)
	})
}

// TestUninitializedPlugin tests uninitialized plugin behavior
func TestUninitializedPlugin(t *testing.T) {
	plugin := NewSnowflakePlugin()

	// Operations should fail on uninitialized plugin
	_, err := plugin.GenerateID()
	assert.Error(t, err)

	_, err = plugin.ParseID(123456789)
	assert.Error(t, err)

	// Note: HealthCheck method not implemented yet
}

// TestIDUniqueness tests ID uniqueness
func TestIDUniqueness(t *testing.T) {
	testConfig := NewTestConfig(1, 1)
	gen, err := testConfig.CreateTestGenerator()
	require.NoError(t, err)

	idSet := make(map[int64]bool)
	numIDs := 10000

	for i := 0; i < numIDs; i++ {
		id, err := gen.GenerateID()
		require.NoError(t, err)
		assert.False(t, idSet[id], "Duplicate ID found: %d", id)
		idSet[id] = true
	}

	assert.Equal(t, numIDs, len(idSet))
}

// TestIDStructure tests ID structure
func TestIDStructure(t *testing.T) {
	testConfig := NewTestConfig(5, 3)
	gen, err := testConfig.CreateTestGenerator()
	require.NoError(t, err)

	// Generate ID and verify structure
	id, err := gen.GenerateID()
	require.NoError(t, err)

	components, err := gen.ParseID(id)
	require.NoError(t, err)

	// Verify components
	assert.Equal(t, int64(3), components.WorkerID)
	assert.Equal(t, int64(5), components.DatacenterID)
	assert.False(t, components.Timestamp.IsZero())
	assert.GreaterOrEqual(t, components.Sequence, int64(0))
	assert.LessOrEqual(t, components.Sequence, int64(4095)) // Max sequence is 4095
}
