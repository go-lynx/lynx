package events

import (
	"runtime"
	"testing"
	"time"
)

func TestMemoryOptimization(t *testing.T) {
	// Create config
	configs := DefaultBusConfigs()
	configs.Plugin.BatchSize = 16
	configs.Plugin.MaxQueue = 1000

	// Create manager
	manager, err := NewEventBusManager(configs)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get plugin bus
	bus := manager.GetBus(BusTypePlugin)
	if bus == nil {
		t.Fatal("Plugin bus not found")
	}

	// Subscribe to events
	bus.Subscribe(func(event LynxEvent) {
		// Simulate some processing
		time.Sleep(1 * time.Microsecond)
	})

	// Record initial memory stats
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Publish many events to trigger batch processing
	event := NewLynxEvent(EventPluginInitialized, "test-plugin", "test")
	for i := 0; i < 10000; i++ {
		bus.Publish(event)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Record final memory stats
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory allocation difference
	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("Memory allocated: %d bytes", allocDiff)

	// The test passes if memory allocation is reasonable
	// With object pooling, we expect significantly less allocation
	if allocDiff > 10*1024*1024 { // 10MB threshold
		t.Errorf("Memory allocation too high: %d bytes", allocDiff)
	}
}

func BenchmarkMemoryOptimizedBatchProcessing(b *testing.B) {
	// Create config
	configs := DefaultBusConfigs()
	configs.Plugin.BatchSize = 32
	configs.Plugin.MaxQueue = 10000

	// Create manager
	manager, err := NewEventBusManager(configs)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get plugin bus
	bus := manager.GetBus(BusTypePlugin)
	if bus == nil {
		b.Fatal("Plugin bus not found")
	}

	// Subscribe to events
	bus.Subscribe(func(event LynxEvent) {
		// Empty handler for benchmark
	})

	// Create test event
	event := NewLynxEvent(EventPluginInitialized, "test-plugin", "test")

	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark batch processing
	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}

func BenchmarkObjectPoolVsDirectAllocation(b *testing.B) {
	// Benchmark object pool
	b.Run("ObjectPool", func(b *testing.B) {
		pool := NewEventBufferPool()
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := pool.GetWithCapacity(64) // Use larger capacity
			// Simulate some work
			for j := 0; j < 32; j++ {
				buf = append(buf, LynxEvent{})
			}
			pool.Put(buf)
		}
	})

	// Benchmark direct allocation
	b.Run("DirectAllocation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := make([]LynxEvent, 0, 64) // Use same capacity
			// Simulate some work
			for j := 0; j < 32; j++ {
				buf = append(buf, LynxEvent{})
			}
			// buf goes out of scope and gets GC'd
		}
	})
}

func BenchmarkMetadataPoolVsDirectAllocation(b *testing.B) {
	// Benchmark metadata pool
	b.Run("MetadataPool", func(b *testing.B) {
		pool := NewMetadataPool()
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			m := pool.Get()
			m["key1"] = "value1"
			m["key2"] = "value2"
			m["key3"] = 123
			pool.Put(m)
		}
	})

	// Benchmark direct allocation
	b.Run("DirectAllocation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			m := make(map[string]any, 8)
			m["key1"] = "value1"
			m["key2"] = "value2"
			m["key3"] = 123
			// m goes out of scope and gets GC'd
		}
	})
}
