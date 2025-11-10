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

// TestRedisConnection tests Redis connectivity
func TestRedisConnection(t *testing.T) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	ctx := context.Background()

	// Test connection
	pong, err := client.Ping(ctx).Result()
	require.NoError(t, err)
	assert.Equal(t, "PONG", pong)

	// Cleanup test data
	defer client.FlushDB(ctx)

	// Test basic operations
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

	// Test expiration
	t.Run("Expiration", func(t *testing.T) {
		err := client.Set(ctx, "expire_key", "value", 1*time.Second).Err()
		assert.NoError(t, err)

		// Check immediately
		val, err := client.Get(ctx, "expire_key").Result()
		assert.NoError(t, err)
		assert.Equal(t, "value", val)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Verify expired
		_, err = client.Get(ctx, "expire_key").Result()
		assert.Equal(t, redis.Nil, err)
	})

	// Test transaction
	t.Run("Transaction", func(t *testing.T) {
		// Initialize counter
		client.Set(ctx, "counter", "0", 0)

		// Execute transaction
		err := client.Watch(ctx, func(tx *redis.Tx) error {
			n, getErr := tx.Get(ctx, "counter").Int()
			if getErr != nil && getErr != redis.Nil {
				return getErr
			}

			_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, "counter", n+1, 0)
				return nil
			})
			return txErr
		}, "counter")

		assert.NoError(t, err)

		// Verify result
		val, err := client.Get(ctx, "counter").Result()
		assert.NoError(t, err)
		assert.Equal(t, "1", val)
	})

	// Test pipeline
	t.Run("Pipeline", func(t *testing.T) {
		pipe := client.Pipeline()

		// Bulk set
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pipe_key_%d", i)
			val := fmt.Sprintf("value_%d", i)
			pipe.Set(ctx, key, val, 0)
		}

		// Execute pipeline
		_, err := pipe.Exec(ctx)
		assert.NoError(t, err)

		// Verify results
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("pipe_key_%d", i)
			expectedVal := fmt.Sprintf("value_%d", i)

			val, err := client.Get(ctx, key).Result()
			assert.NoError(t, err)
			assert.Equal(t, expectedVal, val)
		}

		// Cleanup
		for i := 0; i < 10; i++ {
			client.Del(ctx, fmt.Sprintf("pipe_key_%d", i))
		}
	})

	// Close connection
	err = client.Close()
	assert.NoError(t, err)
}

// TestRedisHealthCheck tests Redis health check
func TestRedisHealthCheck(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Perform health check
	start := time.Now()
	_, err := client.Ping(ctx).Result()
	latency := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, latency, 100*time.Millisecond, "Ping latency should be less than 100ms")

	// Get server info
	info, err := client.Info(ctx, "server").Result()
	assert.NoError(t, err)
	assert.Contains(t, info, "redis_version")

	// Get stats
	stats, err := client.Info(ctx, "stats").Result()
	assert.NoError(t, err)
	assert.Contains(t, stats, "total_commands_processed")
}

// TestRedisPerformance tests Redis performance
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

	// Performance test: bulk write
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

		// Validate performance threshold
		assert.Greater(t, opsPerSec, 1000.0, "Should achieve at least 1000 ops/sec")
	})

	// Performance test: concurrent read
	t.Run("ConcurrentRead", func(t *testing.T) {
		// Prepare test data
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

		// Wait for all workers to finish
		for w := 0; w < workers; w++ {
			<-done
		}

		elapsed := time.Since(start)
		totalReads := workers * readsPerWorker
		opsPerSec := float64(totalReads) / elapsed.Seconds()

		t.Logf("Concurrent read performance: %d operations in %v (%.2f ops/sec)",
			totalReads, elapsed, opsPerSec)

		// Validate performance threshold
		assert.Greater(t, opsPerSec, 5000.0, "Should achieve at least 5000 ops/sec")
	})
}
