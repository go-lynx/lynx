package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisConnection 测试Redis连接
func TestRedisConnection(t *testing.T) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	
	ctx := context.Background()
	
	// 测试连接
	pong, err := client.Ping(ctx).Result()
	require.NoError(t, err)
	assert.Equal(t, "PONG", pong)
	
	// 清理测试数据
	defer client.FlushDB(ctx)
	
	// 测试基本操作
	t.Run("BasicOperations", func(t *testing.T) {
		// SET
		err := client.Set(ctx, "test_key", "test_value", time.Minute).Err()
		assert.NoError(t, err)
		
		// GET
		val, err := client.Get(ctx, "test_key").Result()
		assert.NoError(t, err)
		assert.Equal(t, "test_value", val)
		
		// EXISTS
		exists, err := client.Exists(ctx, "test_key").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), exists)
		
		// DEL
		deleted, err := client.Del(ctx, "test_key").Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), deleted)
	})
	
	// 测试过期功能
	t.Run("Expiration", func(t *testing.T) {
		err := client.Set(ctx, "expire_key", "value", 1*time.Second).Err()
		assert.NoError(t, err)
		
		// 立即检查
		val, err := client.Get(ctx, "expire_key").Result()
		assert.NoError(t, err)
		assert.Equal(t, "value", val)
		
		// 等待过期
		time.Sleep(2 * time.Second)
		
		// 验证已过期
		_, err = client.Get(ctx, "expire_key").Result()
		assert.Equal(t, redis.Nil, err)
	})
	
	// 测试事务
	t.Run("Transaction", func(t *testing.T) {
		// 初始化计数器
		client.Set(ctx, "counter", "0", 0)
		
		// 执行事务
		err := client.Watch(ctx, func(tx *redis.Tx) error {
			n, err := tx.Get(ctx, "counter").Int()
			if err != nil && err != redis.Nil {
				return err
			}
			
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, "counter", n+1, 0)
				return nil
			})
			return err
		}, "counter")
		
		assert.NoError(t, err)
		
		// 验证结果
		val, err := client.Get(ctx, "counter").Result()
		assert.NoError(t, err)
		assert.Equal(t, "1", val)
	})
	
	// 测试管道
	t.Run("Pipeline", func(t *testing.T) {
		pipe := client.Pipeline()
		
		// 批量设置
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pipe_key_%d", i)
			val := fmt.Sprintf("value_%d", i)
			pipe.Set(ctx, key, val, 0)
		}
		
		// 执行管道
		_, err := pipe.Exec(ctx)
		assert.NoError(t, err)
		
		// 验证结果
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pipe_key_%d", i)
			expectedVal := fmt.Sprintf("value_%d", i)
			
			val, err := client.Get(ctx, key).Result()
			assert.NoError(t, err)
			assert.Equal(t, expectedVal, val)
		}
		
		// 清理
		for i := 0; i < 10; i++ {
			client.Del(ctx, fmt.Sprintf("pipe_key_%d", i))
		}
	})
	
	// 关闭连接
	err = client.Close()
	assert.NoError(t, err)
}

// TestRedisHealthCheck 测试Redis健康检查
func TestRedisHealthCheck(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// 执行健康检查
	start := time.Now()
	_, err := client.Ping(ctx).Result()
	latency := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, latency, 100*time.Millisecond, "Ping latency should be less than 100ms")
	
	// 获取服务器信息
	info, err := client.Info(ctx, "server").Result()
	assert.NoError(t, err)
	assert.Contains(t, info, "redis_version")
	
	// 获取统计信息
	stats, err := client.Info(ctx, "stats").Result()
	assert.NoError(t, err)
	assert.Contains(t, stats, "total_commands_processed")
}

// TestRedisPerformance 测试Redis性能
func TestRedisPerformance(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
	})
	defer client.Close()
	
	ctx := context.Background()
	defer client.FlushDB(ctx)
	
	// 性能测试：批量写入
	t.Run("BulkWrite", func(t *testing.T) {
		start := time.Now()
		pipe := client.Pipeline()
		
		count := 1000
		for i := 0; i < count; i++ {
			key := fmt.Sprintf("perf_key_%d", i)
			val := fmt.Sprintf("value_%d", i)
			pipe.Set(ctx, key, val, 0)
		}
		
		_, err := pipe.Exec(ctx)
		require.NoError(t, err)
		
		elapsed := time.Since(start)
		opsPerSec := float64(count) / elapsed.Seconds()
		
		t.Logf("Bulk write performance: %d operations in %v (%.2f ops/sec)", 
			count, elapsed, opsPerSec)
		
		// 验证性能阈值
		assert.Greater(t, opsPerSec, 1000.0, "Should achieve at least 1000 ops/sec")
	})
	
	// 性能测试：并发读取
	t.Run("ConcurrentRead", func(t *testing.T) {
		// 准备测试数据
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("read_key_%d", i)
			val := fmt.Sprintf("value_%d", i)
			client.Set(ctx, key, val, 0)
		}
		
		start := time.Now()
		done := make(chan struct{})
		workers := 10
		readsPerWorker := 100
		
		for w := 0; w < workers; w++ {
			go func(workerID int) {
				for i := 0; i < readsPerWorker; i++ {
					key := fmt.Sprintf("read_key_%d", i%100)
					client.Get(ctx, key)
				}
				done <- struct{}{}
			}(w)
		}
		
		// 等待所有worker完成
		for w := 0; w < workers; w++ {
			<-done
		}
		
		elapsed := time.Since(start)
		totalReads := workers * readsPerWorker
		opsPerSec := float64(totalReads) / elapsed.Seconds()
		
		t.Logf("Concurrent read performance: %d operations in %v (%.2f ops/sec)", 
			totalReads, elapsed, opsPerSec)
		
		// 验证性能阈值
		assert.Greater(t, opsPerSec, 5000.0, "Should achieve at least 5000 ops/sec")
	})
}