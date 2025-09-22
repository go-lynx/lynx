package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-lynx/lynx/app/events"
	"github.com/go-lynx/lynx/plugins"
)

func main() {
	fmt.Println("=== Lynx Unified Event System Advanced Example ===")

	// Initialize the unified event bus system
	err := events.InitWithDefaultConfig()
	if err != nil {
		log.Fatalf("Failed to initialize event bus: %v", err)
	}

	// Create event filters
	pluginFilter := events.NewEventFilter().
		WithEventType(events.EventPluginInitialized).
		WithPriority(events.PriorityNormal)

	healthFilter := events.NewEventFilter().
		WithEventType(events.EventHealthStatusOK).
		WithCategory("health")

	errorFilter := events.NewEventFilter().
		WithEventType(events.EventErrorOccurred).
		WithPriority(events.PriorityHigh)

	// Add listeners with filters
	err = events.AddGlobalListener("plugin-listener", pluginFilter, func(event events.LynxEvent) {
		fmt.Printf("🔧 Plugin initialized: %s (Priority: %d)\n", event.PluginID, event.Priority)
	}, events.BusTypePlugin)
	if err != nil {
		log.Printf("Failed to add plugin listener: %v", err)
	}

	err = events.AddGlobalListener("health-listener", healthFilter, func(event events.LynxEvent) {
		fmt.Printf("💚 Health status OK: %s\n", event.PluginID)
	}, events.BusTypeHealth)
	if err != nil {
		log.Printf("Failed to add health listener: %v", err)
	}

	err = events.AddGlobalListener("error-listener", errorFilter, func(event events.LynxEvent) {
		fmt.Printf("❌ Error occurred: %s - %v\n", event.Source, event.Error)
	}, events.BusTypeSystem)
	if err != nil {
		log.Printf("Failed to add error listener: %v", err)
	}

	// Create a simple runtime for testing
	runtime := plugins.NewSimpleRuntime()

	// Emit events with different priorities and categories
	fmt.Println("\n--- Emitting Events ---")

	// Plugin initialization event
	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventPluginInitialized,
		Priority:  plugins.PriorityNormal,
		Source:    "example",
		Category:  "lifecycle",
		PluginID:  "test-plugin",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"version": "1.0.0",
			"config":  "default",
		},
	}
	runtime.EmitEvent(pluginEvent)

	// Health check event
	healthEvent := plugins.PluginEvent{
		Type:      plugins.EventHealthStatusOK,
		Priority:  plugins.PriorityNormal,
		Source:    "health-checker",
		Category:  "health",
		PluginID:  "test-plugin",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"response_time": 150,
			"memory_usage":  "45%",
		},
	}
	runtime.EmitEvent(healthEvent)

	// Error event
	errorEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityHigh,
		Source:    "database-connector",
		Category:  "error",
		PluginID:  "db-plugin",
		Status:    plugins.StatusFailed,
		Timestamp: time.Now().Unix(),
		Error:     fmt.Errorf("connection timeout"),
		Metadata: map[string]any{
			"retry_count": 3,
			"timeout_ms":  5000,
		},
	}
	runtime.EmitEvent(errorEvent)

	// Wait for events to be processed
	time.Sleep(200 * time.Millisecond)

	// Display system status
	fmt.Println("\n--- System Status ---")

	// Bus status
	busStatus := events.GetGlobalBusStatus()
	fmt.Println("Bus Status:")
	for busType, status := range busStatus {
		busName := getBusTypeName(busType)
		fmt.Printf("  %s: Healthy=%v, QueueSize=%d, Subscribers=%d\n",
			busName, status.IsHealthy, status.QueueSize, status.Subscribers)
	}

	// System health
	health := events.GetEventSystemHealth()
	fmt.Printf("\nOverall System Health: %v\n", health.OverallHealthy)
	if len(health.Issues) > 0 {
		fmt.Println("Issues:")
		for _, issue := range health.Issues {
			fmt.Printf("  - %s\n", issue)
		}
	}

	// Global metrics
	metrics := events.GetGlobalMetrics()
	fmt.Printf("\nGlobal Metrics:\n")
	fmt.Printf("  Events Published: %v\n", metrics["total_events_published"])
	fmt.Printf("  Events Processed: %v\n", metrics["total_events_processed"])
	fmt.Printf("  Events Dropped: %v\n", metrics["total_events_dropped"])
	fmt.Printf("  Events Failed: %v\n", metrics["total_events_failed"])
	fmt.Printf("  Error Count: %v\n", metrics["error_count"])

	// List all listeners
	listeners := events.ListGlobalListeners()
	fmt.Printf("\nActive Listeners (%d):\n", len(listeners))
	for _, listener := range listeners {
		busName := getBusTypeName(listener.BusType)
		fmt.Printf("  %s (Bus: %s, Active: %v)\n", listener.ID, busName, listener.Active.Load())
	}

	// Test event filtering
	fmt.Println("\n--- Testing Event Filtering ---")

	// Create a time-based filter
	timeFilter := events.NewEventFilter().
		WithTimeRange(time.Now().Add(-1*time.Minute), time.Now().Add(1*time.Minute)).
		WithPriority(events.PriorityHigh)

	// Test if events match the filter
	testEvent := events.LynxEvent{
		EventType: events.EventErrorOccurred,
		Priority:  events.PriorityHigh,
		Timestamp: time.Now().Unix(),
	}

	if timeFilter.Matches(testEvent) {
		fmt.Println("✅ Time filter matches test event")
	} else {
		fmt.Println("❌ Time filter does not match test event")
	}

	// Remove a listener
	fmt.Println("\n--- Removing Listener ---")
	err = events.RemoveGlobalListener("plugin-listener")
	if err != nil {
		log.Printf("Failed to remove listener: %v", err)
	} else {
		fmt.Println("✅ Plugin listener removed successfully")
	}

	// List remaining listeners
	listeners = events.ListGlobalListeners()
	fmt.Printf("Remaining Listeners (%d):\n", len(listeners))
	for _, listener := range listeners {
		busName := getBusTypeName(listener.BusType)
		fmt.Printf("  %s (Bus: %s, Active: %v)\n", listener.ID, busName, listener.Active.Load())
	}

	// Close the event bus
	fmt.Println("\n--- Shutting Down ---")
	err = events.CloseGlobalEventBus()
	if err != nil {
		log.Printf("Failed to close event bus: %v", err)
	} else {
		fmt.Println("✅ Event bus closed successfully")
	}

	fmt.Println("\n=== Example completed successfully! ===")
}

// getBusTypeName returns a human-readable name for bus types
func getBusTypeName(busType events.BusType) string {
	switch busType {
	case events.BusTypePlugin:
		return "Plugin"
	case events.BusTypeSystem:
		return "System"
	case events.BusTypeBusiness:
		return "Business"
	case events.BusTypeHealth:
		return "Health"
	case events.BusTypeConfig:
		return "Config"
	case events.BusTypeResource:
		return "Resource"
	case events.BusTypeSecurity:
		return "Security"
	case events.BusTypeMetrics:
		return "Metrics"
	default:
		return "Unknown"
	}
}
