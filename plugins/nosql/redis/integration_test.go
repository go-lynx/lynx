package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"
)

// RedisIntegrationTestSuite 定义Redis集成测试套件
type RedisIntegrationTestSuite struct {
	suite.Suite
	plugin  *PlugRedis
	client  redis.UniversalClient
	ctx     context.Context
	cancel  context.CancelFunc
	runtime *mockRuntime
}

// mockRuntime 模拟Runtime实现
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

// SetupSuite 设置测试套件
func (suite *RedisIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.runtime = &mockRuntime{
		resources: make(map[string]interface{}),
	}

	// 创建Redis插件
	suite.plugin = NewRedisClient()

	// 设置配置
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

	// 初始化插件
	err := suite.plugin.Initialize(suite.plugin, suite.runtime)
	require.NoError(suite.T(), err)

	// 启动插件
	err = suite.plugin.Start(suite.plugin)
	require.NoError(suite.T(), err)

	// 获取客户端
	suite.client = suite.plugin.rdb
	require.NotNil(suite.T(), suite.client)

	// 清理测试数据
	suite.client.FlushDB(suite.ctx)
}

// TearDownSuite 清理测试套件
func (suite *RedisIntegrationTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.FlushDB(suite.ctx)
	}
	if suite.plugin != nil {
		suite.plugin.Stop(suite.plugin)
	}
	suite.cancel()
}

// TestBasicOperations 测试基本操作
func (suite *RedisIntegrationTestSuite) TestBasicOperations() {
	ctx := suite.ctx
	client := suite.client

	// 测试SET和GET
	err := client.Set(ctx, "test_key", "test_value", time.Minute).Err()
	assert.NoError(suite.T(), err)

	val, err := client.Get(ctx, "test_key").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test_value", val)

	// 测试DEL
	err = client.Del(ctx, "test_key").Err()
	assert.NoError(suite.T(), err)

	// 验证删除
	_, err = client.Get(ctx, "test_key").Result()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), redis.Nil, err)
}

// TestHashOperations 测试Hash操作
func (suite *RedisIntegrationTestSuite) TestHashOperations() {
	ctx := suite.ctx
	client := suite.client

	// 测试HSET和HGET
	err := client.HSet(ctx, "test_hash", "field1", "value1").Err()
	assert.NoError(suite.T(), err)

	val, err := client.HGet(ctx, "test_hash", "field1").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value1", val)

	// 测试HMSET和HMGET
	err = client.HMSet(ctx, "test_hash", map[string]interface{}{
		"field2": "value2",
		"field3": "value3",
	}).Err()
	assert.NoError(suite.T(), err)

	vals, err := client.HMGet(ctx, "test_hash", "field2", "field3").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []interface{}{"value2", "value3"}, vals)

	// 测试HGETALL
	all, err := client.HGetAll(ctx, "test_hash").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), map[string]string{
		"field1": "value1",
		"field2": "value2",
		"field3": "value3",
	}, all)
}

// TestListOperations 测试List操作
func (suite *RedisIntegrationTestSuite) TestListOperations() {
	ctx := suite.ctx
	client := suite.client

	// 测试LPUSH和RPUSH
	err := client.LPush(ctx, "test_list", "left1", "left2").Err()
	assert.NoError(suite.T(), err)

	err = client.RPush(ctx, "test_list", "right1", "right2").Err()
	assert.NoError(suite.T(), err)

	// 测试LLEN
	length, err := client.LLen(ctx, "test_list").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(4), length)

	// 测试LRANGE
	vals, err := client.LRange(ctx, "test_list", 0, -1).Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []string{"left2", "left1", "right1", "right2"}, vals)
}

