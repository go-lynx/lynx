// Package plugins provides a plugin system for extending application functionality.
package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType represents the type of event that occurred in the plugin system.
type EventType string

// Priority levels for plugin events
const (
	// PriorityLow indicates minimal impact events that can be processed later
	PriorityLow = 0
	// PriorityNormal indicates standard events requiring routine processing
	PriorityNormal = 1
	// PriorityHigh indicates important events needing prompt attention
	PriorityHigh = 2
	// PriorityCritical indicates urgent events requiring immediate handling
	PriorityCritical = 3
)

// Plugin lifecycle event strings, used as the Type field in PluginEvent.
const (
	EventPluginInitializing = "plugin.initializing"
	EventPluginInitialized  = "plugin.initialized"
	EventPluginStarting     = "plugin.starting"
	EventPluginStarted      = "plugin.started"
	EventPluginStopping     = "plugin.stopping"
	EventPluginStopped      = "plugin.stopped"
)

// Health check event strings.
const (
	EventHealthCheckStarted   = "health.check.started"
	EventHealthCheckRunning   = "health.check.running"
	EventHealthCheckDone      = "health.check.done"
	EventHealthStatusOK       = "health.status.ok"
	EventHealthStatusWarning  = "health.status.warning"
	EventHealthStatusCritical = "health.status.critical"
	EventHealthStatusUnknown  = "health.status.unknown"
	EventHealthMetricsChanged = "health.metrics.changed"
	EventHealthThresholdHit   = "health.metrics.threshold"
	EventHealthStatusChanged  = "health.status.changed"
	EventHealthCheckFailed    = "health.check.failed"
)

// Resource utilisation event strings.
const (
	EventResourceExhausted   = "resource.exhausted"
	EventPerformanceDegraded = "performance.degraded"
)

// Configuration change event strings.
const (
	EventConfigurationChanged = "config.changed"
	EventConfigurationInvalid = "config.invalid"
	EventConfigurationApplied = "config.applied"
)

// Dependency state event strings.
const (
	EventDependencyMissing       = "dependency.missing"
	EventDependencyStatusChanged = "dependency.status.changed"
	EventDependencyError         = "dependency.error"
)

// Security event strings.
const (
	EventSecurityViolation    = "security.violation"
	EventAuthenticationFailed = "auth.failed"
	EventAuthorizationDenied  = "auth.denied"
)

// Resource lifecycle event strings.
const (
	EventResourceCreated      = "resource.created"
	EventResourceModified     = "resource.modified"
	EventResourceDeleted      = "resource.deleted"
	EventResourceUnavailable  = "resource.unavailable"
)

// Error event strings.
const (
	EventErrorOccurred  = "error.occurred"
	EventErrorResolved  = "error.resolved"
	EventPanicRecovered = "panic.recovered"
)

// System-level event types emitted by the plugin manager infrastructure itself
const (
	// EventPluginManagerShutdown indicates the plugin manager has begun shutdown.
	// Emitted once before UnloadPlugins stops individual plugins.
	EventPluginManagerShutdown EventType = "system.plugin_manager_shutdown"
)

// PluginEvent represents a lifecycle event in the plugin system.
// It contains detailed information about the event, including its type,
// priority, source, and any associated metadata.
type PluginEvent struct {
	// Type indicates the specific kind of event that occurred
	Type EventType

	// Priority indicates the importance level of the event
	Priority int

	// PluginID identifies the plugin that generated the event
	PluginID string

	// Source identifies where in the plugin the event originated
	Source string

	// Category groups related events for easier filtering
	Category string

	// Status represents the plugin's state when event occurred
	Status PluginStatus

	// Error contains any error information if applicable
	Error error

	// Metadata contains additional event-specific information
	Metadata map[string]any

	// Timestamp records when the event occurred
	Timestamp int64
}

// EventFilter defines criteria for filtering plugin events.
// It allows selective processing of events based on various attributes.
type EventFilter struct {
	// Types specifies which event types to include
	Types []EventType

	// Priorities specifies which priority levels to include
	Priorities []int

	// PluginIDs specifies which plugins to monitor
	PluginIDs []string

	// Categories specifies which event categories to include
	Categories []string

	// FromTime specifies the start time for event filtering
	FromTime int64

	// ToTime specifies the end time for event filtering
	ToTime int64
}

// Event wraps event payloads with a concrete type so adapters can avoid untyped values
// on the hot path while keeping the legacy PluginEvent API intact.
type Event[T any] struct {
	Payload T
}

// NewEvent creates a typed event envelope.
func NewEvent[T any](payload T) Event[T] {
	return Event[T]{Payload: payload}
}

// Unwrap returns the original typed payload.
func (e Event[T]) Unwrap() T {
	return e.Payload
}

// TypedEventProcessor provides a type-safe event processor for new integrations.
type TypedEventProcessor[T any] interface {
	ProcessTypedEvent(event Event[T]) bool
	AddFilter(filter EventFilter)
	RemoveFilter(filterID string)
}

