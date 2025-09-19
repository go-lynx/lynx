package snowflake

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRedisClient 模拟Redis客户端
type MockRedisClient struct {
	redis.UniversalClient
	data map[string]string
	sets map[string]map[string]bool
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
		sets: make(map[string]map[string]bool),
	}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.data[key] = fmt.Sprintf("%v", value)
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if val, exists := m.data[key]; exists {
		cmd.SetVal(val)
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			delete(m.data, key)
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if m.sets[key] == nil {
		m.sets[key] = make(map[string]bool)
	}
	count := int64(0)
	for _, member := range members {
		memberStr := fmt.Sprintf("%v", member)
		if !m.sets[key][memberStr] {
			m.sets[key][memberStr] = true
			count++
		}
	}
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if m.sets[key] == nil {
		cmd.SetVal(0)
		return cmd
	}
	count := int64(0)
	for _, member := range members {
		memberStr := fmt.Sprintf("%v", member)
		if m.sets[key][memberStr] {
			delete(m.sets[key], memberStr)
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	if m.sets[key] == nil {
		cmd.SetVal([]string{})
		return cmd
	}
	members := make([]string, 0, len(m.sets[key]))
	for member := range m.sets[key] {
		members = append(members, member)
	}
	cmd.SetVal(members)
	return cmd
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}

func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	if _, exists := m.data[key]; !exists {
		m.data[key] = fmt.Sprintf("%v", value)
		cmd.SetVal(true)
	} else {
		cmd.SetVal(false)
	}
	return cmd
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	count := int64(0)
	for _, key := range keys {
		if _, exists := m.data[key]; exists {
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func TestNewWorkerIDManager(t *testing.T) {
	mockClient := NewMockRedisClient()
	config := &WorkerManagerConfig{
		KeyPrefix:         "test:snowflake",
		TTL:               30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}

	manager := NewWorkerIDManager(mockClient, 1, config)
	assert.NotNil(t, manager)
	assert.Equal(t, mockClient, manager.redisClient)
	assert.Equal(t, int64(1), manager.datacenterID)
}

func TestWorkerIDManager_RegisterWorkerID(t *testing.T) {
	mockClient := NewMockRedisClient()
	config := &WorkerManagerConfig{
		KeyPrefix:         "test:snowflake",
		TTL:               30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}

	manager := NewWorkerIDManager(mockClient, 1, config)
	ctx := context.Background()

	// 测试自动注册
	workerID, err := manager.RegisterWorkerID(ctx, 1023)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, workerID, int64(0))
	assert.Less(t, workerID, int64(1024))

	// 验证WorkerID已设置
	assert.Equal(t, workerID, manager.GetWorkerID())
}

func TestWorkerIDManager_RegisterSpecificWorkerID(t *testing.T) {
	mockClient := NewMockRedisClient()
	config := &WorkerManagerConfig{
		KeyPrefix:         "test:snowflake",
		TTL:               30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}

	manager := NewWorkerIDManager(mockClient, 1, config)
	ctx := context.Background()

	// 测试指定WorkerID注册
	err := manager.RegisterSpecificWorkerID(ctx, 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), manager.GetWorkerID())

	// 测试注册已被占用的WorkerID
	manager2 := NewWorkerIDManager(mockClient, 1, config)
	err = manager2.RegisterSpecificWorkerID(ctx, 5)
	assert.Error(t, err)
}

func TestWorkerIDManager_UnregisterWorkerID(t *testing.T) {
	mockClient := NewMockRedisClient()
	config := &WorkerManagerConfig{
		KeyPrefix:         "test:snowflake",
		TTL:               30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}

	manager := NewWorkerIDManager(mockClient, 1, config)
	ctx := context.Background()

	// 先注册一个worker
	_, err := manager.RegisterWorkerID(ctx, 1023)
	require.NoError(t, err)

	// 测试注销
	err = manager.UnregisterWorkerID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), manager.GetWorkerID())
}
