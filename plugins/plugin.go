// Package plugins provides the core plugin system for the Lynx framework.
package plugins

import (
	"context"
	"fmt"
	"reflect"
	"sync"
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

// AddPluginListener adds a specific plugin event listener - simplified for unified event bus
func (r *simpleRuntime) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	// TODO: Implement unified event bus listener registration
	// For now, this is a placeholder that maintains API compatibility
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

// Runtime runtime interface
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
		runtime: NewSimpleRuntime(),
	}
}

// simpleRuntime simple runtime implementation
type simpleRuntime struct {
	// Private resources: each plugin manages independently
	privateResources map[string]map[string]any
	// Shared resources: shared by all plugins
	sharedResources map[string]any
	// Resource info: track resource lifecycle
	resourceInfo map[string]*ResourceInfo
	// Configuration
	config config.Config
	// Mutex
	mu sync.RWMutex

	// Plugin context
	currentPluginContext string
	contextMu            sync.RWMutex

	// Event bus manager for unified event handling
	eventManager interface{}
}

func NewSimpleRuntime() *simpleRuntime {
	return &simpleRuntime{
		privateResources: make(map[string]map[string]any),
		sharedResources:  make(map[string]any),
		resourceInfo:     make(map[string]*ResourceInfo),
		eventManager:     nil, // Will be set later to avoid import cycle
	}
}

// EmitEvent emit event - simplified for unified event bus
func (r *simpleRuntime) EmitEvent(event PluginEvent) {
	if event.Type == "" { // Check for zero value of EventType
		return
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// Get global event bus adapter and publish event
	if adapter := GetGlobalEventBusAdapter(); adapter != nil {
		if err := adapter.PublishEvent(event); err != nil {
			// Log error but don't fail the operation
			// TODO: Add proper logging when logger is available
		}
	}
}

// Shutdown close runtime - simplified for unified event bus
func (r *simpleRuntime) Shutdown() {
	// TODO: Implement unified event bus shutdown
	// For now, this is a placeholder that maintains API compatibility
}

// SetEventDispatchMode set event dispatch mode - simplified for unified event bus
func (r *simpleRuntime) SetEventDispatchMode(mode string) error {
	// TODO: Implement unified event bus dispatch mode setting
	// For now, this is a placeholder that maintains API compatibility
	return nil
}

// SetEventWorkerPoolSize set event worker pool size - simplified for unified event bus
func (r *simpleRuntime) SetEventWorkerPoolSize(size int) {
	// TODO: Implement unified event bus worker pool size setting
	// For now, this is a placeholder that maintains API compatibility
}

// SetEventTimeout set event processing timeout - simplified for unified event bus
func (r *simpleRuntime) SetEventTimeout(timeout time.Duration) {
	// TODO: Implement unified event bus timeout setting
	// For now, this is a placeholder that maintains API compatibility
}

// GetEventStats get event system statistics - simplified for unified event bus
func (r *simpleRuntime) GetEventStats() map[string]any {
	// TODO: Implement unified event bus statistics
	// For now, return empty stats to maintain API compatibility
	return map[string]any{
		"events_emitted":       0,
		"events_delivered":     0,
		"events_dropped":       0,
		"listener_panics":      0,
		"listener_timeouts":    0,
		"worker_pool_size":     0,
		"dispatch_mode":        "unified",
		"event_timeout_ms":     0,
		"history_size":         0,
		"max_history":          0,
		"listeners_groups_cnt": 0,
	}
}

// AddListener add event listener - simplified for unified event bus
func (r *simpleRuntime) AddListener(listener EventListener, filter *EventFilter) {
	// TODO: Implement unified event bus listener registration
	// For now, this is a placeholder that maintains API compatibility
}

// RemoveListener remove event listener - simplified for unified event bus
func (r *simpleRuntime) RemoveListener(listener EventListener) {
	// TODO: Implement unified event bus listener removal
	// For now, this is a placeholder that maintains API compatibility
}

// GetEventHistory get event history - simplified for unified event bus
func (r *simpleRuntime) GetEventHistory(filter EventFilter) []PluginEvent {
	// TODO: Implement unified event bus history retrieval
	// For now, return empty slice to maintain API compatibility
	return []PluginEvent{}
}

// GetPluginEventHistory get plugin event history - simplified for unified event bus
func (r *simpleRuntime) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	// TODO: Implement unified event bus plugin history retrieval
	// For now, return empty slice to maintain API compatibility
	return []PluginEvent{}
}

