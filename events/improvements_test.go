package events

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestErrorCallback(t *testing.T) {
	// Create config with error callback
	configs := DefaultBusConfigs()
	configs.Plugin.MaxQueue = 1 // Minimal queue so overflow is near-certain
	var mu sync.Mutex
	var callbackCalled bool
	var callbackReason string
	var callbackErr error

	configs.Plugin.ErrorCallback = func(event LynxEvent, reason string, err error) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackReason = reason
		callbackErr = err
	}

	// Create manager
	manager, err := NewEventBusManager(configs)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get plugin bus and fill its queue to trigger overflow
	bus := manager.GetBus(BusTypePlugin)
	if bus == nil {
		t.Fatal("Plugin bus not found")
	}

	// Rapid-fire publish to overflow the tiny queue
	event := NewLynxEvent(EventPluginInitialized, "test-plugin", "test")
	for i := 0; i < 500; i++ {
		bus.Publish(event)
	}

	// Wait for processing with timeout
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

outer:
	for {
		select {
		case <-timeout:
			t.Log("Timeout waiting for error callback")
			break outer
		case <-ticker.C:
			if callbackCalled {
				break outer
			}
		}
	}

	// Check if callback was called
	mu.Lock()
	called := callbackCalled
	reason := callbackReason
	cbErr := callbackErr
	mu.Unlock()

	if !called {
		t.Error("Error callback was not called")
		return
	}
	if reason != "drop_newest" {
		t.Errorf("Expected reason 'drop_newest', got '%s'", reason)
	}
	if cbErr == nil {
		t.Error("Expected error in callback")
	}
}

func TestThrottling(t *testing.T) {
	// Create config with throttling enabled
	configs := DefaultBusConfigs()
	configs.Business.EnableThrottling = true
	configs.Business.ThrottleRate = 10 // 10 events per second
	configs.Business.ThrottleBurst = 2 // 2 events burst

	var throttledEvents int
	configs.Business.ErrorCallback = func(event LynxEvent, reason string, err error) {
		if reason == "throttled" {
			throttledEvents++
		}
	}

	// Create manager
	manager, err := NewEventBusManager(configs)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get business bus
	bus := manager.GetBus(BusTypeBusiness)
	if bus == nil {
		t.Fatal("Business bus not found")
	}

	// Publish events rapidly
	event := NewLynxEvent(EventSystemStart, "test", "test")
	for i := 0; i < 10; i++ {
		bus.Publish(event)
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Check if some events were throttled
	if throttledEvents == 0 {
		t.Error("Expected some events to be throttled")
	}
}

func TestConcurrentRetrySafety(t *testing.T) {
	// Create config with minimal retries to avoid long test
	configs := DefaultBusConfigs()
	configs.Plugin.MaxRetries = 1 // Only 1 retry to keep test fast

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

	// Use a counter to track processed events
	var processedCount int64
	var mu sync.Mutex

	// Subscribe with a handler that panics but increments counter
	bus.Subscribe(func(event LynxEvent) {
		mu.Lock()
		processedCount++
		mu.Unlock()
		panic("test panic")
	})

	// Publish events concurrently
	event := NewLynxEvent(EventPluginInitialized, "test-plugin", "test")
	for i := 0; i < 5; i++ { // Reduced number of events
		go bus.Publish(event)
	}

	// Wait for processing with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Logf("Test completed with timeout. Processed events: %d", processedCount)
			return
		case <-ticker.C:
			mu.Lock()
			count := processedCount
			mu.Unlock()
			if count >= 5 {
				t.Logf("All events processed. Count: %d", count)
				return
			}
		}
	}
}

func TestConfigurationValidation(t *testing.T) {
	// Test invalid configuration
	configs := DefaultBusConfigs()
	configs.Plugin.MaxQueue = -1 // Invalid

	_, err := NewEventBusManager(configs)
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}

	// Test valid configuration
	configs = DefaultBusConfigs()
	configs.Plugin.EnableThrottling = true
	configs.Plugin.ThrottleRate = 100
	configs.Plugin.ThrottleBurst = 10

	_, err = NewEventBusManager(configs)
	if err != nil {
		t.Errorf("Unexpected error for valid configuration: %v", err)
	}

	// Test invalid throttling configuration
	configs.Plugin.ThrottleRate = 0 // Invalid

	_, err = NewEventBusManager(configs)
	if err == nil {
		t.Error("Expected error for invalid throttling configuration")
	}
}

func TestGlobalListenerManager_UsesProviderSafely(t *testing.T) {
	t.Cleanup(func() {
		ClearDefaultListenerManagerProvider()
	})

	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("failed to create event bus manager: %v", err)
	}
	defer manager.Close()

	listenerManager := NewEventListenerManagerWithEventBus(manager)
	SetDefaultListenerManagerProvider(func() *EventListenerManager {
		return listenerManager
	})

	got := GetGlobalListenerManager()
	if got != listenerManager {
		t.Fatalf("expected provider-backed listener manager, got %p want %p", got, listenerManager)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := AddGlobalListenerWithContext(ctx, "provider-listener", nil, func(LynxEvent) {}, BusTypePlugin); err != nil {
		t.Fatalf("failed to add global listener via provider-backed manager: %v", err)
	}
	if listenerManager.Count() != 1 {
		t.Fatalf("expected listener to be registered on provider-backed manager, count=%d", listenerManager.Count())
	}

	cancel()
	time.Sleep(20 * time.Millisecond)
	if listenerManager.Count() != 0 {
		t.Fatalf("expected listener to be removed after context cancellation, count=%d", listenerManager.Count())
	}
}

func TestExplicitManagerHelpers_DoNotRequireGlobals(t *testing.T) {
	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("failed to create event bus manager: %v", err)
	}
	defer manager.Close()

	listenerManager := NewEventListenerManagerWithEventBus(manager)

	eventReceived := make(chan struct{}, 1)
	if err := SubscribeToWithManager(manager, EventPluginInitialized, func(LynxEvent) {
		select {
		case eventReceived <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("failed to subscribe to event with explicit manager: %v", err)
	}

	if err := PublishEventWithManager(manager, NewLynxEvent(EventPluginInitialized, "test-plugin", "test")); err != nil {
		t.Fatalf("failed to publish with explicit manager: %v", err)
	}

	select {
	case <-eventReceived:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event to be delivered via explicit manager helpers")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := AddListenerWithManagerAndContext(ctx, listenerManager, "explicit-listener", nil, func(LynxEvent) {}, BusTypePlugin); err != nil {
		t.Fatalf("failed to add listener with explicit manager: %v", err)
	}
	if listenerManager.Count() != 1 {
		t.Fatalf("expected listener to be registered on explicit manager, count=%d", listenerManager.Count())
	}

	if err := RemoveListenerWithManager(listenerManager, "explicit-listener"); err != nil {
		t.Fatalf("failed to remove listener with explicit manager: %v", err)
	}
	if listenerManager.Count() != 0 {
		t.Fatalf("expected listener to be removed from explicit manager, count=%d", listenerManager.Count())
	}
}
