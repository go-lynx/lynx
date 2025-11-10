package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"
)

// RedisIntegrationTestSuite defines Redis integration test suite
type RedisIntegrationTestSuite struct {
	suite.Suite
	plugin  *PlugRedis
	client  redis.UniversalClient
	ctx     context.Context
	cancel  context.CancelFunc
	runtime *mockRuntime
}

// mockRuntime mock implementation of Runtime
type mockRuntime struct {
	resources map[string]interface{}
	mu        sync.RWMutex
}

func (m *mockRuntime) GetResource(id string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if res, ok := m.resources[id]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("resource %s not found", id)
}

func (m *mockRuntime) RegisterResource(id string, resource interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[id] = resource
	return nil
}

func (m *mockRuntime) GetPlugin(name string) plugins.Plugin {
	return nil
}

func (m *mockRuntime) PublishEvent(event interface{}) error {
	return nil
}

func (m *mockRuntime) SubscribeEvent(eventType string, handler func(interface{})) error {
	return nil
}

func (m *mockRuntime) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) RemoveListener(listener plugins.EventListener) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) EmitEvent(event plugins.PluginEvent) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	// Mock implementation - return empty slice
	return []plugins.PluginEvent{}
}

func (m *mockRuntime) GetPrivateResource(name string) (any, error) {
	return m.GetResource(name)
}

func (m *mockRuntime) RegisterPrivateResource(name string, resource any) error {
	return m.RegisterResource(name, resource)
}

func (m *mockRuntime) GetSharedResource(name string) (any, error) {
	return m.GetResource(name)
}

func (m *mockRuntime) RegisterSharedResource(name string, resource any) error {
	return m.RegisterResource(name, resource)
}

func (m *mockRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	// Mock implementation - return empty slice
	return []plugins.PluginEvent{}
}

func (m *mockRuntime) SetEventDispatchMode(mode string) error {
	// Mock implementation - do nothing
	return nil
}

func (m *mockRuntime) SetEventWorkerPoolSize(size int) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) SetEventTimeout(timeout time.Duration) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) GetEventStats() map[string]any {
	// Mock implementation - return empty map
	return make(map[string]any)
}

func (m *mockRuntime) WithPluginContext(pluginName string) plugins.Runtime {
	// Mock implementation - return self
	return m
}

func (m *mockRuntime) GetCurrentPluginContext() string {
	// Mock implementation - return empty string
	return ""
}

func (m *mockRuntime) SetConfig(conf config.Config) {
	// Mock implementation - do nothing
}

func (m *mockRuntime) GetConfig() config.Config {
	// Mock implementation - return nil
	return nil
}

func (m *mockRuntime) GetLogger() log.Logger {
	// Mock implementation - return nil
	return nil
}

// CleanupResources implements the missing method from plugins.Runtime interface
func (m *mockRuntime) CleanupResources(resourceType string) error {
	// Mock implementation - clear all resources
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = make(map[string]interface{})
	return nil
}

// GetTypedResource implements the TypedResourceManager interface
func (m *mockRuntime) GetTypedResource(name string, resourceType string) (any, error) {
	return m.GetResource(name)
}

// RegisterTypedResource implements the TypedResourceManager interface
func (m *mockRuntime) RegisterTypedResource(name string, resource any, resourceType string) error {
	return m.RegisterResource(name, resource)
}

func (m *mockRuntime) GetResourceInfo(resourceType string) (*plugins.ResourceInfo, error) {
	return nil, nil
}

func (m *mockRuntime) GetResourceStats() map[string]any {
	return make(map[string]any)
}

func (m *mockRuntime) ListResources() []*plugins.ResourceInfo {
	return []*plugins.ResourceInfo{}
}

