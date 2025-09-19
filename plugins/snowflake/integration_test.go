package snowflake

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"
)

// SnowflakeIntegrationTestSuite 雪花ID插件集成测试套件
type SnowflakeIntegrationTestSuite struct {
	suite.Suite
	plugin     *PlugSnowflake
	redisClient redis.UniversalClient
	ctx        context.Context
	cancel     context.CancelFunc
	runtime    *mockRuntime
}

// mockRuntime 模拟Runtime实现
type mockRuntime struct {
	resources map[string]interface{}
	mu        sync.RWMutex
}

func (m *mockRuntime) GetResource(name string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.resources == nil {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	resource, exists := m.resources[name]
	if !exists {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	return resource, nil
}

func (m *mockRuntime) RegisterResource(name string, resource any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resources == nil {
		m.resources = make(map[string]interface{})
	}
	m.resources[name] = resource
	return nil
}

func (m *mockRuntime) GetConfig() config.Config {
	// 返回模拟配置
	return &mockConfig{data: make(map[string]interface{})}
}

func (m *mockRuntime) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation - do nothing
}

// AddPluginListener adds a plugin listener (mock implementation)
func (m *mockRuntime) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation
}

func (m *mockRuntime) CleanupResources(pluginID string) error {
	// Mock implementation
	return nil
}

func (m *mockRuntime) GetResourceInfo(name string) (*plugins.ResourceInfo, error) {
	// Mock implementation
	return nil, nil
}

func (m *mockRuntime) ListResources() []*plugins.ResourceInfo {
	// Mock implementation
	return nil
}

func (m *mockRuntime) GetResourceStats() map[string]any {
	// Mock implementation
	return nil
}

func (m *mockRuntime) GetPrivateResource(name string) (any, error) {
	// Mock implementation
	return nil, nil
}

func (m *mockRuntime) RegisterPrivateResource(name string, resource any) error {
	// Mock implementation
	return nil
}

func (m *mockRuntime) GetSharedResource(name string) (any, error) {
	// Mock implementation
	return nil, nil
}

func (m *mockRuntime) RegisterSharedResource(name string, resource any) error {
	// Mock implementation
	return nil
}

func (m *mockRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	// Mock implementation
}

func (m *mockRuntime) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	// Mock implementation
	return nil
}

func (m *mockRuntime) SetEventDispatchMode(mode string) error {
	// Mock implementation
	return nil
}

func (m *mockRuntime) SetEventWorkerPoolSize(size int) {
	// Mock implementation
}

func (m *mockRuntime) SetEventTimeout(timeout time.Duration) {
	// Mock implementation
}

func (m *mockRuntime) GetEventStats() map[string]any {
	// Mock implementation
	return nil
}

func (m *mockRuntime) WithPluginContext(pluginName string) plugins.Runtime {
	// Mock implementation
	return m
}

func (m *mockRuntime) GetCurrentPluginContext() string {
	// Mock implementation
	return ""
}

func (m *mockRuntime) EmitEvent(event plugins.PluginEvent) {
	// Mock implementation
}

func (m *mockRuntime) RemoveListener(listener plugins.EventListener) {
	// Mock implementation
}

func (m *mockRuntime) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	// Mock implementation
	return nil
}

func (m *mockRuntime) GetLogger() log.Logger {
	// Mock implementation - return a default logger
	return log.DefaultLogger
}

func (m *mockRuntime) SetConfig(conf config.Config) {
	// Mock implementation
}

type mockConfig struct {
	data map[string]interface{}
}

func (m *mockConfig) Value(key string) config.Value {
	return &mockValue{data: m.data}
}

func (m *mockConfig) Load() error {
	return nil
}

func (m *mockConfig) Watch(key string, o config.Observer) error {
	return nil
}

func (m *mockConfig) Scan(v interface{}) error {
	// 如果有snowflake配置，将其复制到目标结构
	if snowflakeConfig, ok := m.data["snowflake"]; ok {
		if target, ok := v.(*pb.Snowflake); ok {
			if source, ok := snowflakeConfig.(*pb.Snowflake); ok {
				*target = *source
				return nil
			}
		}
	}
	return nil
}

func (m *mockConfig) Close() error {
	return nil
}

type mockValue struct {
	data interface{}
}

func (m *mockValue) Bool() (bool, error) {
	return false, nil
}

func (m *mockValue) Int() (int64, error) {
	return 0, nil
}

func (m *mockValue) Float() (float64, error) {
	return 0.0, nil
}

func (m *mockValue) String() (string, error) {
	return "", nil
}

