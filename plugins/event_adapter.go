package plugins

import (
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

// PublishEventToGlobalBus publishes an event to the global event bus
func PublishEventToGlobalBus(event PluginEvent) error {
	adapter := GetGlobalEventBusAdapter()
	if adapter == nil {
		// Fallback to runtime if no global adapter is set
		return nil
	}
	return adapter.PublishEvent(event)
}

// SubscribeToGlobalBus subscribes to events on the global event bus
func SubscribeToGlobalBus(eventType EventType, handler func(PluginEvent)) error {
	adapter := GetGlobalEventBusAdapter()
	if adapter == nil {
		// Fallback to runtime if no global adapter is set
		return nil
	}
	return adapter.SubscribeTo(eventType, handler)
}
