package plugins

import (
	"fmt"
	"sync"
)

// EventBusAdapter provides an interface for plugins to interact with the unified event bus
// This avoids circular imports between plugins and app/events packages
type EventBusAdapter interface {
	PublishEvent(event PluginEvent) error
	Subscribe(eventType EventType, handler func(PluginEvent)) error
	SubscribeTo(eventType EventType, handler func(PluginEvent)) error
}

// GlobalEventBusAdapter provides global access to the event bus adapter
var (
	globalEventBusAdapter EventBusAdapter
	globalAdapterOnce     sync.Once
	globalAdapterMu       sync.RWMutex
)

// SetGlobalEventBusAdapter sets the global event bus adapter
func SetGlobalEventBusAdapter(adapter EventBusAdapter) {
	globalAdapterMu.Lock()
	defer globalAdapterMu.Unlock()
	globalEventBusAdapter = adapter
}

// GetGlobalEventBusAdapter returns the global event bus adapter
func GetGlobalEventBusAdapter() EventBusAdapter {
	globalAdapterMu.RLock()
	defer globalAdapterMu.RUnlock()
	return globalEventBusAdapter
}

// EnsureGlobalEventBusAdapter ensures the global event bus adapter is available
// Returns a safe adapter that handles nil cases gracefully
func EnsureGlobalEventBusAdapter() EventBusAdapter {
	adapter := GetGlobalEventBusAdapter()
	if adapter != nil {
		return adapter
	}

	// Return a fallback adapter that safely handles operations
	return &FallbackEventBusAdapter{}
}

// PublishEventToGlobalBus publishes an event to the global event bus
func PublishEventToGlobalBus(event PluginEvent) error {
	adapter := EnsureGlobalEventBusAdapter()
	return adapter.PublishEvent(event)
}

// SubscribeToGlobalBus subscribes to events on the global event bus
func SubscribeToGlobalBus(eventType EventType, handler func(PluginEvent)) error {
	adapter := EnsureGlobalEventBusAdapter()
	return adapter.SubscribeTo(eventType, handler)
}

// FallbackEventBusAdapter provides a safe fallback when no global adapter is available
type FallbackEventBusAdapter struct{}

// PublishEvent handles event publishing when no adapter is available
func (f *FallbackEventBusAdapter) PublishEvent(event PluginEvent) error {
	// Log the event attempt but don't fail
	// This allows the system to continue functioning even if event bus is not initialized
	fmt.Printf("[FALLBACK] Would publish event: type=%s, plugin=%s, source=%s\n",
		event.Type, event.PluginID, event.Source)
	return nil
}

// Subscribe handles event subscription when no adapter is available
func (f *FallbackEventBusAdapter) Subscribe(eventType EventType, handler func(PluginEvent)) error {
	// Log the subscription attempt but don't fail
	fmt.Printf("[FALLBACK] Would subscribe to event type: %s\n", eventType)
	return nil
}

// SubscribeTo handles specific event subscription when no adapter is available
func (f *FallbackEventBusAdapter) SubscribeTo(eventType EventType, handler func(PluginEvent)) error {
	// Log the subscription attempt but don't fail
	fmt.Printf("[FALLBACK] Would subscribe to event type: %s\n", eventType)
	return nil
}