// isEmptyFilter check if filter is empty - simplified for unified event bus
func (r *simpleRuntime) isEmptyFilter(filter EventFilter) bool {
	// TODO: Implement unified event bus filter checking
	// For now, return true to maintain API compatibility
	return true
}

// matchesFilter check if event matches filter - simplified for unified event bus
func (r *simpleRuntime) matchesFilter(event PluginEvent, filter EventFilter) bool {
	// TODO: Implement unified event bus filter matching
	// For now, return true to maintain API compatibility
	return true
}

// getListenerID get listener ID - simplified for unified event bus
func getListenerID(listener EventListener) string {
	return fmt.Sprintf("%p", listener)
}

// GetResource get resource - fix concurrency safety issues
func (r *simpleRuntime) GetResource(name string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Prioritize shared resources
	if value, ok := r.sharedResources[name]; ok {
		return value, nil
	}

	return nil, NewPluginError("runtime", "GetResource", "Resource not found: "+name, nil)
}

// RegisterResource register resource (compatible with old interface, register as shared resource)
func (r *simpleRuntime) RegisterResource(name string, resource any) error {
	return r.RegisterSharedResource(name, resource)
}

// GetPrivateResource get private resource
func (r *simpleRuntime) GetPrivateResource(name string) (any, error) {
	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	if pluginName == "" {
		return nil, NewPluginError("runtime", "GetPrivateResource", "Plugin context not available", nil)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if pluginResources, ok := r.privateResources[pluginName]; ok {
		if resource, exists := pluginResources[name]; exists {
			return resource, nil
		}
	}

	return nil, NewPluginError("runtime", "GetPrivateResource", "Private resource not found: "+name, nil)
}

// RegisterPrivateResource register private resource
func (r *simpleRuntime) RegisterPrivateResource(name string, resource any) error {
	if name == "" {
		return NewPluginError("runtime", "RegisterPrivateResource", "Resource name cannot be empty", nil)
	}
	if resource == nil {
		return NewPluginError("runtime", "RegisterPrivateResource", "Resource cannot be nil", nil)
	}

	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	if pluginName == "" {
		return NewPluginError("runtime", "RegisterPrivateResource", "Plugin context not available", nil)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure the plugin's private resource map exists
	if r.privateResources[pluginName] == nil {
		r.privateResources[pluginName] = make(map[string]any)
	}

	r.privateResources[pluginName][name] = resource
	return nil
}

// GetSharedResource get shared resource - fix concurrency safety issues
func (r *simpleRuntime) GetSharedResource(name string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if value, ok := r.sharedResources[name]; ok {
		return value, nil
	}
	return nil, NewPluginError("runtime", "GetSharedResource", "Shared resource not found: "+name, nil)
}

// RegisterSharedResource register shared resource - fix concurrency safety issues
func (r *simpleRuntime) RegisterSharedResource(name string, resource any) error {
	if name == "" {
		return NewPluginError("runtime", "RegisterSharedResource", "Resource name cannot be empty", nil)
	}
	if resource == nil {
		return NewPluginError("runtime", "RegisterSharedResource", "Resource cannot be nil", nil)
	}

	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sharedResources[name] = resource

	// Record resource info
	r.resourceInfo[name] = &ResourceInfo{
		Name:        name,
		Type:        fmt.Sprintf("%T", resource),
		PluginID:    pluginName,
		IsPrivate:   false,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
		AccessCount: 0,
		Size:        r.estimateResourceSize(resource),
		Metadata:    make(map[string]any),
	}

	return nil
}

// GetConfig get configuration
func (r *simpleRuntime) GetConfig() config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig set configuration
func (r *simpleRuntime) SetConfig(conf config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = conf
}

// GetLogger get logger
func (r *simpleRuntime) GetLogger() log.Logger {
	return log.DefaultLogger
}

// WithPluginContext create runtime with plugin context
func (r *simpleRuntime) WithPluginContext(pluginName string) Runtime {
	if pluginName == "" {
		return r
	}

	// Create a new runtime instance, sharing underlying resources but with a different context
	contextRuntime := &simpleRuntime{
		privateResources:     r.privateResources,
		sharedResources:      r.sharedResources,
		config:               r.config,
		currentPluginContext: pluginName,
		eventManager:         r.eventManager,
	}

	return contextRuntime
}

// GetCurrentPluginContext get current plugin context
func (r *simpleRuntime) GetCurrentPluginContext() string {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	return r.currentPluginContext
}

// GetResourceInfo get resource info
func (r *simpleRuntime) GetResourceInfo(name string) (*ResourceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, exists := r.resourceInfo[name]; exists {
		return info, nil
	}
	return nil, NewPluginError("runtime", "GetResourceInfo", "Resource info not found: "+name, nil)
}

// ListResources list all resources
func (r *simpleRuntime) ListResources() []*ResourceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var resources []*ResourceInfo
	for _, info := range r.resourceInfo {
		resources = append(resources, info)
	}
	return resources
}

// CleanupResources clean up resources for a specific plugin
func (r *simpleRuntime) CleanupResources(pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clean up private resources
	if pluginResources, exists := r.privateResources[pluginID]; exists {
		for resourceName := range pluginResources {
			delete(r.resourceInfo, resourceName)
		}
		delete(r.privateResources, pluginID)
	}

	// Clean up shared resources (if the plugin is the owner)
	var sharedResourcesToRemove []string
	for name, info := range r.resourceInfo {
		if info.PluginID == pluginID && !info.IsPrivate {
			sharedResourcesToRemove = append(sharedResourcesToRemove, name)
		}
	}

	for _, name := range sharedResourcesToRemove {
		delete(r.sharedResources, name)
		delete(r.resourceInfo, name)
	}

	return nil
}

// GetResourceStats get resource statistics
func (r *simpleRuntime) GetResourceStats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]any{
		"total_resources":        len(r.resourceInfo),
		"private_resources":      0,
		"shared_resources":       0,
		"total_size_bytes":       int64(0),
		"plugins_with_resources": 0,
	}

	pluginSet := make(map[string]bool)

	for _, info := range r.resourceInfo {
		if info.IsPrivate {
			stats["private_resources"] = stats["private_resources"].(int) + 1
		} else {
			stats["shared_resources"] = stats["shared_resources"].(int) + 1
		}
		stats["total_size_bytes"] = stats["total_size_bytes"].(int64) + info.Size
		pluginSet[info.PluginID] = true
	}

	stats["plugins_with_resources"] = len(pluginSet)
	return stats
}

