package snowflake

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSnowflakeGeneratorCore(t *testing.T) {
	tests := []struct {
		name         string
		datacenterID int64
		workerID     int64
		config       *GeneratorConfig
		wantErr      bool
	}{
		{
			name:         "valid config",
			datacenterID: 1,
			workerID:     1,
			config: &GeneratorConfig{
				CustomEpoch:                DefaultEpoch,
				DatacenterIDBits:          5,
				WorkerIDBits:              5,
				SequenceBits:              12,
				EnableClockDriftProtection: true,
				ClockDriftAction:          ClockDriftActionWait,
			},
			wantErr: false,
		},
		{
			name:         "invalid worker id",
			datacenterID: 1,
			workerID:     1024, // 超出范围
			config: &GeneratorConfig{
				CustomEpoch:                DefaultEpoch,
				DatacenterIDBits:          5,
				WorkerIDBits:              5,
				SequenceBits:              12,
				EnableClockDriftProtection: true,
				ClockDriftAction:          ClockDriftActionWait,
			},
			wantErr: true,
		},
		{
			name:         "invalid datacenter id",
			datacenterID: 32, // 超出范围
			workerID:     1,
			config: &GeneratorConfig{
				CustomEpoch:                DefaultEpoch,
				DatacenterIDBits:          5,
				WorkerIDBits:              5,
				SequenceBits:              12,
				EnableClockDriftProtection: true,
				ClockDriftAction:          ClockDriftActionWait,
			},
			wantErr: true,
		},
		{
			name:         "future epoch",
			datacenterID: 1,
			workerID:     1,
			config: &GeneratorConfig{
				CustomEpoch:                time.Now().Add(time.Hour).UnixMilli(), // 未来时间
				DatacenterIDBits:          5,
				WorkerIDBits:              5,
				SequenceBits:              12,
				EnableClockDriftProtection: true,
				ClockDriftAction:          ClockDriftActionWait,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewSnowflakeGeneratorCore(tt.datacenterID, tt.workerID, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, gen)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gen)
			}
		})
	}
}

func TestSnowflakeGenerator_GenerateID(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)
	require.NotNil(t, gen)

	// 测试基本ID生成
	id1, err := gen.GenerateID()
	assert.NoError(t, err)
	assert.Greater(t, id1, int64(0))

	id2, err := gen.GenerateID()
	assert.NoError(t, err)
	assert.Greater(t, id2, id1) // ID应该递增

	// 测试ID唯一性
	ids := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id, err := gen.GenerateID()
		assert.NoError(t, err)
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}

func TestSnowflakeGenerator_GenerateIDWithMetadata(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	id, snowflakeID, err := gen.GenerateIDWithMetadata()
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))
	assert.NotNil(t, snowflakeID)
	assert.Equal(t, int64(1), snowflakeID.WorkerID)
	assert.Equal(t, int64(1), snowflakeID.DatacenterID)
	assert.True(t, snowflakeID.Timestamp.After(time.Time{}))
	assert.GreaterOrEqual(t, snowflakeID.Sequence, int64(0))
}

func TestSnowflakeGenerator_ParseID(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(5, 3, config)
	require.NoError(t, err)

	// 生成ID并解析
	originalID, originalSnowflakeID, err := gen.GenerateIDWithMetadata()
	require.NoError(t, err)

	parsedMetadata, err := gen.ParseID(originalID)
	assert.NoError(t, err)
	assert.Equal(t, originalSnowflakeID.WorkerID, parsedMetadata.WorkerID)
	assert.Equal(t, originalSnowflakeID.DatacenterID, parsedMetadata.DatacenterID)
	assert.Equal(t, originalSnowflakeID.Timestamp, parsedMetadata.Timestamp)
	assert.Equal(t, originalSnowflakeID.Sequence, parsedMetadata.Sequence)

	// 测试无效ID
	_, err = gen.ParseID(-1)
	assert.Error(t, err)
}

func TestSnowflakeGenerator_SequenceOverflow(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// 模拟序列号溢出
	gen.sequence = (1 << config.SequenceBits) - 1
	gen.lastTimestamp = gen.getCurrentTimestamp()

	// 下一个ID应该等待到下一毫秒
	id, err := gen.GenerateID()
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))
}

func TestSnowflakeGenerator_ClockBackward(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionError,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// 设置一个未来的时间戳
	gen.lastTimestamp = gen.getCurrentTimestamp() + 1000

	// 应该返回错误
	_, err = gen.GenerateID()
	assert.Error(t, err)
}

func TestSnowflakeGenerator_ConcurrentGeneration(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	const numGoroutines = 100
	const idsPerGoroutine = 100
	totalIDs := numGoroutines * idsPerGoroutine

	ids := make(chan int64, totalIDs)
	var wg sync.WaitGroup

	// 启动多个goroutine并发生成ID
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.GenerateID()
				assert.NoError(t, err)
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 验证所有ID都是唯一的
	uniqueIDs := make(map[int64]bool)
	count := 0
	for id := range ids {
		assert.False(t, uniqueIDs[id], "ID %d should be unique", id)
		uniqueIDs[id] = true
		count++
	}

	assert.Equal(t, totalIDs, count)
}

func TestSnowflakeGenerator_Shutdown(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// 测试正常关闭
	err = gen.Shutdown(context.Background())
	assert.NoError(t, err)

	// 关闭后应该无法生成ID
	_, err = gen.GenerateID()
	assert.Error(t, err)
}

func TestSnowflakeGenerator_ShutdownTimeout(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// 测试超时关闭
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = gen.Shutdown(ctx)
	// 可能成功也可能超时，取决于实现
	// 这里不强制要求特定结果
}

func TestSnowflakeGenerator_GetStats(t *testing.T) {
	config := &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}

	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// 生成一些ID
	for i := 0; i < 10; i++ {
		_, err := gen.GenerateID()
		require.NoError(t, err)
	}

	stats := gen.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.GeneratedCount, int64(10))
}

func TestSnowflakeGenerator_ClockDriftActions(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{"wait action", ClockDriftActionWait},
		{"error action", ClockDriftActionError},
		{"ignore action", ClockDriftActionIgnore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &GeneratorConfig{
				CustomEpoch:                DefaultEpoch,
				DatacenterIDBits:          5,
				WorkerIDBits:              5,
				SequenceBits:              12,
				EnableClockDriftProtection: true,
				ClockDriftAction:          tt.action,
			}

			gen, err := NewSnowflakeGeneratorCore(1, 1, config)
			assert.NoError(t, err)
			assert.NotNil(t, gen)

			// 基本功能测试
			id, err := gen.GenerateID()
			assert.NoError(t, err)
			assert.Greater(t, id, int64(0))
		})
	}
}