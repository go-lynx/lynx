package events

import (
	"fmt"
	"strings"

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

// AddListener bridges a generic listener registration from plugins runtime into the unified event system
// Signature matches the dynamic assertion used by plugins.simpleRuntime
func (a *PluginEventBusAdapter) AddListener(id string, filter *plugins.EventFilter, handler func(interface{}), bus string) error {
	if a == nil || a.eventManager == nil {
		return fmt.Errorf("event manager not initialized")
	}
	ef := convertPluginFilter(filter)
	busType := busFromString(bus)
	lynxHandler := func(ev LynxEvent) {
		// Convert LynxEvent back to PluginEvent and pass via interface{}
		pluginEvent := ConvertLynxEvent(ev)
		handler(pluginEvent)
	}
	return AddGlobalListener(id, ef, lynxHandler, busType)
}

// RemoveListener removes a previously added listener by ID
// Signature matches the dynamic assertion used by plugins.simpleRuntime
func (a *PluginEventBusAdapter) RemoveListener(id string) error {
	return RemoveGlobalListener(id)
}

// AddPluginListener registers a listener bound to a specific plugin namespace
// Signature matches the dynamic assertion used by plugins.simpleRuntime
func (a *PluginEventBusAdapter) AddPluginListener(pluginName string, id string, filter *plugins.EventFilter, handler func(interface{})) error {
	if a == nil || a.eventManager == nil {
		return fmt.Errorf("event manager not initialized")
	}
	ef := convertPluginFilter(filter)
	if ef == nil {
		ef = NewEventFilter()
	}
	// Ensure plugin namespace constraint
	ensurePluginID(ef, pluginName)

	lynxHandler := func(ev LynxEvent) {
		pluginEvent := ConvertLynxEvent(ev)
		handler(pluginEvent)
	}
	return AddGlobalListener(id, ef, lynxHandler, BusTypePlugin)
}

// GetEventHistory returns historical plugin events via the unified event manager
func (a *PluginEventBusAdapter) GetEventHistory(filter *plugins.EventFilter) []plugins.PluginEvent {
	if a == nil || a.eventManager == nil {
		return nil
	}
	ef := convertPluginFilter(filter)
	events := a.eventManager.GetEventHistory(ef)
	out := make([]plugins.PluginEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, ConvertLynxEvent(ev))
	}
	return out
}

// GetPluginEventHistory returns historical events for a specific plugin
func (a *PluginEventBusAdapter) GetPluginEventHistory(pluginID string, filter *plugins.EventFilter) []plugins.PluginEvent {
	if a == nil || a.eventManager == nil {
		return nil
	}
	ef := convertPluginFilter(filter)
	events := a.eventManager.GetPluginEventHistory(pluginID, ef)
	out := make([]plugins.PluginEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, ConvertLynxEvent(ev))
	}
	return out
}
// convertPluginFilter converts plugins.EventFilter to events.EventFilter
func convertPluginFilter(pf *plugins.EventFilter) *EventFilter {
	if pf == nil {
		return nil
	}
	ef := &EventFilter{
		EventTypes: make([]EventType, 0, len(pf.Types)),
		Priorities: make([]Priority, 0, len(pf.Priorities)),
		Sources:    make([]string, 0),
		Categories: append([]string(nil), pf.Categories...),
		PluginIDs:  append([]string(nil), pf.PluginIDs...),
		FromTime:   pf.FromTime,
		ToTime:     pf.ToTime,
		Metadata:   make(map[string]any),
		Statuses:   make([]string, 0),
	}
	for _, t := range pf.Types {
		ef.EventTypes = append(ef.EventTypes, ConvertEventType(t))
	}
	for _, p := range pf.Priorities {
		ef.Priorities = append(ef.Priorities, Priority(p))
	}
	// plugins.EventFilter has no Sources/Statuses/Metadata; leave defaults
	return ef
}

// ensurePluginID adds the pluginID to filter if not present
func ensurePluginID(f *EventFilter, pluginID string) {
	if f == nil || pluginID == "" {
		return
	}
	for _, id := range f.PluginIDs {
		if id == pluginID {
			return
		}
	}
	f.PluginIDs = append(f.PluginIDs, pluginID)
}

// busFromString converts a bus string from plugins runtime to BusType
func busFromString(s string) BusType {
	switch strings.ToLower(s) {
	case "plugin":
		return BusTypePlugin
	case "system":
		return BusTypeSystem
	case "business":
		return BusTypeBusiness
	case "health":
		return BusTypeHealth
	case "config":
		return BusTypeConfig
	case "resource":
		return BusTypeResource
	case "security":
		return BusTypeSecurity
	case "metrics":
		return BusTypeMetrics
	default:
		return BusTypePlugin
	}
}
