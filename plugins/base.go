// Package plugins provides the core plugin system for the Lynx framework.
package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// TypedBasePlugin provides a generic base plugin with type-safe plugin foundation implementation
type TypedBasePlugin[T any] struct {
	// Basic plugin metadata
	id          string // Unique identifier for the plugin
	name        string // Human-readable name
	description string // Detailed description of functionality
	confPrefix  string // Configuration prefix
	version     string // Semantic version number

	// Operational state
	status  PluginStatus // Current plugin status
	runtime Runtime      // Runtime environment reference
	logger  log.Logger   // Plugin-specific logger

	// Event handling
	eventFilters []EventFilter // List of active event filters

	// Configuration
	config map[string]any // Plugin-specific configuration

	// Plugin weight for prioritization, higher values load first
	weight int // Plugin weight for prioritization

	// Dependency management
	dependencies []Dependency        // List of plugin dependencies
	capabilities []UpgradeCapability // List of plugin upgrade capabilities

	// Type-safe instance
	instance T
}

// StartContext provides a context-aware Start with timeout monitoring
// Plugins can override for true context awareness
func (p *TypedBasePlugin[T]) StartContext(ctx context.Context, plugin Plugin) error {
	// Check if context is already canceled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	// Create a done channel to signal completion
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Start: %v", r)
			}
		}()
		done <- p.Start(plugin)
	}()

	// Wait for either completion or context cancellation
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context was canceled, but we can't stop the running Start method
		return fmt.Errorf("start canceled by context: %w", ctx.Err())
	}
}

// StopContext provides a context-aware Stop with timeout monitoring
// Plugins can override for true context awareness
func (p *TypedBasePlugin[T]) StopContext(ctx context.Context, plugin Plugin) error {
	// Check if context is already canceled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before stop: %w", err)
	}

	// Create a done channel to signal completion
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Stop: %v", r)
			}
		}()
		done <- p.Stop(plugin)
	}()

	// Wait for either completion or context cancellation
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context was canceled, but we can't stop the running Stop method
		return fmt.Errorf("stop canceled by context: %w", ctx.Err())
	}
}

// InitializeContext provides a context-aware Initialize with timeout monitoring
// Plugins can override for true context awareness
func (p *TypedBasePlugin[T]) InitializeContext(ctx context.Context, plugin Plugin, rt Runtime) error {
	// Check if context is already canceled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before initialize: %w", err)
	}

	// Create a done channel to signal completion
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Initialize: %v", r)
			}
		}()
		done <- p.Initialize(plugin, rt)
	}()

	// Wait for either completion or context cancellation
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context was canceled, but we can't stop the running Initialize method
		return fmt.Errorf("initialize canceled by context: %w", ctx.Err())
	}
}

// IsContextAware returns false by default for base plugin
// Subclasses should override this if they truly respect context cancellation
func (p *TypedBasePlugin[T]) IsContextAware() bool {
	return false // Base implementation is not truly context-aware
}

// NewTypedBasePlugin creates a new instance of TypedBasePlugin with the provided metadata.
// This is the recommended way to initialize a new typed plugin implementation.
func NewTypedBasePlugin[T any](
	id, name, description, version, confPrefix string,
	weight int,
	instance T,
) *TypedBasePlugin[T] {
	return &TypedBasePlugin[T]{
		id:           id,
		name:         name,
		description:  description,
		version:      version,
		status:       StatusInactive,
		confPrefix:   confPrefix,
		eventFilters: make([]EventFilter, 0),
		config:       make(map[string]any),
		dependencies: make([]Dependency, 0),
		weight:       weight,
		capabilities: []UpgradeCapability{UpgradeNone},
		instance:     instance,
	}
}

// GetTypedInstance returns the type-safe instance
func (p *TypedBasePlugin[T]) GetTypedInstance() T {
	return p.instance
}