func (m *mockValue) Duration() (time.Duration, error) {
	return 0, nil
}

func (m *mockValue) Slice() ([]config.Value, error) {
	return nil, nil
}

func (m *mockValue) Map() (map[string]config.Value, error) {
	return nil, nil
}

func (m *mockValue) Load() any {
	return nil
}

func (m *mockValue) Store(any) {}

func (m *mockValue) Scan(dest interface{}) error {
	return nil
}

// SetupSuite 设置测试套件
func (suite *SnowflakeIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// 尝试连接Redis
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // 使用测试数据库
	})

	// 测试Redis连接
	if err := suite.redisClient.Ping(suite.ctx).Err(); err != nil {
		suite.T().Skip("Redis server is not available, skipping integration tests")
		return
	}

	// 清理测试数据
	suite.redisClient.FlushDB(suite.ctx)

	// 创建模拟runtime
	suite.runtime = &mockRuntime{
		resources: make(map[string]interface{}),
	}

	// 设置Redis客户端到runtime
	suite.runtime.RegisterResource("redis.client", suite.redisClient)

	// 创建插件实例
	suite.plugin = NewSnowflakePlugin()
	suite.plugin.runtime = suite.runtime
	suite.plugin.logger = log.DefaultLogger
}

// TearDownSuite 清理测试套件
func (suite *SnowflakeIntegrationTestSuite) TearDownSuite() {
	if suite.redisClient != nil {
		suite.redisClient.FlushDB(suite.ctx)
		suite.redisClient.Close()
	}
	if suite.plugin != nil {
		suite.plugin.Stop(suite.plugin)
	}
	suite.cancel()
}

// TestPluginLifecycle 测试插件生命周期
func (suite *SnowflakeIntegrationTestSuite) TestPluginLifecycle() {
	// 测试插件元数据
	assert.Equal(suite.T(), PluginName, suite.plugin.Name())
	assert.Equal(suite.T(), PluginVersion, suite.plugin.Version())
	assert.Equal(suite.T(), PluginDescription, suite.plugin.Description())

	// 测试初始化
	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), plugins.StatusActive, suite.plugin.Status(suite.plugin))

	// 测试启动
	err = suite.plugin.Start(suite.plugin)
	assert.NoError(suite.T(), err)

	// 测试健康检查
	healthy, err := suite.plugin.HealthCheck()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), healthy)

	// 测试停止
	err = suite.plugin.Stop(suite.plugin)
	assert.NoError(suite.T(), err)
}

// TestBasicIDGeneration 测试基本ID生成
func (suite *SnowflakeIntegrationTestSuite) TestBasicIDGeneration() {
	// 设置配置到mockRuntime
	config := &pb.Snowflake{
		WorkerId:                   1,
		DatacenterId:               1,
		AutoRegisterWorkerId:       true,
		RedisKeyPrefix:             "test:snowflake",
		WorkerIdTtl:                durationpb.New(30 * time.Second),
		HeartbeatInterval:          durationpb.New(10 * time.Second),
		EnableClockDriftProtection: true,
		MaxClockDrift:              durationpb.New(5 * time.Second),
		ClockDriftAction:           "wait",
		EnableSequenceCache:        false,
		SequenceCacheSize:          1000,
		RedisPluginName:            "redis",
		RedisDb:                    0,
		CustomEpoch:                1609459200000,
		WorkerIdBits:               10,
		SequenceBits:               12,
	}
	
	// 将配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// 测试ID生成
	id1, err := suite.plugin.GenerateID()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id1, int64(0))

	id2, err := suite.plugin.GenerateID()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id2, id1)

	// 测试带元数据的ID生成
	id3, metadata, err := suite.plugin.GenerateIDWithMetadata()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id3, id2)
	assert.NotNil(suite.T(), metadata)
	assert.Equal(suite.T(), int64(1), metadata.WorkerID)
	assert.Equal(suite.T(), int64(1), metadata.DatacenterID)

	// 测试ID解析
	parsedMetadata, err := suite.plugin.ParseID(id3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), metadata.WorkerID, parsedMetadata.WorkerID)
	assert.Equal(suite.T(), metadata.DatacenterID, parsedMetadata.DatacenterID)
	assert.Equal(suite.T(), metadata.Timestamp, parsedMetadata.Timestamp)
	assert.Equal(suite.T(), metadata.Sequence, parsedMetadata.Sequence)
}

