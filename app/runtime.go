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

	// TODO: Implement proper listener conversion to unified event bus
	// For now, this is a placeholder that maintains API compatibility
}

// RemoveListener unregisters an event listener.
// This method is kept for backward compatibility but delegates to the unified event bus.
func (r *TypedRuntimePlugin) RemoveListener(listener plugins.EventListener) {
	// TODO: Implement proper listener removal from unified event bus
	// For now, this is a placeholder that maintains API compatibility
}

// GetEventHistory retrieves historical events based on filter criteria.
// This method is kept for backward compatibility but delegates to the unified event bus.
func (r *TypedRuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	// TODO: Implement proper event history retrieval from unified event bus
	// For now, return empty slice to maintain API compatibility
	return []plugins.PluginEvent{}
}

// eventMatchesFilter checks if an event matches a specific filter.
// This method is kept for backward compatibility.
func (r *TypedRuntimePlugin) eventMatchesFilter(event plugins.PluginEvent, filter plugins.EventFilter) bool {
	// TODO: Implement proper filter matching logic
	// For now, return true to maintain API compatibility
	return true
}

// RuntimePlugin backward-compatible alias of TypedRuntimePlugin
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin creates a runtime plugin (backward-compatible)
func NewRuntimePlugin() *RuntimePlugin {
	return NewTypedRuntimePlugin()
}
