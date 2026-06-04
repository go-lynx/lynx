// Package plugins provides the core plugin system for the Lynx framework.
//
// Runtime is the main environment interface for plugins; the default implementation
// is UnifiedRuntime (see unified_runtime.go). Runtime composes ResourceManager,
// ConfigProvider, LogProvider, EventEmitter, and plugin-context helpers.
package plugins

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// PluginStatus represents the current operational status of a plugin in the system.
// It tracks the plugin's lifecycle state from initialization through termination.
type PluginStatus int

const (
	// StatusInactive indicates that the plugin is loaded but not yet initialized
	// This is the initial state of a plugin when it is first loaded into the system
	StatusInactive PluginStatus = iota

	// StatusInitializing indicates that the plugin is currently performing initialization
	// During this state, the plugin is setting up resources, establishing connections,
	// and preparing for normal operation
	StatusInitializing

	// StatusActive indicates that the plugin is fully operational and running normally
	// In this state, the plugin is processing requests and performing its intended functions
	StatusActive

	// StatusSuspended indicates that the plugin is temporarily paused
	// The plugin retains its resources but is not processing new requests
	// Can be resumed to StatusActive without full reinitialization
	StatusSuspended

	// StatusStopping indicates that the plugin is in the process of shutting down
	// During this state, the plugin is cleaning up resources and finishing pending operations
	StatusStopping

	// StatusTerminated indicates that the plugin has been gracefully shut down
	// All resources have been released and connections closed
	// Requires full reinitialization to become active again
	StatusTerminated

	// StatusFailed indicates that the plugin has encountered a fatal error
	// The plugin is non-operational and may require manual intervention
	// Should transition to StatusTerminated or attempt recovery
	StatusFailed
)

// CorePlugin is the minimal lifecycle-managed plugin contract.
// It intentionally excludes step-level hooks so frameworks can reason about
// plugin identity/lifecycle separately from optional operational capabilities.
type CorePlugin interface {
	Metadata
	Lifecycle
	DependencyAware
}

// Plugin is the current managed plugin contract used by Lynx today.
// New code should prefer reasoning in terms of CorePlugin plus specific capability
// interfaces below, rather than assuming every plugin must implement one wide interface.
type Plugin interface {
	CorePlugin
	LifecycleSteps
}

// LifecycleWithContext defines optional context-aware lifecycle methods.
// Plugins implementing this interface can receive cancellation/timeout signals
// and are encouraged to stop work promptly when the context is done.
// This interface is optional; if not implemented,
// the manager will fall back to calling the non-context methods.
type LifecycleWithContext interface {
	// InitializeContext prepares the plugin with context support.
	InitializeContext(ctx context.Context, plugin Plugin, rt Runtime) error

	// StartContext starts the plugin with context support.
	StartContext(ctx context.Context, plugin Plugin) error

	// StopContext stops the plugin with context support.
	StopContext(ctx context.Context, plugin Plugin) error
}

// ContextAwareness defines an optional marker for real context awareness.
// Some plugins may satisfy LifecycleWithContext via embedded base types but
// still ignore ctx. Implement this interface on the concrete plugin type and
// return true only when lifecycle methods actually observe ctx cancellation.
type ContextAwareness interface {
	// IsContextAware returns true if the plugin genuinely honors context
	// cancellation/timeout within Initialize/Start/Stop.
	IsContextAware() bool
}

// TypedPlugin generic plugin interface, T is the specific plugin type
// Provides type-safe plugin access capabilities
type TypedPlugin[T any] interface {
	Plugin
	GetTypedInstance() T
}

// Metadata exposes a plugin's identity and descriptive attributes.
type Metadata interface {
	// ID returns the plugin's unique identifier, which must be unique across all plugins.
	ID() string

	// Name returns the human-readable display name.
	Name() string

	// Description returns a description of the plugin's purpose and functionality.
	Description() string

	// Version returns the semantic version (MAJOR.MINOR.PATCH).
	Version() string

	// Weight returns the load-ordering weight; higher values load first.
	Weight() int
}

