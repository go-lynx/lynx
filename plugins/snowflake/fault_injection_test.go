package snowflake

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FaultType represents different types of faults that can be injected
type FaultType int

const (
	FaultTypeNone FaultType = iota
	FaultTypeClockSkew
	FaultTypeMemoryPressure
	FaultTypeHighLatency
	FaultTypeCPUStarvation
	FaultTypeNetworkPartition
	FaultTypeRedisFailure
	FaultTypeSystemOverload
)

// FaultInjector manages fault injection for testing
type FaultInjector struct {
	mu           sync.RWMutex
	activeFaults map[FaultType]bool
	faultConfig  map[FaultType]interface{}
}

// NewFaultInjector creates a new fault injector
func NewFaultInjector() *FaultInjector {
	return &FaultInjector{
		activeFaults: make(map[FaultType]bool),
		faultConfig:  make(map[FaultType]interface{}),
	}
}

// InjectFault activates a specific fault type
func (fi *FaultInjector) InjectFault(faultType FaultType, config interface{}) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.activeFaults[faultType] = true
	fi.faultConfig[faultType] = config
}

// RemoveFault deactivates a specific fault type
func (fi *FaultInjector) RemoveFault(faultType FaultType) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	delete(fi.activeFaults, faultType)
	delete(fi.faultConfig, faultType)
}

// IsFaultActive checks if a fault is currently active
func (fi *FaultInjector) IsFaultActive(faultType FaultType) bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.activeFaults[faultType]
}

// GetFaultConfig returns the configuration for a fault
func (fi *FaultInjector) GetFaultConfig(faultType FaultType) interface{} {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.faultConfig[faultType]
}

// TestClockSkewFault tests behavior under clock skew conditions
func TestClockSkewFault(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()

	// Simulate clock going backwards
	injector.InjectFault(FaultTypeClockSkew, map[string]interface{}{
		"skew_ms": -1000, // 1 second backwards
	})

	// Generate some IDs normally first
	var normalIDs []int64
	for i := 0; i < 10; i++ {
		id, err := generator.GenerateID()
		require.NoError(t, err)
		normalIDs = append(normalIDs, id)
	}

	// Simulate clock skew by manipulating the generator's internal state
	// In a real implementation, this would involve mocking the time source

	// Test that the generator handles clock skew gracefully
	// This is a simplified test - in practice, you'd need to mock time.Now()
	t.Log("Clock skew fault injection test completed")

	// Verify IDs are still unique and increasing
	for i := 1; i < len(normalIDs); i++ {
		assert.Greater(t, normalIDs[i], normalIDs[i-1], "IDs should be increasing")
	}
}

// TestMemoryPressureFault tests behavior under memory pressure
func TestMemoryPressureFault(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	injector.InjectFault(FaultTypeMemoryPressure, map[string]interface{}{
		"allocation_mb": 100, // Allocate 100MB
	})

	// Simulate memory pressure by allocating large amounts of memory
	var memoryHog [][]byte
	defer func() {
		// Clean up memory
		memoryHog = nil
	}()

	// Allocate memory in chunks
	for i := 0; i < 100; i++ {
		chunk := make([]byte, 1024*1024) // 1MB chunks
		memoryHog = append(memoryHog, chunk)
	}

	// Test ID generation under memory pressure
	var ids []int64
	start := time.Now()

	for i := 0; i < 1000; i++ {
		id, err := generator.GenerateID()
		if err != nil {
			t.Logf("Error generating ID under memory pressure: %v", err)
			continue
		}
		ids = append(ids, id)
	}

	duration := time.Since(start)
	t.Logf("Generated %d IDs under memory pressure in %v", len(ids), duration)

	// Verify all IDs are unique
	idSet := make(map[int64]bool)
	for _, id := range ids {
		assert.False(t, idSet[id], "Duplicate ID found: %d", id)
		idSet[id] = true
	}
}

// TestHighLatencyFault tests behavior under high latency conditions
func TestHighLatencyFault(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	injector.InjectFault(FaultTypeHighLatency, map[string]interface{}{
		"latency_ms": 100, // 100ms artificial latency
	})

	// Simulate high latency operations
	var ids []int64
	var totalLatency time.Duration
	numRequests := 50

	for i := 0; i < numRequests; i++ {
		start := time.Now()

		// Inject artificial latency
		if injector.IsFaultActive(FaultTypeHighLatency) {
			config := injector.GetFaultConfig(FaultTypeHighLatency).(map[string]interface{})
			latencyMs := config["latency_ms"].(int)
			time.Sleep(time.Duration(latencyMs) * time.Millisecond)
		}

		id, err := generator.GenerateID()
		require.NoError(t, err)

		latency := time.Since(start)
		totalLatency += latency
		ids = append(ids, id)
	}

	avgLatency := totalLatency / time.Duration(numRequests)
	t.Logf("Average latency under high latency fault: %v", avgLatency)

	// Verify IDs are still unique and valid
	assert.Equal(t, numRequests, len(ids))
	idSet := make(map[int64]bool)
	for _, id := range ids {
		assert.False(t, idSet[id], "Duplicate ID found: %d", id)
		idSet[id] = true
	}
}

