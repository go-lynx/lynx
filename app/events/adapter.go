package events

import (
	"github.com/go-lynx/lynx/plugins"
)

// PluginEventBusAdapter implements plugins.EventBusAdapter interface
// This bridges the unified event bus with the plugin system
type PluginEventBusAdapter struct {
	eventManager *EventBusManager
}

// NewPluginEventBusAdapter creates a new plugin event bus adapter
func NewPluginEventBusAdapter(eventManager *EventBusManager) *PluginEventBusAdapter {
	return &PluginEventBusAdapter{
		eventManager: eventManager,
	}
}

// PublishEvent publishes a plugin event to the unified event bus
func (a *PluginEventBusAdapter) PublishEvent(event plugins.PluginEvent) error {
	// Convert PluginEvent to LynxEvent
	lynxEvent := ConvertPluginEvent(event)
	return a.eventManager.PublishEvent(lynxEvent)
}

// Subscribe subscribes to events on the unified event bus
func (a *PluginEventBusAdapter) Subscribe(eventType plugins.EventType, handler func(plugins.PluginEvent)) error {
	// Convert event type and create wrapper handler
	lynxEventType := ConvertEventType(eventType)

	wrapperHandler := func(lynxEvent LynxEvent) {
		// Convert LynxEvent back to PluginEvent
		pluginEvent := ConvertLynxEvent(lynxEvent)
		handler(pluginEvent)
	}

	return a.eventManager.SubscribeTo(lynxEventType, wrapperHandler)
}

// SubscribeTo subscribes to a specific event type on the unified event bus
func (a *PluginEventBusAdapter) SubscribeTo(eventType plugins.EventType, handler func(plugins.PluginEvent)) error {
	// Convert event type and create wrapper handler
	lynxEventType := ConvertEventType(eventType)

	wrapperHandler := func(lynxEvent LynxEvent) {
		// Convert LynxEvent back to PluginEvent
		pluginEvent := ConvertLynxEvent(lynxEvent)
		handler(pluginEvent)
	}

	return a.eventManager.SubscribeTo(lynxEventType, wrapperHandler)
}

// SetupPluginEventBusAdapter sets up the global plugin event bus adapter
func SetupPluginEventBusAdapter() {
	adapter := NewPluginEventBusAdapter(GetGlobalEventBus())
	plugins.SetGlobalEventBusAdapter(adapter)
}