// Lifecycle defines the core lifecycle transitions the framework drives on a plugin.
// The framework calls these in order: Initialize, then Start; Stop on shutdown.
type Lifecycle interface {
	// Initialize prepares the plugin (resources, connections, internal state).
	// Must be called before Start.
	Initialize(plugin Plugin, rt Runtime) error

	// Start begins the plugin's main functionality. Call only after Initialize succeeds.
	Start(plugin Plugin) error

	// Stop gracefully terminates the plugin, releasing resources and connections.
	Stop(plugin Plugin) error

	// Status returns the plugin's current lifecycle state.
	Status(plugin Plugin) PluginStatus
}

// LifecycleSteps are the plugin-supplied hooks the base lifecycle invokes at each
// phase: InitializeResources during Initialize, StartupTasks and CheckHealth during
// Start, CleanupTasks during Stop. Embedding TypedBasePlugin provides no-op defaults
// to override as needed.
type LifecycleSteps interface {
	InitializeResources(rt Runtime) error
	StartupTasks() error
	CleanupTasks() error
	CheckHealth() error
}

// ResourceInitializer indicates the plugin has an initialization hook for runtime-bound resources.
type ResourceInitializer interface {
	InitializeResources(rt Runtime) error
}

// StartupTasker indicates the plugin exposes startup work beyond basic activation.
type StartupTasker interface {
	StartupTasks() error
}

// CleanupTasker indicates the plugin exposes shutdown cleanup work.
type CleanupTasker interface {
	CleanupTasks() error
}

// HealthChecker indicates the plugin can actively verify its own health.
type HealthChecker interface {
	CheckHealth() error
}

// PluginProtocol declares lifecycle-related capabilities explicitly.
// New plugins should prefer declaring protocol instead of relying on legacy runtime probing.
//
// Configuration-mutation flags (ConfigHotReload, ConfigValidation, ConfigRollback) have
// been removed: Lynx core applies configuration changes by process restart, so advertising
// in-process config-reload capability is misleading.  Plugins that previously set those
// flags should use Configurable / ConfigValidator / ConfigRollbacker interfaces directly,
// and callers should rely on GetRestartRequirementReport() to discover which plugins
// need a restart after configuration changes.
type PluginProtocol struct {
	ManagedLifecycle bool
	HealthAware      bool
	ContextLifecycle bool
	Recoverable      bool
}

// ProtocolAwarePlugin explicitly declares its lifecycle protocol.
type ProtocolAwarePlugin interface {
	PluginProtocol() PluginProtocol
}

// PluginCapabilities describes which optional capabilities a plugin exposes.
type PluginCapabilities struct {
	HasMetadata         bool
	HasLifecycle        bool
	HasDependencies     bool
	HasResourceInit     bool
	HasStartupTasks     bool
	HasCleanupTasks     bool
	HasHealthCheck      bool
	HasLifecycleWithCtx bool
	IsTrulyContextAware bool
	// HasContextSteps is true when the plugin implements at least one context-aware
	// lifecycle step hook (ContextResourceInitializer / ContextStartupTasker /
	// ContextCleanupTasker), meaning that phase's work genuinely observes ctx.
	HasContextSteps  bool
	IsManagedPlugin  bool
	Protocol         PluginProtocol
	ProtocolExplicit bool
}

// DescribePluginCapabilities returns a structured capability report for a plugin-like object.
func DescribePluginCapabilities(plugin any) PluginCapabilities {
	_, hasMetadata := plugin.(Metadata)
	_, hasLifecycle := plugin.(Lifecycle)
	_, hasDependencies := plugin.(DependencyAware)
	_, hasResourceInit := plugin.(ResourceInitializer)
	_, hasStartupTasks := plugin.(StartupTasker)
	_, hasCleanupTasks := plugin.(CleanupTasker)
	_, hasHealthCheck := plugin.(HealthChecker)
	_, hasLifecycleWithCtx := plugin.(LifecycleWithContext)
	isContextAware := false
	if ca, ok := plugin.(ContextAwareness); ok {
		isContextAware = ca.IsContextAware()
	}
	hasContextSteps := SupportsContextSteps(plugin)
	_, isManaged := plugin.(Plugin)
	protocol := PluginProtocol{}
	explicit := false
	if declared, ok := plugin.(ProtocolAwarePlugin); ok {
		protocol = declared.PluginProtocol()
		explicit = true
	}
	return PluginCapabilities{
		HasMetadata:         hasMetadata,
		HasLifecycle:        hasLifecycle,
		HasDependencies:     hasDependencies,
		HasResourceInit:     hasResourceInit,
		HasStartupTasks:     hasStartupTasks,
		HasCleanupTasks:     hasCleanupTasks,
		HasHealthCheck:      hasHealthCheck,
		HasLifecycleWithCtx: hasLifecycleWithCtx,
		IsTrulyContextAware: isContextAware,
		HasContextSteps:     hasContextSteps,
		IsManagedPlugin:     isManaged,
		Protocol:            protocol,
		ProtocolExplicit:    explicit,
	}
}

