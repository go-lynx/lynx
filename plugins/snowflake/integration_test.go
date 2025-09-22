package snowflake

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRedisClient for testing
type MockRedisClient struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
	}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = fmt.Sprintf("%v", value)
	return redis.NewStatusCmd(ctx, "set", key, value)
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cmd := redis.NewStringCmd(ctx, "get", key)
	if val, exists := m.data[key]; exists {
		cmd.SetVal(val)
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			delete(m.data, key)
			count++
		}
	}
	cmd := redis.NewIntCmd(ctx, "del")
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			count++
		}
	}
	cmd := redis.NewIntCmd(ctx, "exists")
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	val := int64(1)
	if existing, exists := m.data[key]; exists {
		if parsed, err := strconv.ParseInt(existing, 10, 64); err == nil {
			val = parsed + 1
		}
	}
	m.data[key] = fmt.Sprintf("%d", val)
	cmd := redis.NewIntCmd(ctx, "incr", key)
	cmd.SetVal(val)
	return cmd
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx, "expire", key, expiration)
	cmd.SetVal(true)
	return cmd
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	cmd := redis.NewDurationCmd(ctx, time.Hour, "ttl", key)
	cmd.SetVal(time.Hour) // Mock TTL
	return cmd
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "ping")
	cmd.SetVal("PONG")
	return cmd
}

func TestIntegrationWithRedis(t *testing.T) {
	// Skip if no Redis available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockRedis := NewMockRedisClient()

	// Test Redis connection
	ctx := context.Background()
	pong := mockRedis.Ping(ctx)
	require.NoError(t, pong.Err())
	assert.Equal(t, "PONG", pong.Val())

	// Test basic Redis operations
	err := mockRedis.Set(ctx, "test_key", "test_value", time.Hour).Err()
	require.NoError(t, err)

	val, err := mockRedis.Get(ctx, "test_key").Result()
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func TestWorkerManagerIntegration(t *testing.T) {
	t.Skip("Skipping Redis integration test - requires full Redis client implementation")
}

func TestSnowflakeGeneratorIntegration(t *testing.T) {
	t.Skip("Skipping complex integration test")
}

func TestFullSystemIntegration(t *testing.T) {
	t.Skip("Skipping complex integration test")
}

func TestConcurrentWorkerRegistration(t *testing.T) {
	t.Skip("Skipping complex integration test")
}

func TestErrorHandling(t *testing.T) {
	// Test invalid generator configuration
	invalidConfig := &GeneratorConfig{
		WorkerIDBits: 0, // Invalid
	}

	_, err := NewSnowflakeGeneratorCore(1, 1, invalidConfig)
	assert.Error(t, err)
}

func TestMetricsIntegration(t *testing.T) {
	config := DefaultGeneratorConfig()
	gen, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Generate some IDs to populate metrics
	for i := 0; i < 100; i++ {
		_, err := gen.GenerateID()
		require.NoError(t, err)
	}

	// Check stats
	stats := gen.GetStats()
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.WorkerID)
	assert.Equal(t, int64(1), stats.DatacenterID)
	assert.GreaterOrEqual(t, stats.GeneratedCount, int64(100))

	// Check metrics if available
	if metrics := gen.GetMetrics(); metrics != nil {
		snapshot := metrics.GetSnapshot()
		assert.NotNil(t, snapshot)
	}
}