// estimateResourceSize estimate resource size
func (r *simpleRuntime) estimateResourceSize(resource any) int64 {
	if resource == nil {
		return 0
	}

	// Use reflection to estimate size
	val := reflect.ValueOf(resource)
	return r.estimateValueSize(val)
}

// estimateValueSize recursively estimate value size
func (r *simpleRuntime) estimateValueSize(val reflect.Value) int64 {
	if !val.IsValid() {
		return 0
	}

	switch val.Kind() {
	case reflect.String:
		return int64(val.Len())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return 8
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return 8
	case reflect.Float32, reflect.Float64:
		return 8
	case reflect.Bool:
		return 1
	case reflect.Slice, reflect.Array:
		size := int64(0)
		for i := 0; i < val.Len(); i++ {
			size += r.estimateValueSize(val.Index(i))
		}
		return size
	case reflect.Map:
		size := int64(0)
		for _, key := range val.MapKeys() {
			size += r.estimateValueSize(key)
			size += r.estimateValueSize(val.MapIndex(key))
		}
		return size
	case reflect.Struct:
		size := int64(0)
		for i := 0; i < val.NumField(); i++ {
			size += r.estimateValueSize(val.Field(i))
		}
		return size
	case reflect.Ptr:
		if val.IsNil() {
			return 8 // Size of pointer itself
		}
		return 8 + r.estimateValueSize(val.Elem())
	default:
		return 8 // Default size
	}
}

// EmitPluginEvent emit plugin namespace event
func (r *simpleRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	event := PluginEvent{
		Type:      EventType(eventType),
		PluginID:  pluginName,
		Source:    pluginName,
		Metadata:  data,
		Timestamp: time.Now().Unix(),
	}
	r.EmitEvent(event)
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