// TestRedisIntegration 测试Redis集成
func (suite *SnowflakeIntegrationTestSuite) TestRedisIntegration() {
	// 测试自动WorkerID注册
	config := &pb.Snowflake{
		WorkerId:               -1, // 自动分配
		DatacenterId:           1,
		AutoRegisterWorkerId:   true,
		RedisKeyPrefix:         "test:snowflake",
		WorkerIdTtl:            durationpb.New(30 * time.Second),
		HeartbeatInterval:      durationpb.New(10 * time.Second),
		RedisPluginName:        "redis",
		RedisDb:                0,
	}

	// 将配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// 验证WorkerID已自动分配
	generator := suite.plugin.GetGenerator()
	assert.NotNil(suite.T(), generator)

	stats := generator.GetStats()
	assert.GreaterOrEqual(suite.T(), stats.WorkerID, int64(0))
	assert.Less(suite.T(), stats.WorkerID, int64(1024))

	// 测试ID生成
	id, err := suite.plugin.GenerateID()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id, int64(0))
}

// TestConcurrentGeneration 测试并发ID生成
func (suite *SnowflakeIntegrationTestSuite) TestConcurrentGeneration() {
	// 初始化插件
	config := &pb.Snowflake{
		WorkerId:               1,
		DatacenterId:           1,
		AutoRegisterWorkerId:   true,
		RedisKeyPrefix:         "test:snowflake",
		WorkerIdTtl:            durationpb.New(30 * time.Second),
		HeartbeatInterval:      durationpb.New(10 * time.Second),
		RedisPluginName:        "redis",
		RedisDb:                0,
	}

	// 将配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	const numGoroutines = 50
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
				id, err := suite.plugin.GenerateID()
				assert.NoError(suite.T(), err)
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 检查ID唯一性
	idSet := make(map[int64]bool)
	count := 0
	for id := range ids {
		assert.False(suite.T(), idSet[id], "ID should be unique: %d", id)
		idSet[id] = true
		count++
	}

	assert.Equal(suite.T(), totalIDs, count)
}

// TestMultipleInstances 测试多实例场景
func (suite *SnowflakeIntegrationTestSuite) TestMultipleInstances() {
	const numInstances = 5
	plugins := make([]*PlugSnowflake, numInstances)

	// 创建多个插件实例
	for i := 0; i < numInstances; i++ {
		plugin := NewSnowflakePlugin()
		plugin.runtime = suite.runtime
		plugin.logger = log.DefaultLogger

		config := &pb.Snowflake{
			WorkerId:               -1, // 自动分配
			DatacenterId:           1,
			AutoRegisterWorkerId:   true,
			RedisKeyPrefix:         "test:snowflake:multi",
			WorkerIdTtl:            durationpb.New(30 * time.Second),
			HeartbeatInterval:      durationpb.New(10 * time.Second),
			RedisPluginName:        "redis",
			RedisDb:                0,
		}

		// 将配置设置到mockConfig中
		suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

		err := plugin.Initialize(plugin, suite.runtime)
		require.NoError(suite.T(), err)

		err = plugin.Start(plugin)
		require.NoError(suite.T(), err)

		plugins[i] = plugin
	}

	// 验证每个实例都有不同的WorkerID
	workerIDs := make(map[int64]bool)
	for i, plugin := range plugins {
		generator := plugin.GetGenerator()
		require.NotNil(suite.T(), generator, "Instance %d generator should not be nil", i)

		stats := generator.GetStats()
		assert.False(suite.T(), workerIDs[stats.WorkerID], "WorkerID %d should be unique", stats.WorkerID)
		workerIDs[stats.WorkerID] = true
	}

	// 并发生成ID测试
	const idsPerInstance = 100
	allIDs := make(chan int64, numInstances*idsPerInstance)
	var wg sync.WaitGroup

	for i, plugin := range plugins {
		wg.Add(1)
		go func(p *PlugSnowflake, instanceID int) {
			defer wg.Done()
			for j := 0; j < idsPerInstance; j++ {
				id, err := p.GenerateID()
				assert.NoError(suite.T(), err, "Instance %d should generate ID successfully", instanceID)
				allIDs <- id
			}
		}(plugin, i)
	}

	wg.Wait()
	close(allIDs)

	// 验证所有ID唯一
	idSet := make(map[int64]bool)
	count := 0
	for id := range allIDs {
		assert.False(suite.T(), idSet[id], "ID should be unique across all instances: %d", id)
		idSet[id] = true
		count++
	}

	assert.Equal(suite.T(), numInstances*idsPerInstance, count)

	// 清理
	for _, plugin := range plugins {
		plugin.Stop(plugin)
	}
}

