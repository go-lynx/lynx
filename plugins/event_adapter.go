package plugins

import "github.com/go-kratos/kratos/v2/log"

// EventBusAdapter provides an interface for plugins to interact with the unified event bus
// This avoids circular imports between plugins and app/events packages
type EventBusAdapter interface {
	PublishEvent(event PluginEvent) error
	Subscribe(eventType EventType, handler func(PluginEvent)) error
	SubscribeTo(eventType EventType, handler func(PluginEvent)) error
}

// FallbackEventBusAdapter provides a safe fallback when no global adapter is available.
// Events published via PublishEvent are logged with key "plugin_event_bus_fallback" and are not
// delivered to any subscriber. Subscribe/SubscribeTo are no-ops; handlers will not be invoked.
type FallbackEventBusAdapter struct{}

// PublishEvent handles event publishing when no adapter is available
func (f *FallbackEventBusAdapter) PublishEvent(event PluginEvent) error {
	// Log so operators can see that the event bus was never set; use structured log key for filtering
	log.DefaultLogger.Log(log.LevelWarn, "plugin_event_bus_fallback", "publish",
		"type", string(event.Type), "plugin", event.PluginID, "source", event.Source)
	return nil
}

// Subscribe handles event subscription when no adapter is available
func (f *FallbackEventBusAdapter) Subscribe(eventType EventType, handler func(PluginEvent)) error {
	log.DefaultLogger.Log(log.LevelWarn, "plugin_event_bus_fallback", "subscribe",
		"event_type", string(eventType), "msg", "events will not be delivered")
	return nil
}

// SubscribeTo handles specific event subscription when no adapter is available
func (f *FallbackEventBusAdapter) SubscribeTo(eventType EventType, handler func(PluginEvent)) error {
	log.DefaultLogger.Log(log.LevelWarn, "plugin_event_bus_fallback", "subscribe_to",
		"event_type", string(eventType), "msg", "events will not be delivered")
	return nil
}