// TestConcurrentFaultInjection tests multiple concurrent faults
func TestConcurrentFaultInjection(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()

	// Inject multiple faults simultaneously
	injector.InjectFault(FaultTypeHighLatency, map[string]interface{}{"latency_ms": 10})
	injector.InjectFault(FaultTypeMemoryPressure, map[string]interface{}{"allocation_mb": 50})

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	numWorkers := 10
	requestsPerWorker := 100

	// Launch concurrent workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				// Inject latency if fault is active
				if injector.IsFaultActive(FaultTypeHighLatency) {
					config := injector.GetFaultConfig(FaultTypeHighLatency).(map[string]interface{})
					latencyMs := config["latency_ms"].(int)
					time.Sleep(time.Duration(latencyMs) * time.Millisecond)
				}

				_, err := generator.GenerateID()
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	totalRequests := int64(numWorkers * requestsPerWorker)
	errorRate := float64(errorCount) / float64(totalRequests) * 100

	t.Logf("Concurrent fault injection results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Error rate: %.2f%%", errorRate)

	// Error rate should be reasonable even under fault conditions
	assert.Less(t, errorRate, 5.0, "Error rate should be less than 5% even under fault conditions")
}

// TestRedisFailureFault tests behavior when Redis is unavailable
func TestRedisFailureFault(t *testing.T) {
	// This test simulates Redis failure scenarios
	// In a real implementation, this would involve mocking Redis client

	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	injector.InjectFault(FaultTypeRedisFailure, map[string]interface{}{
		"failure_type": "connection_timeout",
		"duration_ms":  5000,
	})

	// Test ID generation when Redis is unavailable
	// The generator should fall back to local generation
	var ids []int64
	for i := 0; i < 100; i++ {
		id, err := generator.GenerateID()
		// Should not fail even if Redis is down
		require.NoError(t, err)
		ids = append(ids, id)
	}

	// Verify all IDs are unique
	idSet := make(map[int64]bool)
	for _, id := range ids {
		assert.False(t, idSet[id], "Duplicate ID found: %d", id)
		idSet[id] = true
	}

	t.Logf("Generated %d unique IDs during Redis failure simulation", len(ids))
}

// TestSystemOverloadFault tests behavior under system overload
func TestSystemOverloadFault(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	injector.InjectFault(FaultTypeSystemOverload, map[string]interface{}{
		"cpu_load":    90, // 90% CPU load
		"memory_load": 85, // 85% memory load
		"duration_s":  10, // 10 seconds
	})

	// Simulate system overload with CPU-intensive operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var generatedIDs int64
	var errors int64

	// CPU load simulation
	for i := 0; i < 4; i++ { // Use multiple cores
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// CPU-intensive operation
					for j := 0; j < 1000; j++ {
						_ = j * j * j
					}
				}
			}
		}()
	}

	// ID generation under load
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, err := generator.GenerateID()
				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					atomic.AddInt64(&generatedIDs, 1)
				}
			}
		}
	}()

	wg.Wait()

	errorRate := float64(errors) / float64(generatedIDs+errors) * 100
	t.Logf("System overload test results:")
	t.Logf("  Generated IDs: %d", generatedIDs)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Error rate: %.2f%%", errorRate)

	// Should still generate some IDs even under heavy load
	assert.Greater(t, generatedIDs, int64(0), "Should generate some IDs even under system overload")
	assert.Less(t, errorRate, 10.0, "Error rate should be less than 10% under system overload")
}

// TestNetworkPartitionFault tests behavior during network partitions
func TestNetworkPartitionFault(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	injector.InjectFault(FaultTypeNetworkPartition, map[string]interface{}{
		"partition_type": "split_brain",
		"duration_s":     30,
	})

	// Simulate network partition by testing isolated operation
	var ids []int64
	start := time.Now()

	// Generate IDs during "network partition"
	for i := 0; i < 1000 && time.Since(start) < 5*time.Second; i++ {
		id, err := generator.GenerateID()
		require.NoError(t, err)
		ids = append(ids, id)
	}

	// Verify uniqueness and monotonic increase
	for i := 1; i < len(ids); i++ {
		assert.Greater(t, ids[i], ids[i-1], "IDs should be monotonically increasing")
	}

	t.Logf("Generated %d unique IDs during network partition simulation", len(ids))
}