// HasTrueContextLifecycle reports whether a plugin's lifecycle is genuinely
// cancellable through its context-aware entrypoints. It is true when the plugin
// exposes the LifecycleWithContext entrypoints AND either:
//
//   - implements at least one context-aware step hook (the preferred path: the
//     plugin's own work observes ctx, so cancellation is real), or
//   - uses the legacy explicit opt-in: declares PluginProtocol().ContextLifecycle
//     and asserts ContextAwareness.IsContextAware()=true.
//
// The framework routes such plugins through StartContext/StopContext/InitializeContext.
func HasTrueContextLifecycle(plugin any) bool {
	caps := DescribePluginCapabilities(plugin)
	if !caps.HasLifecycleWithCtx {
		return false
	}
	if caps.HasContextSteps {
		return true
	}
	return caps.Protocol.ContextLifecycle && caps.IsTrulyContextAware
}

// GetTrueContextLifecycle returns the plugin's LifecycleWithContext only when it is truly
// context-aware according to HasTrueContextLifecycle.
func GetTrueContextLifecycle(plugin any) (LifecycleWithContext, bool) {
	lc, ok := plugin.(LifecycleWithContext)
	if !ok {
		return nil, false
	}
	if !HasTrueContextLifecycle(plugin) {
		return nil, false
	}
	return lc, true
}

// InitializePluginResources executes the resource initialization hook when present.
func InitializePluginResources(plugin any, rt Runtime) error {
	if initializer, ok := plugin.(ResourceInitializer); ok {
		return initializer.InitializeResources(rt)
	}
	return nil
}

// RunStartupTasks executes startup tasks when present.
func RunStartupTasks(plugin any) error {
	if startup, ok := plugin.(StartupTasker); ok {
		return startup.StartupTasks()
	}
	return nil
}

// RunCleanupTasks executes cleanup tasks when present.
func RunCleanupTasks(plugin any) error {
	if cleanup, ok := plugin.(CleanupTasker); ok {
		return cleanup.CleanupTasks()
	}
	return nil
}

// RunHealthCheck executes the plugin health check when present.
func RunHealthCheck(plugin any) error {
	if checker, ok := plugin.(HealthChecker); ok {
		return checker.CheckHealth()
	}
	return nil
}

// Context-aware lifecycle step hooks.
//
// These are the path to *genuine* cancellation. When a plugin implements one of
// these interfaces, the framework passes the lifecycle context straight through,
// so the plugin's own work can observe ctx.Done() and stop promptly. A plugin
// that only implements the non-context steps (ResourceInitializer / StartupTasker
// / CleanupTasker) cannot be force-stopped — Go has no way to kill a running
// goroutine — so the framework can only abandon it on timeout, never truly cancel
// the work. Implement these to make cancellation real.

// ContextResourceInitializer is the context-aware form of ResourceInitializer.
type ContextResourceInitializer interface {
	InitializeResourcesContext(ctx context.Context, rt Runtime) error
}

// ContextStartupTasker is the context-aware form of StartupTasker.
type ContextStartupTasker interface {
	StartupTasksContext(ctx context.Context) error
}

// ContextCleanupTasker is the context-aware form of CleanupTasker.
type ContextCleanupTasker interface {
	CleanupTasksContext(ctx context.Context) error
}

// RunInitializeResourcesContext runs the context-aware resource init hook when present.
// The returned bool reports whether a context-aware hook handled the call; when false,
// the caller should fall back to the non-context ResourceInitializer.
func RunInitializeResourcesContext(ctx context.Context, plugin any, rt Runtime) (bool, error) {
	if init, ok := plugin.(ContextResourceInitializer); ok {
		return true, init.InitializeResourcesContext(ctx, rt)
	}
	return false, nil
}

