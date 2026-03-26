package lynx

import (
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// TypedRuntimePlugin is a compatibility wrapper around plugins.Runtime.
// Deprecated: prefer plugins.NewUnifiedRuntime and the plugins.Runtime interface directly.
type TypedRuntimePlugin struct {
	runtime plugins.Runtime
}

// NewTypedRuntimePlugin creates a new compatibility runtime wrapper.
// Deprecated: prefer NewDefaultRuntime or plugins.NewUnifiedRuntime.
func NewTypedRuntimePlugin() *TypedRuntimePlugin {
	return &TypedRuntimePlugin{runtime: NewDefaultRuntime()}
}

// RuntimePlugin backward-compatible alias of TypedRuntimePlugin.
// Deprecated: prefer plugins.Runtime.
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin creates a runtime plugin (backward-compatible).
// Deprecated: prefer NewDefaultRuntime or plugins.NewUnifiedRuntime.
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

// GetResource retrieves a shared plugin resource by name.
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	return r.runtime.GetResource(name)
}

// RegisterResource registers a resource to be shared with other plugins.
func (r *TypedRuntimePlugin) RegisterResource(name string, resource any) error {
	return r.runtime.RegisterResource(name, resource)
}

// GetConfig returns the plugin configuration manager.
func (r *TypedRuntimePlugin) GetConfig() config.Config {
	return r.runtime.GetConfig()
}

// GetLogger returns the plugin logger instance.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	return r.runtime.GetLogger()
}

// EmitEvent broadcasts a plugin event to the unified event bus.
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	r.runtime.EmitEvent(event)
}

// Close stops the runtime.
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

func (r *TypedRuntimePlugin) GetPrivateResource(name string) (any, error) {
	return r.runtime.GetPrivateResource(name)
}

func (r *TypedRuntimePlugin) RegisterPrivateResource(name string, resource any) error {
	return r.runtime.RegisterPrivateResource(name, resource)
}

func (r *TypedRuntimePlugin) GetSharedResource(name string) (any, error) {
	return r.runtime.GetSharedResource(name)
}

func (r *TypedRuntimePlugin) RegisterSharedResource(name string, resource any) error {
	return r.runtime.RegisterSharedResource(name, resource)
}

func (r *TypedRuntimePlugin) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	r.runtime.EmitPluginEvent(pluginName, eventType, data)
}

func (r *TypedRuntimePlugin) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	r.runtime.AddPluginListener(pluginName, listener, filter)
}

func (r *TypedRuntimePlugin) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	return r.runtime.GetPluginEventHistory(pluginName, filter)
}

func (r *TypedRuntimePlugin) SetEventDispatchMode(mode string) error {
	return r.runtime.SetEventDispatchMode(mode)
}

func (r *TypedRuntimePlugin) SetEventWorkerPoolSize(size int) {
	r.runtime.SetEventWorkerPoolSize(size)
}

func (r *TypedRuntimePlugin) SetEventTimeout(timeout time.Duration) {
	r.runtime.SetEventTimeout(timeout)
}

func (r *TypedRuntimePlugin) GetEventStats() map[string]any {
	return r.runtime.GetEventStats()
}

func (r *TypedRuntimePlugin) WithPluginContext(pluginName string) plugins.Runtime {
	return r.runtime.WithPluginContext(pluginName)
}

func (r *TypedRuntimePlugin) GetCurrentPluginContext() string {
	return r.runtime.GetCurrentPluginContext()
}

func (r *TypedRuntimePlugin) SetConfig(conf config.Config) {
	r.runtime.SetConfig(conf)
}

func (r *TypedRuntimePlugin) GetResourceInfo(name string) (*plugins.ResourceInfo, error) {
	return r.runtime.GetResourceInfo(name)
}

func (r *TypedRuntimePlugin) ListResources() []*plugins.ResourceInfo {
	return r.runtime.ListResources()
}

func (r *TypedRuntimePlugin) CleanupResources(pluginID string) error {
	return r.runtime.CleanupResources(pluginID)
}

func (r *TypedRuntimePlugin) GetResourceStats() map[string]any {
	return r.runtime.GetResourceStats()
}
