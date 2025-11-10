package app

import (
	"fmt"
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
	// Use unified Runtime as the underlying implementation
	runtime plugins.Runtime

	// Event bus manager for unified event handling (kept for backward compatibility)
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
	runtime := plugins.NewUnifiedRuntime()
	runtime.SetLogger(log.DefaultLogger)
	
	r := &TypedRuntimePlugin{
		runtime:      runtime,
		eventManager: events.GetGlobalEventBus(),
	}
	return r
}

// GetResource retrieves a shared plugin resource by name.
// Returns the resource and any error encountered.
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	return r.runtime.GetResource(name)
}

// RegisterResource registers a resource to be shared with other plugins.
// Returns an error if registration fails.
func (r *TypedRuntimePlugin) RegisterResource(name string, resource any) error {
	return r.runtime.RegisterResource(name, resource)
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
	cfg := r.runtime.GetConfig()
	if cfg == nil {
		if app := Lynx(); app != nil {
			if globalCfg := app.GetGlobalConfig(); globalCfg != nil {
				r.runtime.SetConfig(globalCfg)
				return globalCfg
			}
		}
	}
	return cfg
}

// GetLogger returns the plugin logger instance.
// Provides structured logging capabilities.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	return r.runtime.GetLogger()
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
		if logger := r.runtime.GetLogger(); logger != nil {
			_ = logger.Log(log.LevelError, "msg", "failed to publish event", "type", event.Type, "plugin", event.PluginID, "error", err)
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

// GetPrivateResource gets a private resource for the current plugin context
func (r *TypedRuntimePlugin) GetPrivateResource(name string) (any, error) {
	return r.runtime.GetPrivateResource(name)
}

// RegisterPrivateResource registers a private resource for the current plugin context
func (r *TypedRuntimePlugin) RegisterPrivateResource(name string, resource any) error {
	return r.runtime.RegisterPrivateResource(name, resource)
}

// GetSharedResource gets a shared resource accessible by all plugins
func (r *TypedRuntimePlugin) GetSharedResource(name string) (any, error) {
	return r.runtime.GetSharedResource(name)
}

// RegisterSharedResource registers a shared resource accessible by all plugins
func (r *TypedRuntimePlugin) RegisterSharedResource(name string, resource any) error {
	return r.runtime.RegisterSharedResource(name, resource)
}

// EmitPluginEvent emits a plugin-specific event
func (r *TypedRuntimePlugin) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	r.runtime.EmitPluginEvent(pluginName, eventType, data)
}

// AddPluginListener adds a listener for plugin-specific events
func (r *TypedRuntimePlugin) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	r.runtime.AddPluginListener(pluginName, listener, filter)
}

// GetPluginEventHistory gets event history for a specific plugin
func (r *TypedRuntimePlugin) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	return r.runtime.GetPluginEventHistory(pluginName, filter)
}

// SetEventDispatchMode sets the event dispatch mode
func (r *TypedRuntimePlugin) SetEventDispatchMode(mode string) error {
	return r.runtime.SetEventDispatchMode(mode)
}

// SetEventWorkerPoolSize sets the event worker pool size
func (r *TypedRuntimePlugin) SetEventWorkerPoolSize(size int) {
	r.runtime.SetEventWorkerPoolSize(size)
}

// SetEventTimeout sets the event processing timeout
func (r *TypedRuntimePlugin) SetEventTimeout(timeout time.Duration) {
	r.runtime.SetEventTimeout(timeout)
}

// GetEventStats gets event system statistics
func (r *TypedRuntimePlugin) GetEventStats() map[string]any {
	return r.runtime.GetEventStats()
}

// WithPluginContext creates a runtime with plugin context
func (r *TypedRuntimePlugin) WithPluginContext(pluginName string) plugins.Runtime {
	return r.runtime.WithPluginContext(pluginName)
}

// GetCurrentPluginContext gets the current plugin context
func (r *TypedRuntimePlugin) GetCurrentPluginContext() string {
	return r.runtime.GetCurrentPluginContext()
}

// SetConfig sets the configuration
func (r *TypedRuntimePlugin) SetConfig(conf config.Config) {
	r.runtime.SetConfig(conf)
}

// GetResourceInfo gets resource information
func (r *TypedRuntimePlugin) GetResourceInfo(name string) (*plugins.ResourceInfo, error) {
	return r.runtime.GetResourceInfo(name)
}

// ListResources lists all resources
func (r *TypedRuntimePlugin) ListResources() []*plugins.ResourceInfo {
	return r.runtime.ListResources()
}

// CleanupResources cleans up resources for a specific plugin
func (r *TypedRuntimePlugin) CleanupResources(pluginID string) error {
	return r.runtime.CleanupResources(pluginID)
}

// GetResourceStats gets resource statistics
func (r *TypedRuntimePlugin) GetResourceStats() map[string]any {
	return r.runtime.GetResourceStats()
}

// RemoveListener unregisters an event listener.
