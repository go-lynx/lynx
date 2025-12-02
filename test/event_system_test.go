// Package lynx_test contains integration tests for the Lynx framework event system.
// These tests verify event emission, handling, and performance across plugin contexts.
package lynx_test

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx"
	"github.com/go-lynx/lynx/plugins"
)

// TestEventSystemIntegration tests event system integration
func TestEventSystemIntegration(t *testing.T) {
	runtime := lynx.NewTypedRuntimePlugin()

	// Test basic event emission
	t.Run("BasicEventEmission", func(t *testing.T) {
		// Create test event
		testEvent := plugins.PluginEvent{
			Type:      "test.event",
			Priority:  plugins.PriorityNormal,
			Source:    "test",
			Category:  "test",
			PluginID:  "test-plugin",
			Status:    plugins.StatusActive,
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"test": "data",
			},
		}

		// Emit event (should not error)
		runtime.EmitEvent(testEvent)

		// Wait for event processing
		time.Sleep(50 * time.Millisecond)

		t.Log("Event emitted successfully")
	})

	// Test plugin event emission
	t.Run("PluginEventEmission", func(t *testing.T) {
		pluginRuntime := runtime.WithPluginContext("test-plugin")

		// Emit plugin event
		pluginRuntime.EmitPluginEvent("test-plugin", "test.plugin.event", map[string]any{
			"plugin": "test-data",
		})

		// Wait for event processing
		time.Sleep(50 * time.Millisecond)

		t.Log("Plugin event emitted successfully")
	})

	// Test event system configuration
	t.Run("EventSystemConfiguration", func(t *testing.T) {
		// Set event dispatch mode
		runtime.SetEventDispatchMode("async")

		// Set worker pool size
		runtime.SetEventWorkerPoolSize(5)

		// Set event timeout
		runtime.SetEventTimeout(30 * time.Second)

		// Get event statistics
		stats := runtime.GetEventStats()
		if stats == nil {
			t.Error("Event stats should not be nil")
		} else {
			t.Logf("Event stats: %+v", stats)
		}
	})

	// Test event history
	t.Run("EventHistory", func(t *testing.T) {
		// Emit some test events
		for i := 0; i < 3; i++ {
			testEvent := plugins.PluginEvent{
				Type:      plugins.EventType("test.history." + string(rune('0'+i))),
				Priority:  plugins.PriorityNormal,
				Source:    "test",
				Category:  "history",
				PluginID:  "history-test",
				Status:    plugins.StatusActive,
				Timestamp: time.Now().Unix(),
				Metadata: map[string]any{
					"index": i,
				},
			}
			runtime.EmitEvent(testEvent)
		}

		// Wait for event processing
		time.Sleep(100 * time.Millisecond)

		// Get event history
		filter := plugins.EventFilter{
			PluginIDs:  []string{"history-test"},
			Categories: []string{"history"},
		}

		history := runtime.GetEventHistory(filter)
		t.Logf("Found %d events in history", len(history))

		// Get plugin-specific event history
		pluginHistory := runtime.GetPluginEventHistory("history-test", filter)
		t.Logf("Found %d plugin events in history", len(pluginHistory))
	})
}

// TestEventSystemPerformance tests event system performance
func TestEventSystemPerformance(t *testing.T) {
	runtime := lynx.NewTypedRuntimePlugin()

	// Set async mode for better performance
	runtime.SetEventDispatchMode("async")
	runtime.SetEventWorkerPoolSize(10)

	// Emit a large number of events
	numEvents := 1000
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		testEvent := plugins.PluginEvent{
			Type:      "performance.test",
			Priority:  plugins.PriorityLow,
			Source:    "performance-test",
			Category:  "benchmark",
			PluginID:  "perf-test",
			Status:    plugins.StatusActive,
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"index": i,
			},
		}
		runtime.EmitEvent(testEvent)
	}

	duration := time.Since(start)
	t.Logf("Emitted %d events in %v (%.2f events/sec)",
		numEvents, duration, float64(numEvents)/duration.Seconds())

	// Wait for all events to be processed
	time.Sleep(500 * time.Millisecond)

	// Get final statistics
	stats := runtime.GetEventStats()
	if stats != nil {
		t.Logf("Final event stats: %+v", stats)
	}
}

// TestPluginContextFunctionality tests basic plugin context functionality
func TestPluginContextFunctionalityFixed(t *testing.T) {
	runtime := lynx.NewTypedRuntimePlugin()

	// Test basic plugin context creation
	t.Run("BasicPluginContext", func(t *testing.T) {
		pluginName := "test-plugin"
		contextRuntime := runtime.WithPluginContext(pluginName)

		if contextRuntime == nil {
			t.Fatal("WithPluginContext returned nil")
		}

		// Verify current plugin context
		currentContext := contextRuntime.GetCurrentPluginContext()
		if currentContext != pluginName {
			t.Errorf("Expected plugin context '%s', got '%s'", pluginName, currentContext)
		}
	})

	// Test plugin context isolation
	t.Run("PluginContextIsolation", func(t *testing.T) {
		plugin1 := runtime.WithPluginContext("plugin1")
		plugin2 := runtime.WithPluginContext("plugin2")

		// Verify contexts of different plugins are independent
		context1 := plugin1.GetCurrentPluginContext()
		context2 := plugin2.GetCurrentPluginContext()

		if context1 == context2 {
			t.Error("Plugin contexts should be isolated")
		}

		if context1 != "plugin1" {
			t.Errorf("Plugin1 context: expected 'plugin1', got '%s'", context1)
		}

		if context2 != "plugin2" {
			t.Errorf("Plugin2 context: expected 'plugin2', got '%s'", context2)
		}
	})

	// Test resource isolation in plugin contexts
	t.Run("PluginContextResourceIsolation", func(t *testing.T) {
		plugin1 := runtime.WithPluginContext("plugin1")
		plugin2 := runtime.WithPluginContext("plugin2")

		// Register private resource in plugin1
		err := plugin1.RegisterPrivateResource("test-resource", "plugin1-value")
		if err != nil {
			t.Fatalf("Failed to register private resource in plugin1: %v", err)
		}

		// Register private resource with the same name in plugin2
		err = plugin2.RegisterPrivateResource("test-resource", "plugin2-value")
		if err != nil {
			t.Fatalf("Failed to register private resource in plugin2: %v", err)
		}

		// Verify resource isolation
		value1, err := plugin1.GetPrivateResource("test-resource")
		if err != nil {
			t.Fatalf("Failed to get private resource from plugin1: %v", err)
		}
		if value1 != "plugin1-value" {
			t.Errorf("Plugin1 resource: expected 'plugin1-value', got %v", value1)
		}

		value2, err := plugin2.GetPrivateResource("test-resource")
		if err != nil {
			t.Fatalf("Failed to get private resource from plugin2: %v", err)
		}
		if value2 != "plugin2-value" {
			t.Errorf("Plugin2 resource: expected 'plugin2-value', got %v", value2)
		}
	})
}
