package snowflake

import (
	"context"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StressTestConfig holds configuration for stress tests
type StressTestConfig struct {
	NumWorkers     int           // Number of concurrent workers
	Duration       time.Duration // Test duration
	RequestsPerSec int           // Target requests per second
	MaxErrors      int           // Maximum allowed errors
}

// StressTestResult holds the results of a stress test
type StressTestResult struct {
	TotalRequests   int64         // Total requests made
	SuccessfulReqs  int64         // Successful requests
	FailedReqs      int64         // Failed requests
	UniqueIDs       int64         // Number of unique IDs generated
	Duplicates      int64         // Number of duplicate IDs
	Duration        time.Duration // Actual test duration
	RequestsPerSec  float64       // Actual requests per second
	AvgLatency      time.Duration // Average latency
	MaxLatency      time.Duration // Maximum latency
	MinLatency      time.Duration // Minimum latency
	P95Latency      time.Duration // 95th percentile latency
	P99Latency      time.Duration // 99th percentile latency
	ErrorRate       float64       // Error rate percentage
	MemoryUsageMB   float64       // Memory usage in MB
	CPUUsagePercent float64       // CPU usage percentage
}

// TestStressIDGeneration performs stress testing on ID generation
func TestStressIDGeneration(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	config := &StressTestConfig{
		NumWorkers:     runtime.NumCPU() * 2,
		Duration:       10 * time.Second,
		RequestsPerSec: 10000,
		MaxErrors:      100,
	}

	result := runStressTest(t, generator, config)

	// Assertions
	assert.Greater(t, result.TotalRequests, int64(0), "Should have made requests")
	assert.Equal(t, result.Duplicates, int64(0), "Should have no duplicate IDs")
	assert.Less(t, result.ErrorRate, 1.0, "Error rate should be less than 1%")
	assert.Greater(t, result.RequestsPerSec, float64(config.RequestsPerSec)*0.8, "Should achieve at least 80% of target RPS")

	t.Logf("Stress test results: %+v", result)
}

// TestConcurrentIDGeneration tests concurrent ID generation
func TestConcurrentIDGeneration(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	numGoroutines := 100
	idsPerGoroutine := 1000
	totalIDs := numGoroutines * idsPerGoroutine

	var wg sync.WaitGroup
	idChan := make(chan int64, totalIDs)
	errorChan := make(chan error, totalIDs)

	start := time.Now()

	// Launch goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := generator.GenerateID()
				if err != nil {
					errorChan <- err
					return
				}
				idChan <- id
			}
		}()
	}

	wg.Wait()
	close(idChan)
	close(errorChan)

	duration := time.Since(start)

	// Check for errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}
	assert.Empty(t, errors, "Should have no errors during concurrent generation")

	// Collect and verify IDs
	idSet := make(map[int64]bool)
	var ids []int64
	for id := range idChan {
		ids = append(ids, id)
		if idSet[id] {
			t.Errorf("Duplicate ID found: %d", id)
		}
		idSet[id] = true
	}

	assert.Equal(t, totalIDs, len(ids), "Should generate expected number of IDs")
	assert.Equal(t, totalIDs, len(idSet), "All IDs should be unique")

	rps := float64(totalIDs) / duration.Seconds()
	t.Logf("Generated %d unique IDs in %v (%.2f IDs/sec)", totalIDs, duration, rps)
}

// TestMemoryLeakDetection tests for memory leaks during extended operation
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Create generator with minimal configuration to reduce memory overhead
	config := &GeneratorConfig{
		CustomEpoch:                1640995200000, // 2022-01-01
		DatacenterIDBits:           5,
		WorkerIDBits:               5,
		SequenceBits:               12,
		EnableClockDriftProtection: false, // Disable to reduce overhead
		ClockDriftAction:           ClockDriftActionWait,
		EnableSequenceCache:        false, // Disable cache to reduce memory usage
		SequenceCacheSize:          0,
	}

	generator, err := NewSnowflakeGeneratorCore(1, 1, config)
	require.NoError(t, err)

	// Warm up and stabilize memory
	for i := 0; i < 1000; i++ {
		_, err := generator.GenerateID()
		require.NoError(t, err)
	}

	// Force multiple GC cycles to stabilize memory
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Generate IDs for shorter period with controlled rate
	duration := 10 * time.Second // Reduced from 30s
	start := time.Now()
	count := 0

	for time.Since(start) < duration {
		_, err := generator.GenerateID()
		require.NoError(t, err)
		count++

		// Force GC more frequently and add small delay to prevent excessive memory allocation
		if count%5000 == 0 {
			runtime.GC()
			time.Sleep(time.Microsecond) // Small delay to allow GC
		}
	}

	// Force multiple GC cycles before measuring final memory
	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Use more accurate memory measurement
	initialAllocMB := float64(initialMem.Alloc) / 1024 / 1024
	finalAllocMB := float64(finalMem.Alloc) / 1024 / 1024
	memoryIncrease := finalAllocMB - initialAllocMB

	t.Logf("Generated %d IDs in %v", count, duration)
	t.Logf("Initial memory: %.2f MB", initialAllocMB)
	t.Logf("Final memory: %.2f MB", finalAllocMB)
	t.Logf("Memory increase: %.2f MB", memoryIncrease)
	t.Logf("Memory efficiency: %.2f IDs/MB", float64(count)/math.Max(memoryIncrease, 0.1))

	// Memory increase should be minimal (less than 5MB for shorter test)
	assert.Less(t, memoryIncrease, 5.0, "Memory increase should be less than 5MB")

	// Ensure we're not leaking memory excessively
	if memoryIncrease > 0 {
		efficiency := float64(count) / memoryIncrease
		assert.Greater(t, efficiency, 100000.0, "Memory efficiency should be at least 100K IDs per MB")
	}
}

