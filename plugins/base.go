// Package plugins provides the core plugin system for the Lynx framework.
package plugins

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// TypedBasePlugin provides a generic base plugin with type-safe plugin foundation implementation.
// All reads and writes to status are protected by statusMu for concurrent safety (e.g. health checks
// and lifecycle operations from different goroutines).
type TypedBasePlugin[T any] struct {
	// Basic plugin metadata
	id          string // Unique identifier for the plugin
	name        string // Human-readable name
	description string // Detailed description of functionality
	confPrefix  string // Configuration prefix
	version     string // Semantic version number

	// Operational state (status is guarded by statusMu for concurrency safety)
	status   PluginStatus // Current plugin status
	statusMu sync.RWMutex // protects status
	runtime  Runtime      // Runtime environment reference
	logger   log.Logger   // Plugin-specific logger

	// Event handling
	eventFilters []EventFilter // List of active event filters

	// Configuration
	config map[string]any // Plugin-specific configuration

	// Plugin weight for prioritization, higher values load first
	weight int // Plugin weight for prioritization

	// Dependency management
	dependencies []Dependency        // List of plugin dependencies
	capabilities []UpgradeCapability // List of plugin upgrade capabilities

	// orphanedStages counts legacy lifecycle tasks that were abandoned after a
	// context cancellation but are still running in the background (they ignore
	// ctx and cannot be force-stopped). It is maintained with atomic ops and
	// exposed via OrphanedStageCount for observability and leak detection.
	orphanedStages int64

	// Type-safe instance
	instance T
}

// getStatus returns the current plugin status in a concurrency-safe way.
func (p *TypedBasePlugin[T]) getStatus() PluginStatus {
	p.statusMu.RLock()
	defer p.statusMu.RUnlock()
	return p.status
}

// setStatus sets the plugin status in a concurrency-safe way.
func (p *TypedBasePlugin[T]) setStatus(s PluginStatus) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	p.status = s
}

// StartContext starts the plugin with genuine context cancellation.
//
// Unlike a naive wrapper that spawns Start in a goroutine and returns early on
// ctx.Done() (leaking the goroutine and letting it flip the plugin to Active
// behind the caller's back), this implementation:
//
//   - runs the framework lifecycle inline and checks ctx at every phase boundary,
//     so a cancelled context aborts progression instead of racing it;
//   - prefers the plugin's ContextStartupTasker hook, passing ctx straight through
//     so the plugin's own work observes cancellation — this is real cancellation;
//   - for a legacy StartupTasker that cannot observe ctx, honours the deadline for
//     the caller via runLegacyStageWatched and abandons the work *safely* — the
//     abandoned goroutine never mutates plugin status, and the orphan is counted
//     and logged rather than silently leaked;
//   - re-checks ctx after the startup stage so a late-completing legacy task can
//     never promote the plugin to Active once cancellation has been observed.
func (p *TypedBasePlugin[T]) StartContext(ctx context.Context, plugin Plugin) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}
	if p.getStatus() == StatusActive {
		return ErrPluginAlreadyActive
	}
	if p.runtime == nil {
		return ErrPluginNotInitialized
	}

	p.setStatus(StatusInitializing)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "StartContext",
		Category: "lifecycle",
	})

	// Startup work: genuine cancellation when the plugin observes ctx; safe,
	// deadline-honouring abandonment otherwise.
	if err := p.runStartupStage(ctx, plugin); err != nil {
		p.setStatus(StatusFailed)
		if isLifecycleContextErr(err) {
			return fmt.Errorf("plugin %s start canceled: %w", plugin.Name(), err)
		}
		return NewPluginError(p.id, "Start", "Failed to perform startup tasks", err)
	}

	// Guard: even if a legacy task ignored ctx and returned after the deadline,
	// do not promote the plugin to Active once cancellation has fired.
	if err := ctx.Err(); err != nil {
		p.setStatus(StatusFailed)
		return fmt.Errorf("plugin %s start canceled after startup tasks: %w", plugin.Name(), err)
	}

	if err := RunHealthCheck(plugin); err != nil {
		p.setStatus(StatusFailed)
		log.Errorf("Plugin %s health check failed: %v", plugin.Name(), err)
		return fmt.Errorf("plugin %s health check failed: %w", plugin.Name(), err)
	}

	p.setStatus(StatusActive)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "StartContext",
		Category: "lifecycle",
	})
	return nil
}

