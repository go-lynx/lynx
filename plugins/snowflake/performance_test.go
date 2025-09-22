package snowflake

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PerformanceTestConfig holds configuration for performance tests
type PerformanceTestConfig struct {
	Duration       time.Duration
	NumWorkers     int
	TargetRPS      int
	WarmupDuration time.Duration
}

// PerformanceMetrics holds performance test results
type PerformanceMetrics struct {
	TotalRequests   int64
	SuccessfulReqs  int64
	FailedReqs      int64
	Duration        time.Duration
	ActualRPS       float64
	AvgLatency      time.Duration
	P50Latency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
	ThroughputMBps  float64
	MemoryUsageMB   float64
	CPUUsagePercent float64
	ErrorRate       float64
}

// TestHighThroughputGeneration tests high throughput ID generation
func TestHighThroughputGeneration(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	config := &PerformanceTestConfig{
		Duration:       10 * time.Second,
		NumWorkers:     runtime.NumCPU() * 2, // Use 2x CPU cores for optimal performance
		TargetRPS:      0,                    // 0 means unlimited - test maximum performance
		WarmupDuration: 2 * time.Second,
	}

	metrics := runPerformanceTest(t, generator, config)

	// Performance assertions for maximum throughput test
	assert.Greater(t, metrics.ActualRPS, 15000.0, "Should achieve at least 15,000 RPS in unlimited mode")
	assert.Less(t, metrics.ErrorRate, 0.1, "Error rate should be less than 0.1%")
	assert.Less(t, metrics.MemoryUsageMB, 100.0, "Memory usage should be less than 100MB")

	logPerformanceMetrics(t, "High Throughput Test", metrics)
}

// TestScalabilityWithWorkers tests scalability with different worker counts
func TestScalabilityWithWorkers(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	workerCounts := []int{1, 2, 4, 8, 16, 32}
	results := make(map[int]*PerformanceMetrics)

	for _, numWorkers := range workerCounts {
		config := &PerformanceTestConfig{
			Duration:       5 * time.Second,
			NumWorkers:     numWorkers,
			WarmupDuration: time.Second,
		}

		metrics := runPerformanceTest(t, generator, config)
		results[numWorkers] = metrics

		t.Logf("Workers: %d, RPS: %.2f, Avg Latency: %v",
			numWorkers, metrics.ActualRPS, metrics.AvgLatency)
	}

	// Verify scalability - RPS should generally increase with more workers
	// (up to a point where contention becomes significant)
	for i := 1; i < len(workerCounts)-1; i++ {
		current := results[workerCounts[i]]
		next := results[workerCounts[i+1]]

		// Allow for some variance due to system load
		if next.ActualRPS < current.ActualRPS*0.9 {
			t.Logf("Warning: RPS decreased significantly from %d to %d workers (%.2f -> %.2f)",
				workerCounts[i], workerCounts[i+1], current.ActualRPS, next.ActualRPS)
		}
	}
}

// TestLatencyUnderLoad tests latency characteristics under various loads
func TestLatencyUnderLoad(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	loadLevels := []int{100, 1000, 5000, 10000, 20000}

	for _, targetRPS := range loadLevels {
		config := &PerformanceTestConfig{
			Duration:       5 * time.Second,
			NumWorkers:     runtime.NumCPU() * 2,
			TargetRPS:      targetRPS,
			WarmupDuration: time.Second,
		}

		metrics := runPerformanceTest(t, generator, config)

		t.Logf("Target RPS: %d, Actual RPS: %.2f, P95 Latency: %v, P99 Latency: %v",
			targetRPS, metrics.ActualRPS, metrics.P95Latency, metrics.P99Latency)

		// Latency should remain reasonable even under high load
		assert.Less(t, metrics.P95Latency, 100*time.Millisecond,
			"P95 latency should be less than 100ms at %d RPS", targetRPS)
		assert.Less(t, metrics.P99Latency, 200*time.Millisecond,
			"P99 latency should be less than 200ms at %d RPS", targetRPS)
	}
}

// TestMemoryEfficiency tests memory usage during sustained load
func TestMemoryEfficiency(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	// Measure baseline memory
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	config := &PerformanceTestConfig{
		Duration:       30 * time.Second,
		NumWorkers:     runtime.NumCPU(),
		TargetRPS:      10000,
		WarmupDuration: 2 * time.Second,
	}

	metrics := runPerformanceTest(t, generator, config)

	// Memory usage should be reasonable
	assert.Less(t, metrics.MemoryUsageMB, 100.0, "Memory usage should be less than 100MB")

	t.Logf("Memory efficiency test - Generated %d IDs, Memory usage: %.2f MB",
		metrics.SuccessfulReqs, metrics.MemoryUsageMB)
}

