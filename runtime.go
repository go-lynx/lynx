// Package lynx provides the core plugin orchestration framework used by Lynx applications.
//
// This file (runtime.go) keeps the historical root-package runtime wrapper.
// The actual runtime implementation lives in plugins.UnifiedRuntime; this file
// exists mainly as a backward-compatible adapter.
package lynx

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// TypedRuntimePlugin is a compatibility wrapper around plugins.Runtime.
// Deprecated: prefer plugins.NewUnifiedRuntime and the plugins.Runtime interface directly.
type TypedRuntimePlugin struct {
	// Use unified Runtime as the underlying implementation
	runtime plugins.Runtime
}

// NewTypedRuntimePlugin creates a new compatibility runtime wrapper.
// Deprecated: prefer plugins.NewUnifiedRuntime.
func NewTypedRuntimePlugin() *TypedRuntimePlugin {
	runtime := plugins.NewUnifiedRuntime()
	runtime.SetLogger(log.DefaultLogger)

	r := &TypedRuntimePlugin{
		runtime: runtime,
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
	return r.runtime.GetConfig()
}

// GetLogger returns the plugin logger instance.
// Provides structured logging capabilities.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	return r.runtime.GetLogger()
}

// EmitEvent broadcasts a plugin event to the unified event bus.
// Event will be processed according to its priority and any active filters.
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	r.runtime.EmitEvent(event)
}

// Close stops the runtime (optional to call)
func (r *TypedRuntimePlugin) Close() {
	r.runtime.Shutdown()
}

func (r *TypedRuntimePlugin) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	r.runtime.AddListener(listener, filter)
}

func (r *TypedRuntimePlugin) RemoveListener(listener plugins.EventListener) {
	r.runtime.RemoveListener(listener)
}

func (r *TypedRuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	return r.runtime.GetEventHistory(filter)
}

// RuntimePlugin backward-compatible alias of TypedRuntimePlugin
// Deprecated: prefer plugins.Runtime.
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin creates a runtime plugin (backward-compatible)
// Deprecated: prefer plugins.NewUnifiedRuntime.
func NewRuntimePlugin() *RuntimePlugin {
	return NewTypedRuntimePlugin()
}

// UnderlyingRuntime returns the wrapped plugins.Runtime instance.
func (r *TypedRuntimePlugin) UnderlyingRuntime() plugins.Runtime {
	if r == nil {
		return nil
	}
	return r.runtime
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
