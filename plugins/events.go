// Package plugins provides a plugin system for extending application functionality.
package plugins

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

// Upgrade event types for managing plugin versions
const (
	// EventUpgradeAvailable indicates new version availability.
	// Triggered when update check finds newer version.
	EventUpgradeAvailable = "upgrade.available"

	// EventUpgradeInitiated indicates upgrade process start.
	// Triggered when upgrade sequence begins.
	EventUpgradeInitiated = "upgrade.initiated"

	// EventUpgradeValidating indicates upgrade validation.
	// Triggered when validating system state before upgrade.
	EventUpgradeValidating = "upgrade.validating"

	// EventUpgradeInProgress indicates that the upgrade process is ongoing.
	// Triggered when the upgrade process is in progress.
	EventUpgradeInProgress = "upgrade.in_progress"

	// EventUpgradeCompleted indicates successful upgrade.
	// Triggered when new version is installed and verified.
	EventUpgradeCompleted = "upgrade.completed"

	// EventUpgradeFailed indicates failed upgrade attempt.
	// Triggered when upgrade process encounters error.
	EventUpgradeFailed = "upgrade.failed"

	// EventRollbackInitiated indicates version rollback start.
	// Triggered when rollback to previous version begins.
	EventRollbackInitiated = "rollback.initiated"

	// EventRollbackInProgress indicates that the rollback process is ongoing.
	// Triggered when the rollback process has started and is in progress.
	EventRollbackInProgress = "rollback.in_progress"

	// EventRollbackCompleted indicates successful rollback.
	// Triggered when previous version is restored.
	EventRollbackCompleted = "rollback.completed"

	// EventRollbackFailed indicates failed rollback attempt.
	// Triggered when unable to restore previous version.
	EventRollbackFailed = "rollback.failed"
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

	// GetListenerID returns a unique identifier for the listener.
	// Used for listener management and filtering.
	GetListenerID() string
}