// TypedEventEmitter provides type-safe event publishing/listening for new integrations.
type TypedEventEmitter[T any] interface {
	EmitTypedEvent(event Event[T])
	AddTypedListener(listener TypedEventListener[T], filter *EventFilter)
	RemoveTypedListener(listener TypedEventListener[T])
	GetTypedEventHistory(filter EventFilter) []Event[T]
}

// TypedEventListener provides type-safe event handling for new integrations.
type TypedEventListener[T any] interface {
	HandleTypedEvent(event Event[T])
	GetListenerID() string
}

// EventProcessor provides event processing and filtering capabilities.
type EventProcessor interface {
	// ProcessEvent processes an event through all registered filters.
	// Returns true if the event should be propagated, false if it should be filtered.
	ProcessEvent(event PluginEvent) bool

	// AddFilter adds a new event filter to the processor.
	// Filter will be applied to all subsequent events.
	AddFilter(filter EventFilter)

	// RemoveFilter removes an event filter by its ID.
	// Events will no longer be filtered by the removed filter.
	RemoveFilter(filterID string)
}

// EventEmitter defines the interface for the plugin event system.
type EventEmitter interface {
	// EmitEvent broadcasts a plugin event to all registered listeners.
	// Event will be processed according to its priority and any active filters.
	EmitEvent(event PluginEvent)

	// AddListener registers a new event listener with optional filters.
	// Listener will only receive events that match its filter criteria.
	AddListener(listener EventListener, filter *EventFilter)

	// RemoveListener unregisters an event listener.
	// After removal, the listener will no longer receive any events.
	RemoveListener(listener EventListener)

	// GetEventHistory retrieves historical events based on filter criteria.
	// Returns events that match the specified filter parameters.
	GetEventHistory(filter EventFilter) []PluginEvent
}

// EventListener defines the interface for handling plugin events.
type EventListener interface {
	// HandleEvent processes plugin lifecycle events.
	// Implementation should handle the event according to its type and priority.
	HandleEvent(event PluginEvent)

	// GetListenerID returns a stable unique identifier for this listener.
	// The same logical listener must return the same ID on every call so that RemoveListener
	// can correctly unregister it. Avoid using pointer addresses (e.g. fmt.Sprintf("%p", l))
	// if the listener struct is recreated between Add and Remove.
	GetListenerID() string
}

// globalEventHooks holds injected runtime callbacks so that the plugins package
// avoids a circular import on the lynx root package.
var globalEventHooks struct {
	mu      sync.RWMutex
	emitter func(PluginEvent)
	adder   func(EventListener, *EventFilter)
}

// SetGlobalEventHooks wires up the runtime event emitter and listener adder.
// This must be called once during framework initialisation (typically from lynx.App).
// Calling it a second time replaces the previous hooks.
func SetGlobalEventHooks(emitter func(PluginEvent), adder func(EventListener, *EventFilter)) {
	globalEventHooks.mu.Lock()
	defer globalEventHooks.mu.Unlock()
	globalEventHooks.emitter = emitter
	globalEventHooks.adder = adder
}

// Subscribe registers a typed event listener with optional filtering.
// The listener function receives strongly-typed event payloads.
// Example:
//
//	type OrderCreated struct { OrderID string }
//	Subscribe[OrderCreated](func(ctx context.Context, event OrderCreated) error)
func Subscribe[T any](listener func(ctx context.Context, event T) error, filter *EventFilter) {
	globalEventHooks.mu.RLock()
	adder := globalEventHooks.adder
	globalEventHooks.mu.RUnlock()
	if adder == nil {
		return
	}
	wrapper := &typedListenerAdapter[T]{
		handler: listener,
		id:      fmt.Sprintf("typed-%T-%d", (*T)(nil), time.Now().UnixNano()),
	}
	adder(wrapper, filter)
}

// Publish broadcasts a typed event to all registered listeners.
// The event payload is wrapped in an Event envelope for type safety.
// Example:
//
//	type OrderCreated struct { OrderID string }
//	Publish(OrderCreated{OrderID: "123"})
func Publish[T any](payload T) {
	globalEventHooks.mu.RLock()
	emitter := globalEventHooks.emitter
	globalEventHooks.mu.RUnlock()
	if emitter == nil {
		return
	}
	pluginEvent := PluginEvent{
		Type:      EventType(fmt.Sprintf("typed.%T", payload)),
		Priority:  PriorityNormal,
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"payload": payload},
	}
	emitter(pluginEvent)
}

// typedListenerAdapter adapts a typed listener function to the EventListener interface.
type typedListenerAdapter[T any] struct {
	handler func(ctx context.Context, event T) error
	id      string
}

// HandleEvent implements EventListener by unwrapping typed events.
func (a *typedListenerAdapter[T]) HandleEvent(event PluginEvent) {
	ctx := context.Background()
	if payload, ok := event.Metadata["payload"].(T); ok {
		_ = a.handler(ctx, payload)
	}
}

// GetListenerID returns the unique identifier for this listener.
func (a *typedListenerAdapter[T]) GetListenerID() string {
	return a.id
}