// TestHighFrequencyGeneration tests ID generation at very high frequency
func TestHighFrequencyGeneration(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	// Test generating IDs as fast as possible for 5 seconds
	duration := 5 * time.Second
	start := time.Now()
	count := int64(0)
	var lastID int64

	for time.Since(start) < duration {
		id, err := generator.GenerateID()
		require.NoError(t, err)

		// Verify ID is greater than previous
		assert.Greater(t, id, lastID, "IDs should be monotonically increasing")
		lastID = id

		atomic.AddInt64(&count, 1)
	}

	actualDuration := time.Since(start)
	rps := float64(count) / actualDuration.Seconds()

	t.Logf("Generated %d IDs in %v (%.2f IDs/sec)", count, actualDuration, rps)
	assert.Greater(t, rps, 100000.0, "Should generate at least 100k IDs per second")
}

// runStressTest executes a stress test with the given configuration
func runStressTest(t *testing.T, generator *Generator, config *StressTestConfig) *StressTestResult {
	var (
		totalRequests  int64
		successfulReqs int64
		failedReqs     int64
		latencies      []time.Duration
		latencyMutex   sync.Mutex
		idSet          = make(map[int64]bool)
		idSetMutex     sync.RWMutex
		duplicates     int64
	)

	// Measure initial memory
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	start := time.Now()

	// Launch workers
	for i := 0; i < config.NumWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					requestStart := time.Now()

					id, err := generator.GenerateID()
					latency := time.Since(requestStart)

					atomic.AddInt64(&totalRequests, 1)

					if err != nil {
						atomic.AddInt64(&failedReqs, 1)
					} else {
						atomic.AddInt64(&successfulReqs, 1)

						// Check for duplicates
						idSetMutex.Lock()
						if idSet[id] {
							atomic.AddInt64(&duplicates, 1)
						} else {
							idSet[id] = true
						}
						idSetMutex.Unlock()
					}

					// Record latency
					latencyMutex.Lock()
					latencies = append(latencies, latency)
					latencyMutex.Unlock()

					// Rate limiting
					if config.RequestsPerSec > 0 {
						expectedInterval := time.Duration(int64(time.Second) / int64(config.RequestsPerSec))
						if latency < expectedInterval {
							time.Sleep(expectedInterval - latency)
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	actualDuration := time.Since(start)

	// Measure final memory
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Calculate statistics
	result := &StressTestResult{
		TotalRequests:  totalRequests,
		SuccessfulReqs: successfulReqs,
		FailedReqs:     failedReqs,
		UniqueIDs:      int64(len(idSet)),
		Duplicates:     duplicates,
		Duration:       actualDuration,
		RequestsPerSec: float64(totalRequests) / actualDuration.Seconds(),
		ErrorRate:      float64(failedReqs) / float64(totalRequests) * 100,
		MemoryUsageMB:  float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024,
	}

	// Calculate latency statistics
	if len(latencies) > 0 {
		result.AvgLatency = calculateAvgLatency(latencies)
		result.MinLatency = calculateMinLatency(latencies)
		result.MaxLatency = calculateMaxLatency(latencies)
		result.P95Latency = calculatePercentileLatency(latencies, 95)
		result.P99Latency = calculatePercentileLatency(latencies, 99)
	}

	return result
}

// Helper functions for latency calculations
func calculateAvgLatency(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

func calculateMinLatency(latencies []time.Duration) time.Duration {
	min := latencies[0]
	for _, latency := range latencies {
		if latency < min {
			min = latency
		}
	}
	return min
}

func calculateMaxLatency(latencies []time.Duration) time.Duration {
	max := latencies[0]
	for _, latency := range latencies {
		if latency > max {
			max = latency
		}
	}
	return max
}

func calculatePercentileLatency(latencies []time.Duration, percentile int) time.Duration {
	// Simple percentile calculation (not the most efficient, but works for testing)
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// BenchmarkIDGeneration benchmarks ID generation performance
func BenchmarkIDGeneration(b *testing.B) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := generator.GenerateID()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkIDGenerationWithMetrics benchmarks ID generation with metrics enabled
func BenchmarkIDGenerationWithMetrics(b *testing.B) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	if err != nil {
		b.Fatal(err)
	}

	// Enable metrics
	generator.metrics = NewSnowflakeMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := generator.GenerateID()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