// SetupSuite sets up the test suite
func (suite *RedisIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.runtime = &mockRuntime{
		resources: make(map[string]interface{}),
	}

	// Create Redis plugin
	suite.plugin = NewRedisClient()

	// Set configuration
	config := &conf.Redis{
		Addrs:          []string{"localhost:6379"},
		Password:       "",
		Db:             0,
		MaxActiveConns: 10,
		MinIdleConns:   5,
		MaxIdleConns:   10,
		MaxRetries:     3,
		DialTimeout:    durationpb.New(5 * time.Second),
		ReadTimeout:    durationpb.New(3 * time.Second),
		WriteTimeout:   durationpb.New(3 * time.Second),
		PoolTimeout:    durationpb.New(4 * time.Second),
		IdleTimeout:    durationpb.New(300 * time.Second),
	}

	suite.plugin.conf = config

	// Initialize plugin
	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	// Start plugin
	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// Get client
	suite.client = suite.plugin.rdb
	require.NotNil(suite.T(), suite.client)

	// Cleanup test data
	suite.client.FlushDB(suite.ctx)
}

// TearDownSuite cleans up the test suite
func (suite *RedisIntegrationTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.FlushDB(suite.ctx)
	}
	if suite.plugin != nil {
		suite.plugin.Stop(suite.plugin)
	}
	suite.cancel()
}

// TestBasicOperations tests basic operations
func (suite *RedisIntegrationTestSuite) TestBasicOperations() {
	ctx := suite.ctx
	client := suite.client

	// Test SET and GET
	err := client.Set(ctx, "test_key", "test_value", time.Minute).Err()
	assert.NoError(suite.T(), err)

	val, err := client.Get(ctx, "test_key").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test_value", val)

	// Test DEL
	err = client.Del(ctx, "test_key").Err()
	assert.NoError(suite.T(), err)

	// Verify deletion
	_, err = client.Get(ctx, "test_key").Result()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), redis.Nil, err)
}

// TestHashOperations tests Hash operations
func (suite *RedisIntegrationTestSuite) TestHashOperations() {
	ctx := suite.ctx
	client := suite.client

	// Test HSET and HGET
	err := client.HSet(ctx, "test_hash", "field1", "value1").Err()
	assert.NoError(suite.T(), err)

	val, err := client.HGet(ctx, "test_hash", "field1").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value1", val)

	// Test HMSET and HMGET
	err = client.HMSet(ctx, "test_hash", map[string]interface{}{
		"field2": "value2",
		"field3": "value3",
	}).Err()
	assert.NoError(suite.T(), err)

	vals, err := client.HMGet(ctx, "test_hash", "field2", "field3").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []interface{}{"value2", "value3"}, vals)

	// Test HGETALL
	all, err := client.HGetAll(ctx, "test_hash").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), map[string]string{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
	}, all)
}

// TestListOperations tests List operations
func (suite *RedisIntegrationTestSuite) TestListOperations() {
	ctx := suite.ctx
	client := suite.client

	// Test LPUSH and RPUSH
	err := client.LPush(ctx, "test_list", "left1", "left2").Err()
	assert.NoError(suite.T(), err)

	err = client.RPush(ctx, "test_list", "right1", "right2").Err()
	assert.NoError(suite.T(), err)

	// Test LLEN
	length, err := client.LLen(ctx, "test_list").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(4), length)

	// Test LRANGE
	vals, err := client.LRange(ctx, "test_list", 0, -1).Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"left2", "left1", "right1", "right2"}, vals)
}

// TestSetOperations tests Set operations
func (suite *RedisIntegrationTestSuite) TestSetOperations() {
	ctx := suite.ctx
	client := suite.client

	// Test SADD
	err := client.SAdd(ctx, "test_set", "member1", "member2", "member3").Err()
	assert.NoError(suite.T(), err)

	// Test SISMEMBER
	exists, err := client.SIsMember(ctx, "test_set", "member2").Result()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)

	// Test SMEMBERS
	members, err := client.SMembers(ctx, "test_set").Result()
	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"member1", "member2", "member3"}, members)

	// Test SREM
	err = client.SRem(ctx, "test_set", "member2").Err()
	assert.NoError(suite.T(), err)

	exists, err = client.SIsMember(ctx, "test_set", "member2").Result()
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// TestExpiration tests expiration
func (suite *RedisIntegrationTestSuite) TestExpiration() {
	ctx := suite.ctx
	client := suite.client

	// Set a key with expiration
	err := client.Set(ctx, "expire_key", "value", 2*time.Second).Err()
	assert.NoError(suite.T(), err)

	// Check immediately
	val, err := client.Get(ctx, "expire_key").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value", val)

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// Verify expired
	_, err = client.Get(ctx, "expire_key").Result()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), redis.Nil, err)
}