// StopContext stops the plugin with genuine context cancellation.
// It mirrors StartContext: framework phases run inline with ctx checks, the
// ContextCleanupTasker hook is preferred for real cancellation, and a legacy
// CleanupTasker is abandoned safely (counted, never corrupting status) on timeout.
func (p *TypedBasePlugin[T]) StopContext(ctx context.Context, plugin Plugin) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before stop: %w", err)
	}
	if p.getStatus() != StatusActive {
		return NewPluginError(p.id, "Stop", "Plugin must be active to stop", ErrPluginNotActive)
	}

	p.setStatus(StatusStopping)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "StopContext",
		Category: "lifecycle",
	})

	if err := p.runCleanupStage(ctx, plugin); err != nil {
		p.setStatus(StatusFailed)
		if isLifecycleContextErr(err) {
			return fmt.Errorf("plugin %s stop canceled: %w", plugin.Name(), err)
		}
		return NewPluginError(p.id, "Stop", "Failed to perform cleanup tasks", err)
	}

	if err := ctx.Err(); err != nil {
		p.setStatus(StatusFailed)
		return fmt.Errorf("plugin %s stop canceled after cleanup tasks: %w", plugin.Name(), err)
	}

	p.setStatus(StatusTerminated)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopped,
		Priority: PriorityNormal,
		Source:   "StopContext",
		Category: "lifecycle",
	})
	return nil
}

// InitializeContext initializes the plugin with genuine context cancellation.
// It mirrors StartContext: the ContextResourceInitializer hook is preferred for
// real cancellation, and a legacy ResourceInitializer is abandoned safely on timeout.
func (p *TypedBasePlugin[T]) InitializeContext(ctx context.Context, plugin Plugin, rt Runtime) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before initialize: %w", err)
	}
	if rt == nil {
		return ErrPluginNotInitialized
	}

	p.runtime = rt
	p.logger = rt.GetLogger()
	p.setStatus(StatusInitializing)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitializing,
		Priority: PriorityNormal,
		Source:   "InitializeContext",
		Category: "lifecycle",
	})

	if err := p.runInitStage(ctx, plugin, rt); err != nil {
		p.setStatus(StatusFailed)
		if isLifecycleContextErr(err) {
			return fmt.Errorf("plugin %s initialize canceled: %w", plugin.Name(), err)
		}
		return NewPluginError(p.id, "Initialize", "Failed to initialize resources", err)
	}

	if err := ctx.Err(); err != nil {
		p.setStatus(StatusFailed)
		return fmt.Errorf("plugin %s initialize canceled after resource init: %w", plugin.Name(), err)
	}

	p.setStatus(StatusInactive)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitialized,
		Priority: PriorityNormal,
		Source:   "InitializeContext",
		Category: "lifecycle",
	})
	return nil
}

// runStartupStage runs the startup work with the strongest cancellation the
// plugin supports: the context-aware hook when present, otherwise a safely
// abandoned legacy task.
func (p *TypedBasePlugin[T]) runStartupStage(ctx context.Context, plugin Plugin) error {
	if handled, err := RunStartupTasksContext(ctx, plugin); handled {
		return err
	}
	return p.runLegacyStageWatched(ctx, "StartupTasks", plugin, func() error {
		return RunStartupTasks(plugin)
	})
}

// runCleanupStage is the cleanup counterpart of runStartupStage.
func (p *TypedBasePlugin[T]) runCleanupStage(ctx context.Context, plugin Plugin) error {
	if handled, err := RunCleanupTasksContext(ctx, plugin); handled {
		return err
	}
	return p.runLegacyStageWatched(ctx, "CleanupTasks", plugin, func() error {
		return RunCleanupTasks(plugin)
	})
}

// runInitStage is the resource-init counterpart of runStartupStage.
func (p *TypedBasePlugin[T]) runInitStage(ctx context.Context, plugin Plugin, rt Runtime) error {
	if handled, err := RunInitializeResourcesContext(ctx, plugin, rt); handled {
		return err
	}
	return p.runLegacyStageWatched(ctx, "InitializeResources", plugin, func() error {
		return InitializePluginResources(plugin, rt)
	})
}

