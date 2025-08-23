package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app/events"
	"github.com/go-lynx/lynx/plugins"
)

// Default event queue and worker parameters (kept for backward compatibility)
const (
	defaultEventQueueSize    = 1024
	defaultEventWorkerCount  = 10
	defaultListenerQueueSize = 256
	defaultHistorySize       = 1000
	defaultDrainTimeoutMs    = 500
)

// TypedRuntimePlugin generic runtime plugin
type TypedRuntimePlugin struct {
	// resources stores shared resources between plugins
	resources sync.Map

	// logger is the plugin's logger instance
	logger log.Logger

	// config is the plugin's configuration
	config config.Config

	// Event bus manager for unified event handling
	eventManager *events.EventBusManager
}

// listenerEntry represents a registered event listener with its filter (kept for backward compatibility)
type listenerEntry struct {
	listener plugins.EventListener
	filter   *plugins.EventFilter
	ch       chan plugins.PluginEvent
	quit     chan struct{}
	active   *int32
}

// NewTypedRuntimePlugin creates a new TypedRuntimePlugin instance with default settings.
func NewTypedRuntimePlugin() *TypedRuntimePlugin {
	r := &TypedRuntimePlugin{
		logger:       log.DefaultLogger,
		eventManager: events.GetGlobalEventBus(),
	}
	return r
}

// GetResource retrieves a shared plugin resource by name.
// Returns the resource and any error encountered.
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	if value, ok := r.resources.Load(name); ok {
		return value, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

// RegisterResource registers a resource to be shared with other plugins.
// Returns an error if registration fails.
func (r *TypedRuntimePlugin) RegisterResource(name string, resource any) error {
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	// Store the resource using sync.Map
	r.resources.Store(name, resource)
	return nil
}

// GetTypedResource retrieves a type-safe resource (standalone helper)
func GetTypedResource[T any](r *TypedRuntimePlugin, name string) (T, error) {
	var zero T
	resource, err := r.GetResource(name)
	if err != nil {
		return zero, err
	}

	typed, ok := resource.(T)
	if !ok {
		return zero, fmt.Errorf("type assertion failed for resource %s", name)
	}

	return typed, nil
}

// RegisterTypedResource registers a type-safe resource (standalone helper)
func RegisterTypedResource[T any](r *TypedRuntimePlugin, name string, resource T) error {
	return r.RegisterResource(name, resource)
}

// GetConfig returns the plugin configuration manager.
// Provides access to configuration values and updates.
func (r *TypedRuntimePlugin) GetConfig() config.Config {
	if r.config == nil {
		if app := Lynx(); app != nil {
			if cfg := app.GetGlobalConfig(); cfg != nil {
				r.config = cfg
			}
		}
	}
	return r.config
}

// GetLogger returns the plugin logger instance.
// Provides structured logging capabilities.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	if r.logger == nil {
		// Initialize with a default logger if not set
		r.logger = log.DefaultLogger
	}
	return r.logger
}

// EmitEvent broadcasts a plugin event to the unified event bus.
// Event will be processed according to its priority and any active filters.
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Type == "" { // Check for zero value of EventType
		return
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// Convert to LynxEvent and publish to unified event bus
	lynxEvent := events.ConvertPluginEvent(event)
	if err := r.eventManager.PublishEvent(lynxEvent); err != nil {
		if r.logger != nil {
			_ = r.logger.Log(log.LevelError, "msg", "failed to publish event", "type", event.Type, "plugin", event.PluginID, "error", err)
		}
	}
}

// Close stops the runtime (optional to call)
func (r *TypedRuntimePlugin) Close() {
	// No specific cleanup needed for unified event bus
	// The global event bus manager handles its own lifecycle
}

// AddListener registers a new event listener with optional filters.
// This method is kept for backward compatibility but delegates to the unified event bus.
func (r *TypedRuntimePlugin) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	// Convert to unified event bus subscription
	// This is a simplified implementation for backward compatibility
	if listener == nil {
		return
	}

	// Convert plugin event listener to unified event bus listener
	unifiedListener := func(event events.LynxEvent) {
		// Convert LynxEvent back to PluginEvent for backward compatibility
		pluginEvent := events.ConvertLynxEvent(event)
		listener.HandleEvent(pluginEvent)
	}

	// Convert plugin event filter to unified event bus filter
	var unifiedFilter *events.EventFilter
	if filter != nil {
		unifiedFilter = &events.EventFilter{
			EventTypes: convertEventTypes(filter.Types),
			PluginIDs:  filter.PluginIDs,
			Categories: filter.Categories,
			Priorities: convertPriorities(filter.Priorities),
			Metadata:   make(map[string]any), // Plugin filter doesn't have metadata
		}
	}

	// Register with unified event bus using the listener manager
	if r.eventManager != nil {
		listenerID := fmt.Sprintf("plugin_%p", listener)
		listenerManager := events.GetGlobalListenerManager()
		if listenerManager != nil {
			_ = listenerManager.AddListener(listenerID, unifiedFilter, unifiedListener, events.BusTypePlugin)
		}
	}
}