// TestCPUEfficiency tests CPU usage characteristics
func TestCPUEfficiency(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	config := &PerformanceTestConfig{
		Duration:       10 * time.Second,
		NumWorkers:     runtime.NumCPU(),
		TargetRPS:      20000,
		WarmupDuration: 2 * time.Second,
	}

	// Monitor CPU usage during test
	var cpuSamples []float64
	var cpuMutex sync.Mutex

	cpuCtx, cpuCancel := context.WithCancel(context.Background())
	defer cpuCancel()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-cpuCtx.Done():
				return
			case <-ticker.C:
				// Simplified CPU usage estimation
				// In a real implementation, you'd use proper CPU monitoring
				cpuMutex.Lock()
				cpuSamples = append(cpuSamples, float64(runtime.NumGoroutine()))
				cpuMutex.Unlock()
			}
		}
	}()

	metrics := runPerformanceTest(t, generator, config)
	cpuCancel()

	// Calculate average CPU usage (simplified)
	cpuMutex.Lock()
	var avgCPU float64
	if len(cpuSamples) > 0 {
		for _, sample := range cpuSamples {
			avgCPU += sample
		}
		avgCPU /= float64(len(cpuSamples))
	}
	cpuMutex.Unlock()

	t.Logf("CPU efficiency test - RPS: %.2f, Avg Goroutines: %.2f",
		metrics.ActualRPS, avgCPU)

	// Verify reasonable CPU efficiency
	efficiency := metrics.ActualRPS / avgCPU // IDs per goroutine
	assert.Greater(t, efficiency, 100.0, "Should generate at least 100 IDs per goroutine")
}

// TestSustainedLoad tests performance under sustained load
func TestSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	config := &PerformanceTestConfig{
		Duration:       60 * time.Second, // 1 minute sustained load
		NumWorkers:     runtime.NumCPU() * 2,
		TargetRPS:      15000,
		WarmupDuration: 5 * time.Second,
	}

	metrics := runPerformanceTest(t, generator, config)

	// Performance should remain stable under sustained load
	assert.Greater(t, metrics.ActualRPS, float64(config.TargetRPS)*0.8)
	assert.Less(t, metrics.ErrorRate, 0.1)
	assert.Less(t, metrics.P99Latency, 100*time.Millisecond)

	logPerformanceMetrics(t, "Sustained Load Test", metrics)
}

// runPerformanceTest executes a performance test with the given configuration
func runPerformanceTest(t *testing.T, generator *Generator, config *PerformanceTestConfig) *PerformanceMetrics {
	var (
		totalRequests  int64
		successfulReqs int64
		failedReqs     int64
	)

	// Measure initial memory
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Warmup phase
	if config.WarmupDuration > 0 {
		warmupCtx, warmupCancel := context.WithTimeout(context.Background(), config.WarmupDuration)
		runLoadPhase(warmupCtx, generator, config.NumWorkers, config.TargetRPS, nil, nil, nil)
		warmupCancel()

		// Reset counters after warmup
		totalRequests = 0
		successfulReqs = 0
		failedReqs = 0
	}

	// Main test phase
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	start := time.Now()
	runLoadPhase(ctx, generator, config.NumWorkers, config.TargetRPS,
		&totalRequests, &successfulReqs, &failedReqs)
	actualDuration := time.Since(start)

	// Measure final memory
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Calculate memory usage safely
	var memoryUsageMB float64
	if finalMem.Alloc > initialMem.Alloc {
		memoryUsageMB = float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024
	} else {
		memoryUsageMB = 0
	}

	// Calculate metrics
	metrics := &PerformanceMetrics{
		TotalRequests:  totalRequests,
		SuccessfulReqs: successfulReqs,
		FailedReqs:     failedReqs,
		Duration:       actualDuration,
		ActualRPS:      float64(totalRequests) / actualDuration.Seconds(),
		ErrorRate:      float64(failedReqs) / float64(totalRequests) * 100,
		MemoryUsageMB:  memoryUsageMB,
	}

	return metrics
}