// RunStartupTasksContext runs the context-aware startup hook when present.
// The returned bool reports whether a context-aware hook handled the call.
func RunStartupTasksContext(ctx context.Context, plugin any) (bool, error) {
	if startup, ok := plugin.(ContextStartupTasker); ok {
		return true, startup.StartupTasksContext(ctx)
	}
	return false, nil
}

// RunCleanupTasksContext runs the context-aware cleanup hook when present.
// The returned bool reports whether a context-aware hook handled the call.
func RunCleanupTasksContext(ctx context.Context, plugin any) (bool, error) {
	if cleanup, ok := plugin.(ContextCleanupTasker); ok {
		return true, cleanup.CleanupTasksContext(ctx)
	}
	return false, nil
}

// SupportsContextSteps reports whether a plugin implements any context-aware
// lifecycle step hook, meaning at least one phase of its lifecycle can be
// genuinely cancelled rather than merely abandoned on timeout.
func SupportsContextSteps(plugin any) bool {
	if _, ok := plugin.(ContextResourceInitializer); ok {
		return true
	}
	if _, ok := plugin.(ContextStartupTasker); ok {
		return true
	}
	if _, ok := plugin.(ContextCleanupTasker); ok {
		return true
	}
	return false
}

// ResourceInfo resource information
type ResourceInfo struct {
	Name          string
	Type          string
	PluginID      string
	OwnerHandleID uint64
	IsPrivate     bool
	CreatedAt     time.Time
	LastUsedAt    time.Time
	AccessCount   int64
	Size          int64 // Resource size (bytes)
	Metadata      map[string]any
}

// ResourceManager manages named resources shared across plugins, plus their
// lifecycle metadata.
type ResourceManager interface {
	// GetResource retrieves a shared plugin resource by name.
	GetResource(name string) (any, error)

	// RegisterResource registers a resource to be shared with other plugins.
	RegisterResource(name string, resource any) error

	GetResourceInfo(name string) (*ResourceInfo, error)
	ListResources() []*ResourceInfo
	CleanupResources(pluginID string) error
	GetResourceStats() map[string]any
}

// TypedResourceManager generic resource manager interface
type TypedResourceManager interface {
	ResourceManager
}

// ResourceHandle provides a stable typed view over a named runtime resource.
// It does not hold the concrete object; each call resolves the current resource,
// so it remains valid across runtime replacements.
type ResourceHandle[T any] struct {
	manager ResourceManager
	name    string
}

// GetTypedResource get type-safe resource (standalone function)
func GetTypedResource[T any](manager ResourceManager, name string) (T, error) {
	var zero T
	resource, err := manager.GetResource(name)
	if err != nil {
		return zero, err
	}

	typed, ok := resource.(T)
	if !ok {
		return zero, NewPluginError("runtime", "GetTypedResource", "Type assertion failed", nil)
	}

	return typed, nil
}

// RegisterTypedResource register type-safe resource (standalone function)
func RegisterTypedResource[T any](manager ResourceManager, name string, resource T) error {
	return manager.RegisterResource(name, resource)
}

// NewResourceHandle creates a stable typed handle for a named resource.
func NewResourceHandle[T any](manager ResourceManager, name string) ResourceHandle[T] {
	return ResourceHandle[T]{manager: manager, name: name}
}

// Name returns the bound resource name.
func (h ResourceHandle[T]) Name() string {
	return h.name
}

// Get resolves the current typed resource.
func (h ResourceHandle[T]) Get() (T, error) {
	return GetTypedResource[T](h.manager, h.name)
}

// Info returns the current resource metadata when the manager supports it.
func (h ResourceHandle[T]) Info() (*ResourceInfo, error) {
	if h.manager == nil {
		return nil, NewPluginError("runtime", "ResourceHandle.Info", "resource manager is nil", nil)
	}
	return h.manager.GetResourceInfo(h.name)
}

// ConfigProvider provides access to plugin configuration.
type ConfigProvider interface {
	// GetConfig returns the plugin configuration manager.
	GetConfig() config.Config
}

// LogProvider provides access to logging functionality.
type LogProvider interface {
	// GetLogger returns the plugin logger instance.
	GetLogger() log.Logger
}