// runLegacyStageWatched runs a lifecycle task that cannot observe ctx (it has no
// context-aware hook). When ctx is not cancellable it simply runs inline. When ctx
// can be cancelled, it runs the task in a watched goroutine so the deadline is
// honoured for the caller; on cancellation it abandons the goroutine *safely* —
// the leaked goroutine only runs the user task and never touches framework status,
// and the orphan is tracked (counted + logged) instead of vanishing silently.
// Go cannot kill a running goroutine, so this is the honest best a non-cooperating
// task allows; implement the Context* hooks for real cancellation.
func (p *TypedBasePlugin[T]) runLegacyStageWatched(ctx context.Context, stage string, plugin Plugin, fn func() error) error {
	if ctx.Done() == nil {
		// Non-cancellable context (e.g. context.Background): run inline, no goroutine.
		return runStageGuarded(stage, fn)
	}

	done := make(chan error, 1) // buffered so a late send never blocks the leaked goroutine
	go func() {
		done <- runStageGuarded(stage, fn)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		p.trackOrphanedStage(stage, plugin, done)
		return fmt.Errorf("plugin %s %s canceled by context (legacy task ignores cancellation; goroutine continues in background): %w", plugin.Name(), stage, ctx.Err())
	}
}

// runStageGuarded runs fn, converting a panic into an error so a misbehaving task
// cannot take down the lifecycle goroutine.
func runStageGuarded(stage string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in %s: %v", stage, r)
		}
	}()
	return fn()
}

// trackOrphanedStage records a legacy task that was abandoned after cancellation
// and is still running. It increments the orphan counter and starts a tiny watcher
// that decrements it (and logs) once the task finally returns.
func (p *TypedBasePlugin[T]) trackOrphanedStage(stage string, plugin Plugin, done <-chan error) {
	atomic.AddInt64(&p.orphanedStages, 1)
	log.Warnf("plugin %s %s abandoned after context cancellation; goroutine leaked until the task returns. Implement the context-aware lifecycle hooks (e.g. %sContext) to make this cancellable.",
		plugin.Name(), stage, stage)
	go func() {
		err := <-done // buffered channel guarantees this eventually receives
		atomic.AddInt64(&p.orphanedStages, -1)
		if err != nil {
			log.Warnf("plugin %s orphaned %s finally returned (late) with error: %v", plugin.Name(), stage, err)
		} else {
			log.Infof("plugin %s orphaned %s finally completed (late)", plugin.Name(), stage)
		}
	}()
}

// OrphanedStageCount returns the number of legacy lifecycle tasks that were
// abandoned after a context cancellation and are still running in the background.
// A non-zero value means a plugin's task is ignoring cancellation; it should
// drop back to zero once those tasks finish.
func (p *TypedBasePlugin[T]) OrphanedStageCount() int64 {
	return atomic.LoadInt64(&p.orphanedStages)
}

// isLifecycleContextErr reports whether err was caused by context cancellation or
// a deadline, so callers can distinguish cancellation from genuine task failure.
func isLifecycleContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// IsContextAware returns false by default for base plugin
// Subclasses should override this if they truly respect context cancellation
func (p *TypedBasePlugin[T]) IsContextAware() bool {
	return false // Base implementation is not truly context-aware
}

// PluginProtocol declares the default explicit lifecycle protocol for base plugins.
// Concrete plugins may override this method to opt into stronger capabilities.
// The framework default stays conservative: plugin orchestration and stability
// are core concerns, while runtime hot-reload/rollback capabilities must be
// explicitly declared by concrete plugins rather than inherited implicitly.
func (p *TypedBasePlugin[T]) PluginProtocol() PluginProtocol {
	return PluginProtocol{
		ManagedLifecycle: true,
		HealthAware:      true,
		ContextLifecycle: false,
		Recoverable:      false,
	}
}

// NewTypedBasePlugin creates a new TypedBasePlugin with the provided metadata.
// Plugins should embed this and call it from their own constructor.
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

// GetTypedInstance returns the concrete plugin instance stored in this base.
func (p *TypedBasePlugin[T]) GetTypedInstance() T {
	return p.instance
}

// Initialize stores the runtime, drives the InitializeResources hook, and transitions
// the plugin from Inactive through Initializing back to Inactive on success.
func (p *TypedBasePlugin[T]) Initialize(plugin Plugin, rt Runtime) error {
	if rt == nil {
		return ErrPluginNotInitialized
	}

	p.runtime = rt
	p.logger = rt.GetLogger()
	p.setStatus(StatusInitializing)

	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitializing,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	if err := InitializePluginResources(plugin, rt); err != nil {
		p.setStatus(StatusFailed)
		return NewPluginError(p.id, "Initialize", "Failed to initialize resources", err)
	}

	p.setStatus(StatusInactive)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitialized,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	return nil
}