// convertEventTypes converts plugin event types to unified event types
func convertEventTypes(pluginTypes []plugins.EventType) []events.EventType {
	if len(pluginTypes) == 0 {
		return nil
	}

	unifiedTypes := make([]events.EventType, len(pluginTypes))
	for i, eventType := range pluginTypes {
		unifiedTypes[i] = events.ConvertEventType(eventType)
	}
	return unifiedTypes
}

// convertPriorities converts plugin priorities to unified event bus priorities
func convertPriorities(pluginPriorities []int) []events.Priority {
	if len(pluginPriorities) == 0 {
		return nil
	}

	unifiedPriorities := make([]events.Priority, len(pluginPriorities))
	for i, priority := range pluginPriorities {
		switch priority {
		case plugins.PriorityLow:
			unifiedPriorities[i] = events.PriorityLow
		case plugins.PriorityNormal:
			unifiedPriorities[i] = events.PriorityNormal
		case plugins.PriorityHigh:
			unifiedPriorities[i] = events.PriorityHigh
		case plugins.PriorityCritical:
			unifiedPriorities[i] = events.PriorityCritical
		default:
			unifiedPriorities[i] = events.PriorityNormal
		}
	}
	return unifiedPriorities
}

// RemoveListener unregisters an event listener.
// This method is kept for backward compatibility but delegates to the unified event bus.
func (r *TypedRuntimePlugin) RemoveListener(listener plugins.EventListener) {
	// Convert plugin event listener to unified event bus listener removal
	if listener == nil {
		return
	}

	// Remove from unified event bus using the listener manager
	listenerManager := events.GetGlobalListenerManager()
	if listenerManager != nil {
		listenerID := fmt.Sprintf("plugin_%p", listener)
		_ = listenerManager.RemoveListener(listenerID)
	}
}

// GetEventHistory retrieves historical events based on filter criteria.
// This method is kept for backward compatibility but delegates to the unified event bus.
func (r *TypedRuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	// Check if filter is empty
	if r.isEmptyFilter(filter) {
		// Return empty slice for empty filter
		return []plugins.PluginEvent{}
	}

	// Get history from unified event bus
	if r.eventManager != nil {
		// Convert plugin filter to unified event bus filter
		unifiedFilter := &events.EventFilter{
			EventTypes: convertEventTypes(filter.Types),
			PluginIDs:  filter.PluginIDs,
			Categories: filter.Categories,
			Priorities: convertPriorities(filter.Priorities),
			FromTime:   filter.FromTime,
			ToTime:     filter.ToTime,
			Metadata:   make(map[string]any), // Plugin filter doesn't have metadata
		}

		// Get events from unified event bus
		lynxEvents := r.eventManager.GetEventHistory(unifiedFilter)

		// Convert back to plugin events
		pluginEvents := make([]plugins.PluginEvent, len(lynxEvents))
		for i, lynxEvent := range lynxEvents {
			pluginEvents[i] = events.ConvertLynxEvent(lynxEvent)
		}

		return pluginEvents
	}

	return []plugins.PluginEvent{}
}

// isEmptyFilter checks if the event filter is empty
func (r *TypedRuntimePlugin) isEmptyFilter(filter plugins.EventFilter) bool {
	return len(filter.Types) == 0 &&
		len(filter.Priorities) == 0 &&
		len(filter.PluginIDs) == 0 &&
		len(filter.Categories) == 0 &&
		filter.FromTime == 0 &&
		filter.ToTime == 0
}

// eventMatchesFilter checks if an event matches a specific filter.
// This method is kept for backward compatibility.
func (r *TypedRuntimePlugin) eventMatchesFilter(event plugins.PluginEvent, filter plugins.EventFilter) bool {
	// Check if filter is empty - empty filter matches all events
	if r.isEmptyFilter(filter) {
		return true
	}

	// Check event type
	if len(filter.Types) > 0 {
		typeMatch := false
		for _, filterType := range filter.Types {
			if event.Type == filterType {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// Check priority
	if len(filter.Priorities) > 0 {
		priorityMatch := false
		for _, filterPriority := range filter.Priorities {
			if event.Priority == filterPriority {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

	// Check plugin ID
	if len(filter.PluginIDs) > 0 {
		pluginMatch := false
		for _, filterPluginID := range filter.PluginIDs {
			if event.PluginID == filterPluginID {
				pluginMatch = true
				break
			}
		}
		if !pluginMatch {
			return false
		}
	}

	// Check category
	if len(filter.Categories) > 0 {
		categoryMatch := false
		for _, filterCategory := range filter.Categories {
			if event.Category == filterCategory {
				categoryMatch = true
				break
			}
		}
		if !categoryMatch {
			return false
		}
	}

	// Check time range
	if filter.FromTime > 0 && event.Timestamp < filter.FromTime {
		return false
	}
	if filter.ToTime > 0 && event.Timestamp > filter.ToTime {
		return false
	}

	return true
}

// RuntimePlugin backward-compatible alias of TypedRuntimePlugin
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin creates a runtime plugin (backward-compatible)
func NewRuntimePlugin() *RuntimePlugin {
	return NewTypedRuntimePlugin()
}