// Suspendable allows temporarily pausing and resuming a plugin without
// reinitializing it; suspended plugins retain their state and resources.
type Suspendable interface {
	// Suspend pauses plugin operations while preserving state.
	Suspend() error

	// Resume restores operation from a suspended state without reinitialization.
	Resume() error
}

// DependencyAware lets a plugin declare the dependencies that drive load order.
// The framework calls GetDependencies before initialization to build the graph.
type DependencyAware interface {
	// GetDependencies returns the plugin's required and optional dependencies.
	GetDependencies() []Dependency
}

// EventHandler processes plugin lifecycle events delivered by the event system.
type EventHandler interface {
	// HandleEvent processes a single plugin event.
	HandleEvent(event PluginEvent)
}

// ServicePlugin service plugin constraint interface
type ServicePlugin[T any] interface {
	Plugin
	GetServer() T
	GetServerType() string
}

// DatabasePlugin database plugin constraint interface
type DatabasePlugin[T any] interface {
	Plugin
	GetDriver() T
	GetStats() any
	IsConnected() bool
	CheckHealth() error
}

// CachePlugin cache plugin constraint interface
type CachePlugin[T any] interface {
	Plugin
	GetClient() T
	GetConnectionStats() map[string]any
}

// MessagingPlugin messaging plugin constraint interface
type MessagingPlugin[T any] interface {
	Plugin
	GetProducer() T
	GetConsumer() T
}

// ServiceDiscoveryPlugin service discovery plugin constraint interface
type ServiceDiscoveryPlugin[T any] interface {
	Plugin
	GetRegistry() T
	GetDiscovery() T
}

// ========== Backward Compatible Interfaces ==========

// ServicePluginAny backward compatible service plugin interface
type ServicePluginAny interface {
	Plugin
	GetServer() any
	GetServerType() string
}

// DatabasePluginAny backward compatible database plugin interface
type DatabasePluginAny interface {
	Plugin
	GetDriver() any
	GetStats() any
	IsConnected() bool
	CheckHealth() error
}

// CachePluginAny backward compatible cache plugin interface
type CachePluginAny interface {
	Plugin
	GetClient() any
	GetConnectionStats() map[string]any
}

// MessagingPluginAny backward compatible messaging plugin interface
type MessagingPluginAny interface {
	Plugin
	GetProducer() any
	GetConsumer() any
}

// ServiceDiscoveryPluginAny backward compatible service discovery plugin interface
type ServiceDiscoveryPluginAny interface {
	Plugin
	GetRegistry() any
	GetDiscovery() any
}

// Runtime is the main interface for plugin runtime environment (resources, config, log, events, context).
// Implementations may compose smaller interfaces (ResourceManager, EventEmitter, etc.) for clarity.
type Runtime interface {
	TypedResourceManager
	ConfigProvider
	LogProvider
	EventEmitter
	// New: Logically separated resource management
	GetPrivateResource(name string) (any, error)
	RegisterPrivateResource(name string, resource any) error
	GetSharedResource(name string) (any, error)
	RegisterSharedResource(name string, resource any) error
	// New: Improved event system
	EmitPluginEvent(pluginName string, eventType string, data map[string]any)
	AddPluginListener(pluginName string, listener EventListener, filter *EventFilter)
	GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent
	// New: Event system configuration and metrics
	SetEventDispatchMode(mode string) error
	SetEventWorkerPoolSize(size int)
	SetEventTimeout(timeout time.Duration)
	GetEventStats() map[string]any
	// New: Plugin context management
	WithPluginContext(pluginName string) Runtime
	GetCurrentPluginContext() string
	// New: Configuration management
	SetConfig(conf config.Config)
	// Shutdown gracefully shuts down the runtime (cancels shutdown context, closes event adapter, etc.).
	// Safe to call multiple times. Should be called when the application is closing.
	Shutdown()
}

// TypedRuntime generic runtime interface
type TypedRuntime interface {
	Runtime
}

// TypedRuntimeImpl generic runtime implementation
type TypedRuntimeImpl struct {
	runtime Runtime
}

// NewTypedRuntime create generic runtime environment
func NewTypedRuntime() *TypedRuntimeImpl {
	return &TypedRuntimeImpl{
		runtime: NewUnifiedRuntime(),
	}
}