// Start runs StartupTasks and CheckHealth, then transitions the plugin to Active.
func (p *TypedBasePlugin[T]) Start(plugin Plugin) error {
	if p.getStatus() == StatusActive {
		return ErrPluginAlreadyActive
	}

	if p.runtime == nil {
		return ErrPluginNotInitialized
	}

	p.setStatus(StatusInitializing)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	if err := RunStartupTasks(plugin); err != nil {
		p.setStatus(StatusFailed)
		return NewPluginError(p.id, "Start", "Failed to perform startup tasks", err)
	}

	err := RunHealthCheck(plugin)
	if err != nil {
		p.setStatus(StatusFailed)
		log.Errorf("Plugin %s health check failed: %v", plugin.Name(), err)
		return fmt.Errorf("plugin %s health check failed: %w", plugin.Name(), err)
	}

	p.setStatus(StatusActive)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	return nil
}

// Stop runs CleanupTasks and transitions the plugin to Terminated.
func (p *TypedBasePlugin[T]) Stop(plugin Plugin) error {
	if p.getStatus() != StatusActive {
		return NewPluginError(p.id, "Stop", "Plugin must be active to stop", ErrPluginNotActive)
	}

	p.setStatus(StatusStopping)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	if err := RunCleanupTasks(plugin); err != nil {
		p.setStatus(StatusFailed)
		return NewPluginError(p.id, "Stop", "Failed to perform cleanup tasks", err)
	}

	p.setStatus(StatusTerminated)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopped,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	return nil
}

// Status returns the plugin's current lifecycle state.
func (p *TypedBasePlugin[T]) Status(plugin Plugin) PluginStatus {
	return p.getStatus()
}

// InitializeResources is a no-op default; override in the embedding struct to set up resources.
func (p *TypedBasePlugin[T]) InitializeResources(rt Runtime) error {
	return nil
}

// StartupTasks is a no-op default; override in the embedding struct to run startup work.
func (p *TypedBasePlugin[T]) StartupTasks() error {
	return nil
}

// CleanupTasks is a no-op default; override in the embedding struct to run teardown work.
func (p *TypedBasePlugin[T]) CleanupTasks() error {
	return nil
}

// ID returns the unique identifier of the plugin.
func (p *TypedBasePlugin[T]) ID() string {
	return p.id
}

// Name returns the human-readable plugin name used for lookup and logging.
func (p *TypedBasePlugin[T]) Name() string {
	return p.name
}

// Weight returns the plugin weight for prioritization
func (p *TypedBasePlugin[T]) Weight() int {
	return p.weight
}

// Description returns a detailed description of the plugin's functionality.
func (p *TypedBasePlugin[T]) Description() string {
	return p.description
}

// Version returns the semantic version of the plugin.
// Version format should follow semver conventions (MAJOR.MINOR.PATCH).
func (p *TypedBasePlugin[T]) Version() string {
	p.statusMu.RLock()
	defer p.statusMu.RUnlock()
	return p.version
}

// SetStatus updates the plugin's lifecycle state.
func (p *TypedBasePlugin[T]) SetStatus(status PluginStatus) {
	p.setStatus(status)
}