// Initialize prepares the plugin for use by setting up its runtime environment.
// This method must be called before the plugin can be started.
func (p *TypedBasePlugin[T]) Initialize(plugin Plugin, rt Runtime) error {
	if rt == nil {
		return ErrPluginNotInitialized
	}

	p.runtime = rt
	p.logger = rt.GetLogger()
	p.status = StatusInitializing

	// Emit event indicating plugin is initializing
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitializing,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	// Call InitializeResources for custom initialization
	if err := plugin.InitializeResources(rt); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Initialize", "Failed to initialize resources", err)
	}

	p.status = StatusInactive
	// Emit event indicating plugin has been initialized
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitialized,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	return nil
}

// Start activates the plugin and begins its main operations.
// The plugin must be initialized before it can be started.
func (p *TypedBasePlugin[T]) Start(plugin Plugin) error {
	if p.status == StatusActive {
		return ErrPluginAlreadyActive
	}

	if p.runtime == nil {
		return ErrPluginNotInitialized
	}

	p.status = StatusInitializing
	// Emit event indicating plugin is starting
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	// Call StartupTasks for custom startup logic
	if err := plugin.StartupTasks(); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Start", "Failed to perform startup tasks", err)
	}

	p.status = StatusActive
	// Emit event indicating plugin has started
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	// Check health status after ready
	err := plugin.CheckHealth()
	if err != nil {
		log.Errorf("Plugin %s health check failed: %v", plugin.Name(), err)
		return fmt.Errorf("plugin %s health check failed: %w", plugin.Name(), err)
	}
	return nil
}

// Stop gracefully terminates the plugin's operations.
// This method should release all resources and perform cleanup.
func (p *TypedBasePlugin[T]) Stop(plugin Plugin) error {
	if p.status != StatusActive {
		return NewPluginError(p.id, "Stop", "Plugin must be active to stop", ErrPluginNotActive)
	}

	p.status = StatusStopping
	// Emit event indicating plugin is stopping
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	// Call CleanupTasks for custom cleanup logic
	if err := plugin.CleanupTasks(); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Stop", "Failed to perform cleanup tasks", err)
	}

	p.status = StatusTerminated
	// Emit event indicating plugin has stopped
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopped,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	return nil
}

// Status returns the current operational status of the plugin.
// This method is thread-safe and can be called at any time.
func (p *TypedBasePlugin[T]) Status(plugin Plugin) PluginStatus {
	return p.status
}

// InitializeResources sets up the plugin's required resources.
// This method can be overridden by embedding structs to provide custom initialization.
func (p *TypedBasePlugin[T]) InitializeResources(rt Runtime) error {
	return nil
}

// StartupTasks performs necessary tasks during plugin startup.
// This method can be overridden by embedding structs to provide custom startup logic.
func (p *TypedBasePlugin[T]) StartupTasks() error {
	return nil
}

// CleanupTasks performs cleanup during plugin shutdown.
// This method can be overridden by embedding structs to provide custom cleanup logic.
func (p *TypedBasePlugin[T]) CleanupTasks() error {
	return nil
}

// ID returns the unique identifier of the plugin.
// This ID must be unique across all plugins in the system.
func (p *TypedBasePlugin[T]) ID() string {
	return p.id
}

// Name returns the human-readable name of the plugin.
// This name is used for display and logging purposes.
func (p *TypedBasePlugin[T]) Name() string {
	return p.name
}

// Weight returns the plugin weight for prioritization
func (p *TypedBasePlugin[T]) Weight() int {
	return p.weight
}

// Description returns a detailed description of the plugin's functionality.
// This helps users understand the plugin's purpose and capabilities.
func (p *TypedBasePlugin[T]) Description() string {
	return p.description
}

// Version returns the semantic version of the plugin.
// Version format should follow semver conventions (MAJOR.MINOR.PATCH).
func (p *TypedBasePlugin[T]) Version() string {
	return p.version
}

