package events

import (
	"testing"
	"time"
)

// BenchmarkEventPublishing benchmarks event publishing performance
func BenchmarkEventPublishing(b *testing.B) {
	configs := DefaultBusConfigs()
	manager, err := NewEventBusManager(configs)
	if err != nil {
		b.Fatalf("Failed to create event bus manager: %v", err)
	}
	defer manager.Close()

	// Create test event
	event := LynxEvent{
		EventType: EventSystemStart,
		Priority:  PriorityNormal,
		Source:    "benchmark",
		Category:  "test",
		PluginID:  "benchmark",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"test": "benchmark"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = manager.PublishEvent(event)
	}
}

// BenchmarkEventSubscription benchmarks event subscription performance
func BenchmarkEventSubscription(b *testing.B) {
	configs := DefaultBusConfigs()
	manager, err := NewEventBusManager(configs)
	if err != nil {
		b.Fatalf("Failed to create event bus manager: %v", err)
	}
	defer manager.Close()

	handler := func(event LynxEvent) {
		// Empty handler for benchmark
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cancel, _ := manager.SubscribeWithCancel(BusTypeSystem, handler)
		cancel()
	}
}

// BenchmarkEventHistory benchmarks event history performance
func BenchmarkEventHistory(b *testing.B) {
	history := NewEventHistory(1000)

	event := LynxEvent{
		EventType: EventSystemStart,
		Priority:  PriorityNormal,
		Source:    "benchmark",
		Category:  "test",
		PluginID:  "benchmark",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"test": "benchmark"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		history.Add(event)
	}
}

// BenchmarkEventMetrics benchmarks event metrics performance
func BenchmarkEventMetrics(b *testing.B) {
	metrics := NewEventMetrics()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		metrics.UpdateLatency(time.Microsecond)
		metrics.IncrementPublished()
		metrics.IncrementProcessed()
	}
}

// BenchmarkEventClassification benchmarks event classification performance
func BenchmarkEventClassification(b *testing.B) {
	classifier := NewEventClassifier()

	event := LynxEvent{
		EventType: EventSystemStart,
		Priority:  PriorityNormal,
		Source:    "benchmark",
		Category:  "test",
		PluginID:  "benchmark",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"test": "benchmark"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = classifier.GetBusType(event)
	}
}

// BenchmarkConcurrentEventPublishing benchmarks concurrent event publishing
func BenchmarkConcurrentEventPublishing(b *testing.B) {
	configs := DefaultBusConfigs()
	manager, err := NewEventBusManager(configs)
	if err != nil {
		b.Fatalf("Failed to create event bus manager: %v", err)
	}
	defer manager.Close()

	// Subscribe to events to make it realistic
	handler := func(event LynxEvent) {
		// Empty handler
	}
	manager.Subscribe(BusTypeSystem, handler)

	event := LynxEvent{
		EventType: EventSystemStart,
		Priority:  PriorityNormal,
		Source:    "benchmark",
		Category:  "test",
		PluginID:  "benchmark",
		Status:    "active",
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"test": "benchmark"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = manager.PublishEvent(event)
		}
	})
}