// GetHealth runs CheckHealth (when active) and returns a structured health report.
func (p *TypedBasePlugin[T]) GetHealth() HealthReport {
	report := HealthReport{
		Status:    "unknown",
		Details:   make(map[string]any),
		Timestamp: time.Now().Unix(),
	}

	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckStarted,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Non-active states report directly without running the health check.
	switch p.getStatus() {
	case StatusTerminated, StatusFailed:
		report.Status = "unhealthy"
		report.Message = "Plugin is not operational"
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
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusInitializing:
		report.Status = "initializing"
		report.Message = "Plugin is being initialized"
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

	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckRunning,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	if err := p.CheckHealth(); err != nil {
		report.Status = "unhealthy"
		report.Message = err.Error()
		p.EmitEvent(PluginEvent{
			Type:     EventHealthCheckFailed,
			Priority: PriorityHigh,
			Source:   "GetHealth",
			Category: "health",
			Error:    err,
		})
		return report
	}

	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckDone,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	if report.Status == "healthy" {
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusOK,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
	}

	return report
}

// GetDependencies returns a copy of the plugin dependencies so callers cannot
// mutate the slice and to avoid races with concurrent AddDependency.
// For correct load order: the framework calls this before initializing plugins
// (during TopologicalSort). Required dependencies that affect load order should
// be added in the plugin constructor so they are available here.
func (p *TypedBasePlugin[T]) GetDependencies() []Dependency {
	if len(p.dependencies) == 0 {
		return nil
	}
	out := make([]Dependency, len(p.dependencies))
	copy(out, p.dependencies)
	return out
}

// AddDependency adds a new dependency to the plugin.
// The dependency will be validated during plugin initialization.
// For load-order resolution, add required dependencies in the plugin constructor
// so GetDependencies() is complete before the manager runs topological sort.
func (p *TypedBasePlugin[T]) AddDependency(dep Dependency) {
	p.dependencies = append(p.dependencies, dep)
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

// AddEventFilter appends an event filter; events that match no filter are suppressed.
func (p *TypedBasePlugin[T]) AddEventFilter(filter EventFilter) {
	p.eventFilters = append(p.eventFilters, filter)
}

// RemoveEventFilter removes the filter at the given index.
func (p *TypedBasePlugin[T]) RemoveEventFilter(index int) {
	if index >= 0 && index < len(p.eventFilters) {
		p.eventFilters = append(p.eventFilters[:index], p.eventFilters[index+1:]...)
	}
}

// HandleEvent routes an event to the appropriate typed handler after filter check.
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
func (p *TypedBasePlugin[T]) EmitEvent(event PluginEvent) {
	p.EmitEventInternal(event)
}

// EmitEventInternal stamps the event with plugin ID, status, and timestamp, then forwards
// it to the runtime bus. Dropped silently when the runtime has not been set yet.
func (p *TypedBasePlugin[T]) EmitEventInternal(event PluginEvent) {
	if p.runtime == nil {
		return
	}
	event.PluginID = p.id
	event.Status = p.getStatus()
	event.Timestamp = time.Now().Unix()

	if p.ShouldEmitEvent(event) {
		p.runtime.EmitEvent(event)
	}
}

// ShouldEmitEvent returns true if the event passes all configured filters,
// or when no filters are configured.
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

// ShouldHandleEvent delegates to ShouldEmitEvent; separate entry point for incoming event routing.
func (p *TypedBasePlugin[T]) ShouldHandleEvent(event PluginEvent) bool {
	return p.ShouldEmitEvent(event)
}

// EventMatchesFilter reports whether an event satisfies all non-empty criteria in filter.
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

	if len(filter.Priorities) > 0 {
		priorityMatch := false
		for _, pri := range filter.Priorities {
			if event.Priority == pri {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

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

	if filter.FromTime > 0 && event.Timestamp < filter.FromTime {
		return false
	}
	if filter.ToTime > 0 && event.Timestamp > filter.ToTime {
		return false
	}

	return true
}

// CheckHealth is the no-op default; override to report plugin-specific health.
func (p *TypedBasePlugin[T]) CheckHealth() error {
	return nil
}

// ValidateConfig is the no-op default; override to validate config before applying it.
func (p *TypedBasePlugin[T]) ValidateConfig(conf any) error {
	return nil
}

// ApplyConfig is the no-op default; override to apply validated config.
func (p *TypedBasePlugin[T]) ApplyConfig(conf any) error {
	return nil
}

// HandleHealthEvent processes health-related events. Override to add behavior.
func (p *TypedBasePlugin[T]) HandleHealthEvent(event PluginEvent) {
}

// HandleConfigEvent processes configuration-related events. Override to add behavior.
func (p *TypedBasePlugin[T]) HandleConfigEvent(event PluginEvent) {
}

// HandleDependencyEvent processes dependency-related events. Override to add behavior.
func (p *TypedBasePlugin[T]) HandleDependencyEvent(event PluginEvent) {
}

// HandleDefaultEvent processes events without a specific handler. Override to add behavior.
func (p *TypedBasePlugin[T]) HandleDefaultEvent(event PluginEvent) {
}

// Suspend pauses the plugin without releasing its resources, moving it to Suspended.
func (p *TypedBasePlugin[T]) Suspend() error {
	if p.getStatus() != StatusActive {
		return NewPluginError(p.id, "Suspend", "Plugin must be active to suspend", ErrPluginNotActive)
	}

	p.setStatus(StatusStopping)
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Suspend",
		Category: "lifecycle",
	})

	p.setStatus(StatusSuspended)
	return nil
}

// Resume transitions the plugin from Suspended back to Active.
func (p *TypedBasePlugin[T]) Resume() error {
	if p.getStatus() != StatusSuspended {
		return NewPluginError(p.id, "Resume", "Plugin must be suspended to resume", ErrPluginNotActive)
	}

	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	p.setStatus(StatusActive)

	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	return nil
}

// BasePlugin maintains backward compatibility for base plugins
type BasePlugin = TypedBasePlugin[any]

// NewBasePlugin creates a base plugin (backward compatibility)
func NewBasePlugin(id, name, description, version, confPrefix string, weight int) *BasePlugin {
	return NewTypedBasePlugin[any](id, name, description, version, confPrefix, weight, nil)
}