// SetStatus sets the current operational status of the plugin.
// This method is thread-safe and should be used to update plugin status.
func (p *TypedBasePlugin[T]) SetStatus(status PluginStatus) {
	p.status = status
}

// GetHealth performs a health check and returns a detailed health report.
// This method should be called periodically to monitor plugin health.
func (p *TypedBasePlugin[T]) GetHealth() HealthReport {
	report := HealthReport{
		Status:    "unknown",
		Details:   make(map[string]any),
		Timestamp: time.Now().Unix(),
	}

	// Emit event indicating health check has started
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckStarted,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Check if plugin is in a valid state for health check
	switch p.status {
	case StatusTerminated, StatusFailed:
		report.Status = "unhealthy"
		report.Message = "Plugin is not operational"
		// Emit event indicating critical health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusCritical,
			Priority: PriorityHigh,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusSuspended:
		report.Status = "suspended"
		report.Message = "Plugin is temporarily suspended"
		// Emit event indicating warning health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusUpgrading:
		report.Status = "upgrading"
		report.Message = "Plugin is being upgraded"
		// Emit event indicating unknown health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusRollback:
		report.Status = "rolling-back"
		report.Message = "Plugin is rolling back to previous version"
		// Emit event indicating unknown health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusInitializing:
		report.Status = "initializing"
		report.Message = "Plugin is being initialized"
		// Emit event indicating unknown health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusInactive:
		report.Status = "inactive"
		report.Message = "Plugin is not yet started"
		// Emit event indicating warning health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusStopping:
		report.Status = "stopping"
		report.Message = "Plugin is shutting down"
		// Emit event indicating warning health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	default:
		report.Status = "unhealthy"
		report.Message = "Plugin status is unknown"

	}

	// Emit event indicating health check is running
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckRunning,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Perform health check for active plugin
	if err := p.CheckHealth(); err != nil {
		report.Status = "unhealthy"
		report.Message = err.Error()
		// Emit event indicating health check failed
		p.EmitEvent(PluginEvent{
			Type:     EventHealthCheckFailed,
			Priority: PriorityHigh,
			Source:   "GetHealth",
			Category: "health",
			Error:    err,
		})
		return report
	}

	// Emit event indicating health check is done
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckDone,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Emit appropriate health status event
	if report.Status == "healthy" {
		// Emit event indicating OK health status
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusOK,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
	}

	return report
}

// Configure updates the plugin's configuration with the provided settings.
// This method validates and applies new configuration values.
func (p *TypedBasePlugin[T]) Configure(conf any) error {
	// Emit event indicating configuration has changed
	p.EmitEvent(PluginEvent{
		Type:     EventConfigurationChanged,
		Priority: PriorityNormal,
		Source:   "Configure",
		Category: "configuration",
	})

	// Validate and apply configuration
	if err := p.ValidateConfig(conf); err != nil {
		// Emit event indicating invalid configuration
		p.EmitEvent(PluginEvent{
			Type:     EventConfigurationInvalid,
			Priority: PriorityHigh,
			Source:   "Configure",
			Category: "configuration",
			Error:    err,
		})
		return NewPluginError(p.id, "Configure", "Invalid configuration", err)
	}

	if err := p.ApplyConfig(conf); err != nil {
		return NewPluginError(p.id, "Configure", "Failed to apply configuration", err)
	}

	// Emit event indicating configuration has been applied
	p.EmitEvent(PluginEvent{
		Type:     EventConfigurationApplied,
		Priority: PriorityNormal,
		Source:   "Configure",
		Category: "configuration",
	})

	return nil
}

// GetDependencies returns the list of plugin dependencies.
// This includes both required and optional dependencies.
func (p *TypedBasePlugin[T]) GetDependencies() []Dependency {
	return p.dependencies
}