// NewSimpleRuntime returns the default Runtime implementation (UnifiedRuntime).
// Kept for backward compatibility; prefer NewUnifiedRuntime() for new code.
func NewSimpleRuntime() Runtime {
	return NewUnifiedRuntime()
}

// AddListener add event listener
func (r *TypedRuntimeImpl) AddListener(listener EventListener, filter *EventFilter) {
	r.runtime.AddListener(listener, filter)
}

// RemoveListener remove event listener
func (r *TypedRuntimeImpl) RemoveListener(listener EventListener) {
	r.runtime.RemoveListener(listener)
}

// GetEventHistory get event history
func (r *TypedRuntimeImpl) GetEventHistory(filter EventFilter) []PluginEvent {
	return r.runtime.GetEventHistory(filter)
}

// GetPrivateResource get private resource
func (r *TypedRuntimeImpl) GetPrivateResource(name string) (any, error) {
	return r.runtime.GetPrivateResource(name)
}

// RegisterPrivateResource register private resource
func (r *TypedRuntimeImpl) RegisterPrivateResource(name string, resource any) error {
	return r.runtime.RegisterPrivateResource(name, resource)
}

// GetSharedResource get shared resource
func (r *TypedRuntimeImpl) GetSharedResource(name string) (any, error) {
	return r.runtime.GetSharedResource(name)
}

// RegisterSharedResource register shared resource
func (r *TypedRuntimeImpl) RegisterSharedResource(name string, resource any) error {
	return r.runtime.RegisterSharedResource(name, resource)
}

// GetResource get resource (compatible with old interface)
func (r *TypedRuntimeImpl) GetResource(name string) (any, error) {
	return r.runtime.GetResource(name)
}

// RegisterResource register resource (compatible with old interface)
func (r *TypedRuntimeImpl) RegisterResource(name string, resource any) error {
	return r.runtime.RegisterResource(name, resource)
}

// EmitPluginEvent emit plugin namespace event
func (r *TypedRuntimeImpl) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	r.runtime.EmitPluginEvent(pluginName, eventType, data)
}

// WithPluginContext create runtime with plugin context
func (r *TypedRuntimeImpl) WithPluginContext(pluginName string) Runtime {
	return r.runtime.WithPluginContext(pluginName)
}

// GetCurrentPluginContext get current plugin context
func (r *TypedRuntimeImpl) GetCurrentPluginContext() string {
	return r.runtime.GetCurrentPluginContext()
}

// GetResourceInfo get resource info
func (r *TypedRuntimeImpl) GetResourceInfo(name string) (*ResourceInfo, error) {
	return r.runtime.GetResourceInfo(name)
}

// ListResources list all resources
func (r *TypedRuntimeImpl) ListResources() []*ResourceInfo {
	return r.runtime.ListResources()
}

// CleanupResources clean up resources for a specific plugin
func (r *TypedRuntimeImpl) CleanupResources(pluginID string) error {
	return r.runtime.CleanupResources(pluginID)
}

// GetResourceStats get resource statistics
func (r *TypedRuntimeImpl) GetResourceStats() map[string]any {
	return r.runtime.GetResourceStats()
}

// GetConfig get configuration
func (r *TypedRuntimeImpl) GetConfig() config.Config {
	return r.runtime.GetConfig()
}

// SetConfig set configuration
func (r *TypedRuntimeImpl) SetConfig(conf config.Config) {
	r.runtime.SetConfig(conf)
}

// GetLogger get logger
func (r *TypedRuntimeImpl) GetLogger() log.Logger {
	return r.runtime.GetLogger()
}

// EmitEvent emit event
func (r *TypedRuntimeImpl) EmitEvent(event PluginEvent) {
	r.runtime.EmitEvent(event)
}

// AddPluginListener add specific plugin event listener
func (r *TypedRuntimeImpl) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	r.runtime.AddPluginListener(pluginName, listener, filter)
}

// GetPluginEventHistory get specific plugin event history
func (r *TypedRuntimeImpl) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	return r.runtime.GetPluginEventHistory(pluginName, filter)
}

// Shutdown delegates to the underlying runtime.
func (r *TypedRuntimeImpl) Shutdown() {
	r.runtime.Shutdown()
}
