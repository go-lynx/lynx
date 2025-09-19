package snowflake

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicFunctionality 测试基本功能
func TestBasicFunctionality(t *testing.T) {
	// 测试生成器配置验证
	t.Run("Generator Config Validation", func(t *testing.T) {
		// 有效配置
		config := GetTestGeneratorConfig(1, 1)
		gen, err := NewSnowflakeGeneratorCore(1, 1, config)
		assert.NoError(t, err)
		assert.NotNil(t, gen)

		// 无效WorkerID
		invalidConfig := GetTestGeneratorConfig(1024, 1)
		gen, err = NewSnowflakeGeneratorCore(1, 1024, invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, gen)
	})

	// 测试ID生成
	t.Run("ID Generation", func(t *testing.T) {
		config := GetTestGeneratorConfig(1, 1)
		gen, err := NewSnowflakeGeneratorCore(1, 1, config)
		require.NoError(t, err)

		// 生成ID
		id1, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.Greater(t, id1, int64(0))

		id2, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.Greater(t, id2, id1)

		// 生成带元数据的ID
		id3, snowflakeID, err := gen.GenerateIDWithMetadata()
		assert.NoError(t, err)
		assert.Greater(t, id3, id2)
		assert.NotNil(t, snowflakeID)
		assert.Equal(t, int64(1), snowflakeID.WorkerID)
		assert.Equal(t, int64(1), snowflakeID.DatacenterID)
	})

	// 测试ID解析
	t.Run("ID Parsing", func(t *testing.T) {
		config := GetTestGeneratorConfig(5, 3)
		gen, err := NewSnowflakeGeneratorCore(3, 5, config)
		require.NoError(t, err)

		// 生成并解析ID
		originalID, originalSnowflakeID, err := gen.GenerateIDWithMetadata()
		require.NoError(t, err)

		parsedMetadata, err := gen.ParseID(originalID)
		assert.NoError(t, err)
		assert.Equal(t, originalSnowflakeID.WorkerID, parsedMetadata.WorkerID)
		assert.Equal(t, originalSnowflakeID.DatacenterID, parsedMetadata.DatacenterID)
		assert.Equal(t, originalSnowflakeID.Timestamp, parsedMetadata.Timestamp)
		assert.Equal(t, originalSnowflakeID.Sequence, parsedMetadata.Sequence)
	})

	// 测试插件元数据
	t.Run("Plugin Metadata", func(t *testing.T) {
		plugin := NewSnowflakePlugin()
		assert.Equal(t, PluginName, plugin.Name())
		assert.Equal(t, PluginVersion, plugin.Version())
		assert.Equal(t, PluginDescription, plugin.Description())
		assert.Equal(t, plugins.StatusInactive, plugin.Status(plugin))
	})

	// 测试常量定义
	t.Run("Constants", func(t *testing.T) {
		assert.Equal(t, "snowflake", PluginName)
		assert.Equal(t, "lynx.snowflake", ConfPrefix)
		// 使用默认配置计算最大值
		maxWorkerID := int64((1 << 10) - 1)     // 10 bits for worker ID
		maxDatacenterID := int64((1 << 5) - 1)  // 5 bits for datacenter ID  
		maxSequence := int64((1 << 12) - 1)     // 12 bits for sequence
		assert.Greater(t, maxWorkerID, int64(0))
		assert.Greater(t, maxDatacenterID, int64(0))
		assert.Greater(t, maxSequence, int64(0))
	})
}

// TestErrorCases 测试错误情况
func TestErrorCases(t *testing.T) {
	// 测试无效配置
	t.Run("Invalid Configurations", func(t *testing.T) {
		maxWorkerID := int64((1 << 10) - 1)     // 10 bits for worker ID
		maxDatacenterID := int64((1 << 5) - 1)  // 5 bits for datacenter ID
		
		// WorkerID超出范围
		config := GetTestGeneratorConfig(1, 1)
		gen, err := NewSnowflakeGeneratorCore(maxDatacenterID+1, maxWorkerID+1, config)
		assert.Error(t, err)
		assert.Nil(t, gen)

		// DatacenterID超出范围
		gen, err = NewSnowflakeGeneratorCore(maxDatacenterID+1, 1, config)
		assert.Error(t, err)
		assert.Nil(t, gen)
	})

	// 测试未初始化的插件
	t.Run("Uninitialized Plugin", func(t *testing.T) {
		plugin := NewSnowflakePlugin()

		// 未初始化状态下的操作应该失败
		_, err := plugin.GenerateID()
		assert.Error(t, err)

		_, _, err = plugin.GenerateIDWithMetadata()
		assert.Error(t, err)

		_, err = plugin.ParseID(123456)
		assert.Error(t, err)

		err = plugin.CheckHealth()
		assert.Error(t, err)
	})
}

// TestIDUniqueness 测试ID唯一性
func TestIDUniqueness(t *testing.T) {
	config := GetTestGeneratorConfig(1, 1)
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	const numIDs = 10000
	ids := make(map[int64]bool)

	for i := 0; i < numIDs; i++ {
		id, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.False(t, ids[id], "ID should be unique: %d", id)
		ids[id] = true
	}

	assert.Len(t, ids, numIDs)
}

// TestIDStructure 测试ID结构
func TestIDStructure(t *testing.T) {
	config := GetTestGeneratorConfig(15, 10)
	gen, err := NewSnowflakeGeneratorCore(10, 15, config)
	require.NoError(t, err)

	id, snowflakeID, err := gen.GenerateIDWithMetadata()
	require.NoError(t, err)

	// 验证ID结构
	assert.Equal(t, int64(15), snowflakeID.WorkerID)
	assert.Equal(t, int64(10), snowflakeID.DatacenterID)
	assert.GreaterOrEqual(t, snowflakeID.Sequence, int64(0))
	maxSequence := int64((1 << 12) - 1)  // 12 bits for sequence
	assert.LessOrEqual(t, snowflakeID.Sequence, maxSequence)
	assert.True(t, snowflakeID.Timestamp.After(time.Time{}))

	// 验证ID可以正确解析
	parsedMetadata, err := gen.ParseID(id)
	assert.NoError(t, err)
	assert.Equal(t, snowflakeID, parsedMetadata)
}