// AddDependency adds a new dependency to the plugin.
// The dependency will be validated during plugin initialization.
func (p *TypedBasePlugin[T]) AddDependency(dep Dependency) {
	p.dependencies = append(p.dependencies, dep)
	// Emit event indicating dependency status has changed
	p.EmitEvent(PluginEvent{
		Type:     EventDependencyStatusChanged,
		Priority: PriorityNormal,
		Source:   "AddDependency",
		Category: "dependency",
		Metadata: map[string]any{
			"dependency": dep,
		},
	})
}

// AddEventFilter adds a new event filter to the plugin.
// Events will be filtered according to the specified criteria.
func (p *TypedBasePlugin[T]) AddEventFilter(filter EventFilter) {
	p.eventFilters = append(p.eventFilters, filter)
}

// RemoveEventFilter removes an event filter from the plugin.
// This affects how future events will be processed.
func (p *TypedBasePlugin[T]) RemoveEventFilter(index int) {
	if index >= 0 && index < len(p.eventFilters) {
		p.eventFilters = append(p.eventFilters[:index], p.eventFilters[index+1:]...)
	}
}

// HandleEvent processes incoming plugin events.
// Events are filtered and handled according to configured filters.
func (p *TypedBasePlugin[T]) HandleEvent(event PluginEvent) {
	if !p.ShouldHandleEvent(event) {
		return
	}

	// Process the event based on type
	switch event.Type {
	case EventHealthStatusChanged:
		p.HandleHealthEvent(event)
	case EventConfigurationChanged:
		p.HandleConfigEvent(event)
	case EventDependencyStatusChanged:
		p.HandleDependencyEvent(event)
	default:
		p.HandleDefaultEvent(event)
	}
}

// EmitEvent emits an event to the runtime event system.
// This method adds standard fields to the event before emission.
func (p *TypedBasePlugin[T]) EmitEvent(event PluginEvent) {
	p.EmitEventInternal(event)
}

// EmitEventInternal emits an event to the unified event bus system.
// This method adds standard fields to the event before emission.
func (p *TypedBasePlugin[T]) EmitEventInternal(event PluginEvent) {
	// Add standard fields
	event.PluginID = p.id
	event.Status = p.status
	event.Timestamp = time.Now().Unix()

	// Apply filters
	if p.ShouldEmitEvent(event) {
		// Use runtime's EmitEvent which now delegates to unified event bus
		p.runtime.EmitEvent(event)
	}
}

// ShouldEmitEvent checks if an event should be emitted based on filters.
// This implements the event filtering logic.
func (p *TypedBasePlugin[T]) ShouldEmitEvent(event PluginEvent) bool {
	if len(p.eventFilters) == 0 {
		return true
	}

	for _, filter := range p.eventFilters {
		if p.EventMatchesFilter(event, filter) {
			return true
		}
	}

	return false
}

// ShouldHandleEvent checks if an event should be handled based on filters.
// This implements the event handling filter logic.
func (p *TypedBasePlugin[T]) ShouldHandleEvent(event PluginEvent) bool {
	return p.ShouldEmitEvent(event)
}