// TestTransactions tests transactions
func (suite *RedisIntegrationTestSuite) TestTransactions() {
	ctx := suite.ctx
	client := suite.client

	// Use WATCH and transaction
	key := "counter"
	client.Set(ctx, key, "0", 0)

	// Execute transaction
	err := client.Watch(ctx, func(tx *redis.Tx) error {
		n, getErr := tx.Get(ctx, key).Int()
		if getErr != nil && getErr != redis.Nil {
			return getErr
		}

		_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, n+1, 0)
			return nil
		})
		return txErr
	}, key)

	assert.NoError(suite.T(), err)

	// Verify result
	val, err := client.Get(ctx, key).Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "1", val)
}

// TestPipelining tests pipelining
func (suite *RedisIntegrationTestSuite) TestPipelining() {
	ctx := suite.ctx
	client := suite.client

	// Use pipeline to execute commands in bulk
	pipe := client.Pipeline()

	for i := 0; i < 10; i++ {
		pipe.Set(ctx, fmt.Sprintf("pipe_key_%d", i), fmt.Sprintf("value_%d", i), 0)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	assert.NoError(suite.T(), err)

	// Verify results
	for i := 0; i < 10; i++ {
		val, err := client.Get(ctx, fmt.Sprintf("pipe_key_%d", i)).Result()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), fmt.Sprintf("value_%d", i), val)
	}
}

// TestConcurrency tests concurrency safety
func (suite *RedisIntegrationTestSuite) TestConcurrency() {
	ctx := suite.ctx
	client := suite.client

	// Set initial value
	client.Set(ctx, "concurrent_counter", "0", 0)

	// Concurrently increment the counter
	var wg sync.WaitGroup
	var successCount int32
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Use Lua script to guarantee atomicity
			script := redis.NewScript(`
				local current = redis.call('GET', KEYS[1])
				if not current then
					current = 0
				end
				local new = tonumber(current) + 1
				redis.call('SET', KEYS[1], new)
				return new
			`)

			_, err := script.Run(ctx, client, []string{"concurrent_counter"}).Result()
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Verify results
	val, err := client.Get(ctx, "concurrent_counter").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf("%d", successCount), val)
	assert.Equal(suite.T(), int32(iterations), successCount)
}

// TestConnectionPoolMetrics tests connection pool metrics
func (suite *RedisIntegrationTestSuite) TestConnectionPoolMetrics() {
	// Get pool stats
	stats := suite.plugin.GetPoolStats()
	assert.NotNil(suite.T(), stats)

	// Validate metrics
	assert.GreaterOrEqual(suite.T(), stats.TotalConns, uint32(0))
	assert.GreaterOrEqual(suite.T(), stats.IdleConns, uint32(0))
	assert.GreaterOrEqual(suite.T(), stats.Hits, uint64(0))
	assert.GreaterOrEqual(suite.T(), stats.Timeouts, uint64(0))
}

// TestHealthCheck tests health check
func (suite *RedisIntegrationTestSuite) TestHealthCheck() {
	// Perform health check
	healthy, err := suite.plugin.HealthCheck()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), healthy)
}

// TestRunIntegrationTestSuite runs the test suite
func TestRunIntegrationTestSuite(t *testing.T) {
	// Skip if no Redis server
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server is not available")
		return
	}
	client.Close()

	suite.Run(t, new(RedisIntegrationTestSuite))
}