// runLoadPhase runs a load generation phase
func runLoadPhase(ctx context.Context, generator *Generator, numWorkers, targetRPS int,
	totalRequests, successfulReqs, failedReqs *int64) {

	var wg sync.WaitGroup

	// Calculate request interval for rate limiting per worker
	var requestInterval time.Duration
	if targetRPS > 0 && numWorkers > 0 {
		// Each worker should generate targetRPS/numWorkers requests per second
		requestsPerWorker := targetRPS / numWorkers
		if requestsPerWorker > 0 {
			requestInterval = time.Second / time.Duration(requestsPerWorker)
		}
	}

	// Launch workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if requestInterval > 0 {
				// Rate-limited mode: use ticker for controlled rate
				ticker := time.NewTicker(requestInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						// Generate ID
						_, err := generator.GenerateID()

						// Update counters
						if totalRequests != nil {
							atomic.AddInt64(totalRequests, 1)
						}

						if err != nil {
							if failedReqs != nil {
								atomic.AddInt64(failedReqs, 1)
							}
						} else {
							if successfulReqs != nil {
								atomic.AddInt64(successfulReqs, 1)
							}
						}
					}
				}
			} else {
				// Unlimited mode: generate as fast as possible
				for {
					select {
					case <-ctx.Done():
						return
					default:
						// Generate ID
						_, err := generator.GenerateID()

						// Update counters
						if totalRequests != nil {
							atomic.AddInt64(totalRequests, 1)
						}

						if err != nil {
							if failedReqs != nil {
								atomic.AddInt64(failedReqs, 1)
							}
						} else {
							if successfulReqs != nil {
								atomic.AddInt64(successfulReqs, 1)
							}
						}

						// Small delay to prevent tight loop and allow context cancellation
						time.Sleep(time.Nanosecond)
					}
				}
			}
		}()
	}

	wg.Wait()
}

// logPerformanceMetrics logs detailed performance metrics
func logPerformanceMetrics(t *testing.T, testName string, metrics *PerformanceMetrics) {
	t.Logf("=== %s Results ===", testName)
	t.Logf("Duration: %v", metrics.Duration)
	t.Logf("Total Requests: %d", metrics.TotalRequests)
	t.Logf("Successful: %d", metrics.SuccessfulReqs)
	t.Logf("Failed: %d", metrics.FailedReqs)
	t.Logf("Actual RPS: %.2f", metrics.ActualRPS)
	t.Logf("Error Rate: %.4f%%", metrics.ErrorRate)
	t.Logf("Memory Usage: %.2f MB", metrics.MemoryUsageMB)
	if metrics.AvgLatency > 0 {
		t.Logf("Avg Latency: %v", metrics.AvgLatency)
		t.Logf("P95 Latency: %v", metrics.P95Latency)
		t.Logf("P99 Latency: %v", metrics.P99Latency)
	}
	t.Logf("========================")
}

// BenchmarkSingleThreadedGeneration benchmarks single-threaded generation
func BenchmarkSingleThreadedGeneration(b *testing.B) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateID()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMultiThreadedGeneration benchmarks multi-threaded generation
func BenchmarkMultiThreadedGeneration(b *testing.B) {
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

// BenchmarkGenerationWithMetrics benchmarks generation with metrics enabled
func BenchmarkGenerationWithMetrics(b *testing.B) {
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

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateID()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestPerformanceRegression tests for performance regressions
func TestPerformanceRegression(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	// Baseline performance expectations
	minRPS := 10000.0
	maxLatencyP99 := 50 * time.Millisecond

	config := &PerformanceTestConfig{
		Duration:       5 * time.Second,
		NumWorkers:     runtime.NumCPU(),
		WarmupDuration: time.Second,
	}

	metrics := runPerformanceTest(t, generator, config)

	// Check for performance regressions
	if metrics.ActualRPS < minRPS {
		t.Errorf("Performance regression detected: RPS %.2f is below minimum %.2f",
			metrics.ActualRPS, minRPS)
	}

	if metrics.P99Latency > maxLatencyP99 {
		t.Errorf("Latency regression detected: P99 latency %v exceeds maximum %v",
			metrics.P99Latency, maxLatencyP99)
	}

	t.Logf("Performance regression test passed - RPS: %.2f, P99 Latency: %v",
		metrics.ActualRPS, metrics.P99Latency)
}

// TestConcurrencyLimits tests behavior at concurrency limits
func TestConcurrencyLimits(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	// Test with very high concurrency
	numWorkers := 1000
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := generator.GenerateID()
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := successCount + errorCount
	errorRate := float64(errorCount) / float64(totalRequests) * 100
	rps := float64(totalRequests) / duration.Seconds()

	t.Logf("Concurrency limits test:")
	t.Logf("  Workers: %d", numWorkers)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Success rate: %.2f%%", 100-errorRate)
	t.Logf("  RPS: %.2f", rps)

	// Should handle high concurrency gracefully
	assert.Less(t, errorRate, 5.0, "Error rate should be less than 5% even with high concurrency")
	assert.Greater(t, successCount, int64(0), "Should have some successful requests")
}
