package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-lynx/lynx/app/events"
	"github.com/go-lynx/lynx/plugins"
)

func main() {
	// Create custom configuration with error callback
	configs := events.DefaultBusConfigs()
	
	// Add error callback to plugin bus
	configs.Plugin.ErrorCallback = func(event events.LynxEvent, reason string, err error) {
		fmt.Printf("Plugin bus error: event=%s, reason=%s, error=%v\n", event.PluginID, reason, err)
	}
	
	// Enable throttling for business bus
	configs.Business.EnableThrottling = true
	configs.Business.ThrottleRate = 100  // 100 events per second
	configs.Business.ThrottleBurst = 10  // 10 events burst
	
	// Initialize the unified event bus system
	err := events.Init(configs)
	if err != nil {
		log.Fatalf("Failed to initialize event bus: %v", err)
	}

	// Subscribe to plugin events
	err = events.SubscribeTo(events.EventPluginInitialized, func(event events.LynxEvent) {
		fmt.Printf("Plugin initialized: %s\n", event.PluginID)
	})
	if err != nil {
		log.Printf("Failed to subscribe to plugin events: %v", err)
	}

	// Subscribe to health events
	err = events.SubscribeTo(events.EventHealthStatusOK, func(event events.LynxEvent) {
		fmt.Printf("Health status OK: %s\n", event.PluginID)
	})
	if err != nil {
		log.Printf("Failed to subscribe to health events: %v", err)
	}

	// Subscribe to error events
	err = events.SubscribeTo(events.EventErrorOccurred, func(event events.LynxEvent) {
		fmt.Printf("Error occurred: %s\n", event.Source)
	})
	if err != nil {
		log.Printf("Failed to subscribe to error events: %v", err)
	}

	// Create a simple runtime for testing
	runtime := plugins.NewSimpleRuntime()

	// Emit some test events
	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventPluginInitialized,
		Priority:  plugins.PriorityNormal,
		Source:    "example",
		Category:  "lifecycle",
		PluginID:  "test-plugin",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
	}

	// Emit event through runtime (which now uses unified event bus)
	runtime.EmitEvent(pluginEvent)

	// Emit health event
	healthEvent := plugins.PluginEvent{
		Type:      plugins.EventHealthStatusOK,
		Priority:  plugins.PriorityNormal,
		Source:    "health-checker",
		Category:  "health",
		PluginID:  "test-plugin",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
	}

	runtime.EmitEvent(healthEvent)

	// Emit error event
	errorEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityHigh,
		Source:    "system",
		Category:  "error",
		PluginID:  "system",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
	}

	runtime.EmitEvent(errorEvent)

	// Wait a bit for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Get bus status
	status := events.GetGlobalBusStatus()
	fmt.Printf("\nBus Status:\n")
	for busType, busStatus := range status {
		fmt.Printf("  %d: Healthy=%v, QueueSize=%d, Subscribers=%d\n",
			busType, busStatus.IsHealthy, busStatus.QueueSize, busStatus.Subscribers)
	}

	// Close the event bus
	err = events.CloseGlobalEventBus()
	if err != nil {
		log.Printf("Failed to close event bus: %v", err)
	}

	fmt.Println("Example completed successfully!")
}
