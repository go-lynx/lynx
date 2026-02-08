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

	// StatusUpgrading indicates that the plugin is currently being upgraded
	// During this state, the plugin may be partially operational
	// Should transition to StatusActive or StatusFailed
	StatusUpgrading

	// StatusRollback indicates that the plugin is rolling back from a failed upgrade
	// Attempting to restore the previous working state
	// Should transition to StatusActive or StatusFailed
	StatusRollback
)

// UpgradeCapability defines the various ways a plugin can be upgraded during runtime
type UpgradeCapability int

const (
	// UpgradeNone indicates the plugin does not support any runtime upgrades
	// Must be stopped and restarted to apply any changes
	UpgradeNone UpgradeCapability = iota

	// UpgradeConfig indicates the plugin can update its configuration without restart
	// Supports runtime configuration changes but not code updates
	UpgradeConfig

	// UpgradeVersion indicates the plugin can perform version upgrades without restart
	// Supports both configuration and code updates during runtime
	UpgradeVersion

	// UpgradeReplace indicates the plugin supports complete replacement during runtime
	// Can be entirely replaced with a new instance while maintaining service
	UpgradeReplace
)

// Plugin is the minimal interface that all plugins must implement
// It combines basic metadata and lifecycle management capabilities
type Plugin interface {
	Metadata
	Lifecycle
	LifecycleSteps
	DependencyAware
}

// LifecycleWithContext defines optional context-aware lifecycle methods.
// Plugins implementing this interface can receive cancellation/timeout signals
// and are encouraged to stop work promptly when the context is done.
// This interface is backward-compatible and optional; if not implemented,
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

// Metadata defines methods for retrieving plugin metadata
// This interface provides essential information about the plugin
type Metadata interface {
	// ID returns the unique identifier of the plugin
	// This ID must be unique across all plugins in the system
	ID() string

	// Name returns the display name of the plugin
	// This is a human-readable name used for display purposes
	Name() string

	// Description returns a detailed description of the plugin
	// Should provide information about the plugin's purpose and functionality
	Description() string

	// Version returns the semantic version of the plugin
	// Should follow semver format (MAJOR.MINOR.PATCH)
	Version() string

	// Weight returns the weight value
	Weight() int
}

// Lifecycle defines the basic lifecycle methods for a plugin
// Handles initialization, operation, and termination of the plugin
type Lifecycle interface {
	// Initialize prepares the plugin for use
	// Sets up resources, connections, and internal state
	// Returns error if initialization fails
	Initialize(plugin Plugin, rt Runtime) error

	// Start begins the plugin's main functionality
	// Should only be called after successful initialization
	// Returns error if startup fails
	Start(plugin Plugin) error

	// Stop gracefully terminates the plugin's functionality
	// Releases resources and closes connections
	// Returns error if shutdown fails
	Stop(plugin Plugin) error

	// Status returns the current status of the plugin
	// Provides real-time state information
	Status(plugin Plugin) PluginStatus
}

type LifecycleSteps interface {
	InitializeResources(rt Runtime) error
	StartupTasks() error
	CleanupTasks() error
	CheckHealth() error
}

// ResourceInfo resource information
type ResourceInfo struct {
	Name        string
	Type        string
	PluginID    string
	IsPrivate   bool
	CreatedAt   time.Time
	LastUsedAt  time.Time
	AccessCount int64
	Size        int64 // Resource size (bytes)
	Metadata    map[string]any
}

// ResourceManager resource manager interface
type ResourceManager interface {
	// GetResource retrieves a shared plugin resource by name
	// Returns the resource and any error encountered
	GetResource(name string) (any, error)

	// RegisterResource registers a resource to be shared with other plugins
	// Returns error if registration fails
	RegisterResource(name string, resource any) error

	// New: Resource lifecycle management
	GetResourceInfo(name string) (*ResourceInfo, error)
	ListResources() []*ResourceInfo
	CleanupResources(pluginID string) error
	GetResourceStats() map[string]any
}

// TypedResourceManager generic resource manager interface
type TypedResourceManager interface {
	ResourceManager
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

// ConfigProvider provides access to plugin configuration
// Manages plugin configuration loading and access
type ConfigProvider interface {
	// GetConfig returns the plugin configuration manager
	// Provides access to configuration values and updates
	GetConfig() config.Config
}

// LogProvider provides access to logging functionality
// Manages plugin logging capabilities
type LogProvider interface {
	// GetLogger returns the plugin logger instance
	// Provides structured logging capabilities
	GetLogger() log.Logger
}

// Suspendable defines methods for temporary plugin suspension
// Manages temporary plugin deactivation and reactivation
type Suspendable interface {
	// Suspend temporarily suspends plugin operations
	// Pauses plugin activity while maintaining state
	Suspend() error

	// Resume restores plugin operations from a suspended state
	// Resumes normal operation without reinitialization
	Resume() error
}

// Configurable defines methods for plugin configuration management
// Manages plugin configuration updates and validation
type Configurable interface {
	// Configure applies and validates the given configuration
	// Updates plugin configuration during runtime
	Configure(conf any) error
}

// DependencyAware defines methods for plugin dependency management
// Manages plugin dependencies and their relationships
type DependencyAware interface {
	// GetDependencies returns the list of plugin dependencies
	// Lists all required and optional dependencies
	GetDependencies() []Dependency
}

// EventHandler defines methods for plugin event handling
// Processes plugin-related events and notifications
type EventHandler interface {
	// HandleEvent processes plugin lifecycle events
	// Handles various plugin system events
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