// EventMatchesFilter checks if an event matches a specific filter.
// This implements the detailed filter matching logic.
func (p *TypedBasePlugin[T]) EventMatchesFilter(event PluginEvent, filter EventFilter) bool {
	// Check event type
	if len(filter.Types) > 0 {
		typeMatch := false
		for _, t := range filter.Types {
			if event.Type == t {
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
		for _, p := range filter.Priorities {
			if event.Priority == p {
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
		idMatch := false
		for _, id := range filter.PluginIDs {
			if event.PluginID == id {
				idMatch = true
				break
			}
		}
		if !idMatch {
			return false
		}
	}

	// Check category
	if len(filter.Categories) > 0 {
		categoryMatch := false
		for _, c := range filter.Categories {
			if event.Category == c {
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

// CheckHealth performs the actual health check operations.
// This is called during health status reporting.
func (p *TypedBasePlugin[T]) CheckHealth() error {
	// Implementation-specific health checks
	return nil
}

// ValidateConfig validates the provided configuration.
// This is called before applying new configuration.
func (p *TypedBasePlugin[T]) ValidateConfig(conf any) error {
	// Implementation-specific configuration validation
	return nil
}

// ApplyConfig applies the validated configuration.
// This is called after configuration validation succeeds.
func (p *TypedBasePlugin[T]) ApplyConfig(conf any) error {
	// Implementation-specific configuration application
	return nil
}

// HandleHealthEvent processes health-related events.
// This implements specific handling for health events.
func (p *TypedBasePlugin[T]) HandleHealthEvent(event PluginEvent) {
	// Implementation-specific health event handling
}

// HandleConfigEvent processes configuration-related events.
// This implements specific handling for configuration events.
func (p *TypedBasePlugin[T]) HandleConfigEvent(event PluginEvent) {
	// Implementation-specific configuration event handling
}

// HandleDependencyEvent processes dependency-related events.
// This implements specific handling for dependency events.
func (p *TypedBasePlugin[T]) HandleDependencyEvent(event PluginEvent) {
	// Implementation-specific dependency event handling
}

// HandleDefaultEvent processes events that don't have specific handlers.
// This implements default event handling behavior.
func (p *TypedBasePlugin[T]) HandleDefaultEvent(event PluginEvent) {
	// Implementation-specific default event handling
}

// Suspend temporarily suspends the plugin.
// This method checks if the plugin is in the active state.
func (p *TypedBasePlugin[T]) Suspend() error {
	if p.status != StatusActive {
		return NewPluginError(p.id, "Suspend", "Plugin must be active to suspend", ErrPluginNotActive)
	}

	p.status = StatusStopping
	// Emit event indicating plugin is stopping
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Suspend",
		Category: "lifecycle",
	})

	// Perform any suspension tasks here if needed
	p.status = StatusSuspended
	return nil
}

// Resume resumes the plugin from suspended state.
// This method checks if the plugin is in the suspended state.
func (p *TypedBasePlugin[T]) Resume() error {
	if p.status != StatusSuspended {
		return NewPluginError(p.id, "Resume", "Plugin must be suspended to resume", ErrPluginNotActive)
	}

	// Emit event indicating plugin is starting
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	p.status = StatusActive

	// Emit event indicating plugin has started
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	return nil
}

// PrepareUpgrade prepares the plugin for upgrade.
// This method checks if the plugin supports the upgrade capability.
func (p *TypedBasePlugin[T]) PrepareUpgrade(targetVersion string) error {
	if !p.SupportsCapability(UpgradeConfig) && !p.SupportsCapability(UpgradeVersion) {
		return NewPluginError(p.id, "PrepareUpgrade", "Upgrade not supported", ErrPluginUpgradeNotSupported)
	}

	if p.status != StatusActive {
		return NewPluginError(p.id, "PrepareUpgrade", "Plugin must be active to upgrade", ErrPluginNotActive)
	}

	// Emit event indicating upgrade has been initiated
	p.EmitEvent(PluginEvent{
		Type:     EventUpgradeInitiated,
		Priority: PriorityHigh,
		Source:   "PrepareUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"targetVersion":  targetVersion,
			"currentVersion": p.version,
		},
	})

	p.status = StatusUpgrading
	return nil
}

// ExecuteUpgrade performs the plugin upgrade.
// This method checks if the plugin is in the upgrading state.
func (p *TypedBasePlugin[T]) ExecuteUpgrade(targetVersion string) error {
	if p.status != StatusUpgrading {
		return NewPluginError(p.id, "ExecuteUpgrade", "Plugin must be in upgrading state", ErrPluginNotActive)
	}

	// Perform upgrade tasks
	if err := p.PerformUpgrade(targetVersion); err != nil {
		// Emit event indicating upgrade failed
		p.EmitEvent(PluginEvent{
			Type:     EventUpgradeFailed,
			Priority: PriorityCritical,
			Source:   "ExecuteUpgrade",
			Category: "upgrade",
			Error:    err,
			Metadata: map[string]any{
				"targetVersion":  targetVersion,
				"currentVersion": p.version,
			},
		})

		// Attempt automatic rollback
		if rollbackErr := p.RollbackUpgrade(p.version); rollbackErr != nil {
			// If rollback fails, plugin is in an inconsistent state
			p.status = StatusFailed
			return NewPluginError(p.id, "ExecuteUpgrade", "Upgrade and rollback failed", err)
		}

		return NewPluginError(p.id, "ExecuteUpgrade", "Upgrade failed, rolled back", err)
	}

	// Update version and restore active state
	p.version = targetVersion
	p.status = StatusActive

	// Emit event indicating upgrade has been completed
	p.EmitEvent(PluginEvent{
		Type:     EventUpgradeCompleted,
		Priority: PriorityHigh,
		Source:   "ExecuteUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"version": targetVersion,
		},
	})

	return nil
}

// RollbackUpgrade rolls back the plugin upgrade.
// This method checks if the plugin is in the upgrading or failed state.
func (p *TypedBasePlugin[T]) RollbackUpgrade(previousVersion string) error {
	if p.status != StatusUpgrading && p.status != StatusFailed {
		return NewPluginError(p.id, "RollbackUpgrade", "Plugin must be in upgrading or failed state", ErrPluginNotActive)
	}

	p.status = StatusRollback
	// Emit event indicating rollback has been initiated
	p.EmitEvent(PluginEvent{
		Type:     EventRollbackInitiated,
		Priority: PriorityHigh,
		Source:   "RollbackUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"previousVersion": previousVersion,
			"currentVersion":  p.version,
		},
	})

	// Perform rollback tasks
	if err := p.PerformRollback(previousVersion); err != nil {
		p.status = StatusFailed
		// Emit event indicating rollback failed
		p.EmitEvent(PluginEvent{
			Type:     EventRollbackFailed,
			Priority: PriorityCritical,
			Source:   "RollbackUpgrade",
			Category: "upgrade",
			Error:    err,
			Metadata: map[string]any{
				"previousVersion": previousVersion,
				"currentVersion":  p.version,
			},
		})
		return NewPluginError(p.id, "RollbackUpgrade", "Rollback failed", err)
	}

	// Restore version and active state
	p.version = previousVersion
	p.status = StatusActive

	// Emit event indicating rollback has been completed
	p.EmitEvent(PluginEvent{
		Type:     EventRollbackCompleted,
		Priority: PriorityHigh,
		Source:   "RollbackUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"version": previousVersion,
		},
	})

	return nil
}

// PerformUpgrade handles the actual upgrade process.
// This is an internal method called by ExecuteUpgrade.
func (p *TypedBasePlugin[T]) PerformUpgrade(targetVersion string) error {
	// Implementation-specific upgrade logic
	return nil
}

// PerformRollback handles the actual rollback process.
// This is an internal method called by RollbackUpgrade.
func (p *TypedBasePlugin[T]) PerformRollback(previousVersion string) error {
	// Implementation-specific rollback logic
	return nil
}

// GetCapabilities returns the plugin's upgrade capabilities.
func (p *TypedBasePlugin[T]) GetCapabilities() []UpgradeCapability {
	return p.capabilities
}

// SupportsCapability checks if the plugin supports the specified upgrade capability.
func (p *TypedBasePlugin[T]) SupportsCapability(cap UpgradeCapability) bool {
	for _, c := range p.capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// BasePlugin maintains backward compatibility for base plugins
type BasePlugin = TypedBasePlugin[any]

// NewBasePlugin creates a base plugin (backward compatibility)
func NewBasePlugin(id, name, description, version, confPrefix string, weight int) *BasePlugin {
	return NewTypedBasePlugin[any](id, name, description, version, confPrefix, weight, nil)
}