// TestHeartbeatMechanism 测试心跳机制
func (suite *SnowflakeIntegrationTestSuite) TestHeartbeatMechanism() {
	// 使用较短的心跳间隔进行测试
	config := &pb.Snowflake{
		WorkerId:               1,
		DatacenterId:           1,
		AutoRegisterWorkerId:   true,
		RedisKeyPrefix:         "test:snowflake:heartbeat",
		WorkerIdTtl:            durationpb.New(2 * time.Second),
		HeartbeatInterval:      durationpb.New(500 * time.Millisecond),
		RedisPluginName:        "redis",
		RedisDb:                0,
	}

	// 将配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// 等待几个心跳周期
	time.Sleep(3 * time.Second)

	// 验证插件仍然健康
	healthy, err := suite.plugin.HealthCheck()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), healthy)

	// 验证仍能生成ID
	id, err := suite.plugin.GenerateID()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id, int64(0))
}

// TestErrorHandling 测试错误处理
func (suite *SnowflakeIntegrationTestSuite) TestErrorHandling() {
	// 测试无效配置
	invalidConfig := &pb.Snowflake{
		WorkerId:     1024, // 超出范围
		DatacenterId: 1,
	}

	// 将无效配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = invalidConfig

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	assert.Error(suite.T(), err)

	// 测试未初始化状态下的操作
	plugin := NewSnowflakePlugin()
	_, err = plugin.GenerateID()
	assert.Error(suite.T(), err)

	_, _, err = plugin.GenerateIDWithMetadata()
	assert.Error(suite.T(), err)

	_, err = plugin.ParseID(123456)
	assert.Error(suite.T(), err)

	healthy, err := plugin.HealthCheck()
	assert.Error(suite.T(), err)
	assert.False(suite.T(), healthy)
}

// TestGlobalHelperFunctions 测试全局辅助函数
func (suite *SnowflakeIntegrationTestSuite) TestGlobalHelperFunctions() {
	// 初始化插件
	config := &pb.Snowflake{
		WorkerId:               1,
		DatacenterId:           1,
		AutoRegisterWorkerId:   true,
		RedisKeyPrefix:         "test:snowflake:global",
		WorkerIdTtl:            durationpb.New(30 * time.Second),
		HeartbeatInterval:      durationpb.New(10 * time.Second),
		RedisPluginName:        "redis",
		RedisDb:                0,
	}

	// 将配置设置到mockConfig中
	suite.runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// 模拟全局插件注册
	// 注意：在实际测试中，这需要与插件工厂集成
	// 这里我们直接测试插件方法

	// 测试生成ID
	id, err := suite.plugin.GenerateID()
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), id, int64(0))

	// 测试解析ID
	metadata, err := suite.plugin.ParseID(id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), metadata.WorkerID)
	assert.Equal(suite.T(), int64(1), metadata.DatacenterID)

	// 测试健康检查
	healthy, err := suite.plugin.HealthCheck()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), healthy)
}

// TestRunIntegrationTestSuite 运行集成测试套件
func TestRunIntegrationTestSuite(t *testing.T) {
	// 跳过如果没有Redis服务
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis server is not available, skipping integration tests")
		return
	}
	client.Close()

	suite.Run(t, new(SnowflakeIntegrationTestSuite))
}

// BenchmarkIntegrationIDGeneration 集成测试性能基准
func BenchmarkIntegrationIDGeneration(b *testing.B) {
	// 设置Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		b.Skip("Redis server is not available")
		return
	}

	// 清理测试数据
	redisClient.FlushDB(ctx)

	// 创建插件
	runtime := &mockRuntime{
		resources: make(map[string]interface{}),
	}
	runtime.RegisterResource("redis.client", redisClient)

	plugin := NewSnowflakePlugin()
	plugin.runtime = runtime
	plugin.logger = log.DefaultLogger

	config := &pb.Snowflake{
		WorkerId:               1,
		DatacenterId:           1,
		AutoRegisterWorkerId:   true,
		RedisKeyPrefix:         "bench:snowflake",
		WorkerIdTtl:            durationpb.New(30 * time.Second),
		HeartbeatInterval:      durationpb.New(10 * time.Second),
		RedisPluginName:        "redis",
		RedisDb:                0,
	}

	// 将配置设置到mockConfig中
	runtime.GetConfig().(*mockConfig).data["snowflake"] = config

	err := plugin.Initialize(plugin, runtime)
	require.NoError(b, err)

	err = plugin.Start(plugin)
	require.NoError(b, err)

	defer func() {
		plugin.Stop(plugin)
		redisClient.FlushDB(ctx)
		redisClient.Close()
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := plugin.GenerateID()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}