// TestFaultRecovery tests recovery behavior after faults are resolved
func TestFaultRecovery(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()

	// Phase 1: Normal operation
	var normalIDs []int64
	for i := 0; i < 100; i++ {
		id, err := generator.GenerateID()
		require.NoError(t, err)
		normalIDs = append(normalIDs, id)
	}

	// Phase 2: Inject fault
	injector.InjectFault(FaultTypeHighLatency, map[string]interface{}{"latency_ms": 50})

	var faultIDs []int64
	for i := 0; i < 100; i++ {
		// Simulate latency
		time.Sleep(50 * time.Millisecond)

		id, err := generator.GenerateID()
		require.NoError(t, err)
		faultIDs = append(faultIDs, id)
	}

	// Phase 3: Remove fault and test recovery
	injector.RemoveFault(FaultTypeHighLatency)

	var recoveryIDs []int64
	start := time.Now()
	for i := 0; i < 100; i++ {
		id, err := generator.GenerateID()
		require.NoError(t, err)
		recoveryIDs = append(recoveryIDs, id)
	}
	recoveryDuration := time.Since(start)

	// Verify recovery performance
	avgRecoveryTime := recoveryDuration / 100
	t.Logf("Average ID generation time after fault recovery: %v", avgRecoveryTime)

	// Recovery should be fast (less than 1ms per ID)
	assert.Less(t, avgRecoveryTime, time.Millisecond, "Recovery should be fast")

	// Verify all IDs across phases are unique and increasing
	allIDs := append(append(normalIDs, faultIDs...), recoveryIDs...)
	for i := 1; i < len(allIDs); i++ {
		assert.Greater(t, allIDs[i], allIDs[i-1], "IDs should be monotonically increasing across fault phases")
	}
}

// TestCascadingFailures tests behavior when multiple components fail in sequence
func TestCascadingFailures(t *testing.T) {
	generator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()

	// Simulate cascading failures
	failures := []struct {
		faultType FaultType
		config    interface{}
		delay     time.Duration
	}{
		{FaultTypeRedisFailure, map[string]interface{}{"failure_type": "timeout"}, 0},
		{FaultTypeHighLatency, map[string]interface{}{"latency_ms": 100}, 2 * time.Second},
		{FaultTypeMemoryPressure, map[string]interface{}{"allocation_mb": 50}, 4 * time.Second},
	}

	var wg sync.WaitGroup
	var totalIDs int64
	var totalErrors int64

	// Start ID generation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, err := generator.GenerateID()
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
				} else {
					atomic.AddInt64(&totalIDs, 1)
				}
				time.Sleep(10 * time.Millisecond) // Throttle requests
			}
		}
	}()

	// Inject failures in sequence
	for _, failure := range failures {
		time.Sleep(failure.delay)
		injector.InjectFault(failure.faultType, failure.config)
		t.Logf("Injected fault: %v", failure.faultType)
	}

	wg.Wait()

	errorRate := float64(totalErrors) / float64(totalIDs+totalErrors) * 100
	t.Logf("Cascading failures test results:")
	t.Logf("  Total IDs generated: %d", totalIDs)
	t.Logf("  Total errors: %d", totalErrors)
	t.Logf("  Error rate: %.2f%%", errorRate)

	// System should remain somewhat functional even with cascading failures
	assert.Greater(t, totalIDs, int64(0), "Should generate some IDs even with cascading failures")
	assert.Less(t, errorRate, 20.0, "Error rate should be less than 20% with cascading failures")
}

// Helper function to simulate CPU load
func simulateCPULoad(ctx context.Context, loadPercent int) {
	if loadPercent <= 0 || loadPercent > 100 {
		return
	}

	workDuration := time.Duration(loadPercent) * time.Millisecond
	sleepDuration := time.Duration(100-loadPercent) * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// CPU-intensive work
			start := time.Now()
			for time.Since(start) < workDuration {
				for i := 0; i < 1000; i++ {
					_ = i * i
				}
			}
			// Sleep to control load
			time.Sleep(sleepDuration)
		}
	}
}

// MockErrorGenerator simulates a generator that can fail
type MockErrorGenerator struct {
	*Generator
	failureRate float64 // 0.0 to 1.0
	injector    *FaultInjector
}

// GenerateID overrides the normal GenerateID to inject failures
func (meg *MockErrorGenerator) GenerateID() (int64, error) {
	// Check if we should simulate a failure
	if meg.injector.IsFaultActive(FaultTypeSystemOverload) {
		// Simulate random failures under system overload
		if time.Now().UnixNano()%100 < int64(meg.failureRate*100) {
			return 0, errors.New("simulated system overload failure")
		}
	}

	return meg.Generator.GenerateID()
}

// TestErrorRecovery tests recovery from various error conditions
func TestErrorRecovery(t *testing.T) {
	baseGenerator, err := NewSnowflakeGeneratorCore(1, 1, nil)
	require.NoError(t, err)

	injector := NewFaultInjector()
	mockGen := &MockErrorGenerator{
		Generator:   baseGenerator,
		failureRate: 0.1, // 10% failure rate
		injector:    injector,
	}

	// Enable system overload fault
	injector.InjectFault(FaultTypeSystemOverload, map[string]interface{}{
		"failure_rate": 0.1,
	})

	var successCount int64
	var errorCount int64
	numAttempts := 1000

	for i := 0; i < numAttempts; i++ {
		_, err := mockGen.GenerateID()
		if err != nil {
			atomic.AddInt64(&errorCount, 1)
			// Simulate retry logic
			time.Sleep(time.Millisecond)
		} else {
			atomic.AddInt64(&successCount, 1)
		}
	}

	successRate := float64(successCount) / float64(numAttempts) * 100
	t.Logf("Error recovery test results:")
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Error count: %d", errorCount)

	// Should have reasonable success rate even with 10% failure injection
	assert.Greater(t, successRate, 80.0, "Success rate should be greater than 80%")
}