// TestSetOperations 测试Set操作
func (suite *RedisIntegrationTestSuite) TestSetOperations() {
	ctx := suite.ctx
	client := suite.client

	// 测试SADD
	err := client.SAdd(ctx, "test_set", "member1", "member2", "member3").Err()
	assert.NoError(suite.T(), err)

	// 测试SISMEMBER
	exists, err := client.SIsMember(ctx, "test_set", "member2").Result()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exists)

	// 测试SMEMBERS
	members, err := client.SMembers(ctx, "test_set").Result()
	assert.NoError(suite.T(), err)
	assert.ElementsMatch(suite.T(), []string{"member1", "member2", "member3"}, members)

	// 测试SREM
	err = client.SRem(ctx, "test_set", "member2").Err()
	assert.NoError(suite.T(), err)

	exists, err = client.SIsMember(ctx, "test_set", "member2").Result()
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exists)
}

// TestExpiration 测试过期功能
func (suite *RedisIntegrationTestSuite) TestExpiration() {
	ctx := suite.ctx
	client := suite.client

	// 设置带过期时间的键
	err := client.Set(ctx, "expire_key", "value", 2*time.Second).Err()
	assert.NoError(suite.T(), err)

	// 立即检查
	val, err := client.Get(ctx, "expire_key").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "value", val)

	// 等待过期
	time.Sleep(3 * time.Second)

	// 验证已过期
	_, err = client.Get(ctx, "expire_key").Result()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), redis.Nil, err)
}

// TestTransactions 测试事务
func (suite *RedisIntegrationTestSuite) TestTransactions() {
	ctx := suite.ctx
	client := suite.client

	// 使用WATCH和事务
	key := "counter"
	client.Set(ctx, key, "0", 0)

	// 执行事务
	err := client.Watch(ctx, func(tx *redis.Tx) error {
		n, err := tx.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			return err
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, n+1, 0)
			return nil
		})
		return err
	}, key)

	assert.NoError(suite.T(), err)

	// 验证结果
	val, err := client.Get(ctx, key).Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "1", val)
}

// TestPipelining 测试管道
func (suite *RedisIntegrationTestSuite) TestPipelining() {
	ctx := suite.ctx
	client := suite.client

	// 使用管道批量执行命令
	pipe := client.Pipeline()

	for i := 0; i < 10; i++ {
		pipe.Set(ctx, fmt.Sprintf("pipe_key_%d", i), fmt.Sprintf("value_%d", i), 0)
	}

	// 执行管道
	_, err := pipe.Exec(ctx)
	assert.NoError(suite.T(), err)

	// 验证结果
	for i := 0; i < 10; i++ {
		val, err := client.Get(ctx, fmt.Sprintf("pipe_key_%d", i)).Result()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), fmt.Sprintf("value_%d", i), val)
	}
}

// TestConcurrency 测试并发安全性
func (suite *RedisIntegrationTestSuite) TestConcurrency() {
	ctx := suite.ctx
	client := suite.client

	// 设置初始值
	client.Set(ctx, "concurrent_counter", "0", 0)

	// 并发增加计数器
	var wg sync.WaitGroup
	var successCount int32
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 使用Lua脚本保证原子性
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

	// 验证结果
	val, err := client.Get(ctx, "concurrent_counter").Result()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf("%d", successCount), val)
	assert.Equal(suite.T(), int32(iterations), successCount)
}

// TestConnectionPoolMetrics 测试连接池指标
func (suite *RedisIntegrationTestSuite) TestConnectionPoolMetrics() {
	// 获取连接池统计
	stats := suite.plugin.GetPoolStats()
	assert.NotNil(suite.T(), stats)

	// 验证指标
	assert.GreaterOrEqual(suite.T(), stats.TotalConns, uint32(0))
	assert.GreaterOrEqual(suite.T(), stats.IdleConns, uint32(0))
	assert.GreaterOrEqual(suite.T(), stats.Hits, uint64(0))
	assert.GreaterOrEqual(suite.T(), stats.Timeouts, uint64(0))
}

// TestHealthCheck 测试健康检查
func (suite *RedisIntegrationTestSuite) TestHealthCheck() {
	// 执行健康检查
	healthy, err := suite.plugin.HealthCheck()
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), healthy)
}

// TestRunIntegrationTestSuite 运行测试套件
func TestRunIntegrationTestSuite(t *testing.T) {
	// 跳过如果没有Redis服务
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
