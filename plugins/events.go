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

// Plugin lifecycle event types for comprehensive system monitoring
const (
	// EventPluginInitializing indicates the plugin is starting initialization.
	// Triggered when plugin begins loading resources and establishing connections.
	EventPluginInitializing = "plugin.initializing"

	// EventPluginInitialized indicates the plugin completed initialization.
	// Triggered when all resources are loaded and connections established.
	EventPluginInitialized = "plugin.initialized"

	// EventPluginStarting indicates the plugin is beginning its operations.
	// Triggered when core functionality is about to begin.
	EventPluginStarting = "plugin.starting"

	// EventPluginStarted indicates the plugin is fully operational.
	// Triggered when all systems are running and ready to handle requests.
	EventPluginStarted = "plugin.started"

	// EventPluginStopping indicates the plugin is beginning shutdown.
	// Triggered when shutdown command is received and cleanup begins.
	EventPluginStopping = "plugin.stopping"

	// EventPluginStopped indicates the plugin completed shutdown.
	// Triggered when all resources are released and connections closed.
	EventPluginStopped = "plugin.stopped"
)

// Health check event types for monitoring plugin health status
const (
	// EventHealthCheckStarted indicates a health check operation has begun.
	// Triggered when the health check routine starts executing.
	EventHealthCheckStarted = "health.check.started"

	// EventHealthCheckRunning indicates a health check is in progress.
	// Triggered during the execution of health check procedures.
	EventHealthCheckRunning = "health.check.running"

	// EventHealthCheckDone indicates a health check has completed.
	// Triggered when all health check procedures have finished.
	EventHealthCheckDone = "health.check.done"

	// EventHealthStatusOK indicates the plugin is healthy.
	// Triggered when all health metrics are within normal ranges.
	EventHealthStatusOK = "health.status.ok"

	// EventHealthStatusWarning indicates potential health issues.
	// Triggered when health metrics show concerning trends.
	EventHealthStatusWarning = "health.status.warning"

	// EventHealthStatusCritical indicates severe health issues.
	// Triggered when health metrics exceed critical thresholds.
	EventHealthStatusCritical = "health.status.critical"

	// EventHealthStatusUnknown indicates health status cannot be determined.
	// Triggered when health check procedures fail to complete.
	EventHealthStatusUnknown = "health.status.unknown"

	// EventHealthMetricsChanged indicates a change in health metrics.
	// Triggered when monitored metrics show significant changes.
	EventHealthMetricsChanged = "health.metrics.changed"

	// EventHealthThresholdHit indicates metrics exceeded defined thresholds.
	// Triggered when health metrics cross warning or critical levels.
	EventHealthThresholdHit = "health.metrics.threshold"

	// EventHealthStatusChanged indicates overall health status change.
	// Triggered when the aggregate health status transitions.
	EventHealthStatusChanged = "health.status.changed"

	// EventHealthCheckFailed indicates health check operation failure.
	// Triggered when health check procedures encounter errors.
	EventHealthCheckFailed = "health.check.failed"
)

// Resource event types for monitoring system resources
const (
	// EventResourceExhausted indicates critical resource depletion.
	// Triggered when system resources reach critical levels.
	EventResourceExhausted = "resource.exhausted"

	// EventPerformanceDegraded indicates performance deterioration.
	// Triggered when system performance metrics decline significantly.
	EventPerformanceDegraded = "performance.degraded"
)

// Configuration event types for managing plugin configuration
const (
	// EventConfigurationChanged indicates configuration update initiation.
	// Triggered when new configuration is being applied.
	EventConfigurationChanged = "config.changed"

	// EventConfigurationInvalid indicates invalid configuration.
	// Triggered when configuration validation fails.
	EventConfigurationInvalid = "config.invalid"

	// EventConfigurationApplied indicates successful configuration update.
	// Triggered when new configuration is active and verified.
	EventConfigurationApplied = "config.applied"
)

// Dependency event types for managing plugin dependencies
const (
	// EventDependencyMissing indicates missing required dependency.
	// Triggered when required plugin or resource is unavailable.
	EventDependencyMissing = "dependency.missing"

	// EventDependencyStatusChanged indicates dependency state change.
	// Triggered when dependent plugin changes operational state.
	EventDependencyStatusChanged = "dependency.status.changed"

	// EventDependencyError indicates dependency-related error.
	// Triggered when dependency fails or becomes unstable.
	EventDependencyError = "dependency.error"
)

// Security event types for monitoring security-related events
const (
	// EventSecurityViolation indicates security policy breach.
	// Triggered when security rules are violated.
	EventSecurityViolation = "security.violation"

	// EventAuthenticationFailed indicates failed authentication.
	// Triggered when invalid credentials are used.
	EventAuthenticationFailed = "auth.failed"

	// EventAuthorizationDenied indicates unauthorized access.
	// Triggered when insufficient permissions are detected.
	EventAuthorizationDenied = "auth.denied"
)

// Resource lifecycle event types
const (
	// EventResourceCreated indicates new resource allocation.
	// Triggered when new resource is successfully created.
	EventResourceCreated = "resource.created"

	// EventResourceModified indicates resource modification.
	// Triggered when existing resource is updated.
	EventResourceModified = "resource.modified"

	// EventResourceDeleted indicates resource removal.
	// Triggered when resource is successfully deleted.
	EventResourceDeleted = "resource.deleted"

	// EventResourceUnavailable indicates resource access failure.
	// Triggered when resource becomes inaccessible.
	EventResourceUnavailable = "resource.unavailable"
)

// Error event types for error handling and recovery
const (
	// EventErrorOccurred indicates error detection.
	// Triggered when system encounters an error condition.
	EventErrorOccurred = "error.occurred"

	// EventErrorResolved indicates error recovery.
	// Triggered when error condition is successfully resolved.
	EventErrorResolved = "error.resolved"

	// EventPanicRecovered indicates panic recovery.
	// Triggered when system recovers from panic condition.
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
