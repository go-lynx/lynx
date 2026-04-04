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

func (r *TypedRuntimePlugin) compatRuntime() (plugins.Runtime, error) {
	if runtime := r.UnderlyingRuntime(); runtime != nil {
		return runtime, nil
	}
	return nil, fmt.Errorf("runtime compatibility wrapper is not initialized")
}

// GetResource retrieves a shared plugin resource by name.
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil, err
	}
	return runtime.GetResource(name)
}

// RegisterResource registers a resource to be shared with other plugins.
func (r *TypedRuntimePlugin) RegisterResource(name string, resource any) error {
	runtime, err := r.compatRuntime()
	if err != nil {
		return err
	}
	return runtime.RegisterResource(name, resource)
}

// GetConfig returns the plugin configuration manager.
func (r *TypedRuntimePlugin) GetConfig() config.Config {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetConfig()
}

// GetLogger returns the plugin logger instance.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetLogger()
}

// EmitEvent broadcasts a plugin event to the unified event bus.
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	runtime.EmitEvent(event)
}

// Close stops the runtime.
func (r *TypedRuntimePlugin) Close() {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.Shutdown()
}

func (r *TypedRuntimePlugin) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.AddListener(listener, filter)
}

func (r *TypedRuntimePlugin) RemoveListener(listener plugins.EventListener) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.RemoveListener(listener)
}

func (r *TypedRuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetEventHistory(filter)
}

func (r *TypedRuntimePlugin) GetPrivateResource(name string) (any, error) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil, err
	}
	return runtime.GetPrivateResource(name)
}

func (r *TypedRuntimePlugin) RegisterPrivateResource(name string, resource any) error {
	runtime, err := r.compatRuntime()
	if err != nil {
		return err
	}
	return runtime.RegisterPrivateResource(name, resource)
}

func (r *TypedRuntimePlugin) GetSharedResource(name string) (any, error) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil, err
	}
	return runtime.GetSharedResource(name)
}

func (r *TypedRuntimePlugin) RegisterSharedResource(name string, resource any) error {
	runtime, err := r.compatRuntime()
	if err != nil {
		return err
	}
	return runtime.RegisterSharedResource(name, resource)
}

func (r *TypedRuntimePlugin) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.EmitPluginEvent(pluginName, eventType, data)
}

func (r *TypedRuntimePlugin) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.AddPluginListener(pluginName, listener, filter)
}

func (r *TypedRuntimePlugin) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetPluginEventHistory(pluginName, filter)
}

func (r *TypedRuntimePlugin) SetEventDispatchMode(mode string) error {
	runtime, err := r.compatRuntime()
	if err != nil {
		return err
	}
	return runtime.SetEventDispatchMode(mode)
}

func (r *TypedRuntimePlugin) SetEventWorkerPoolSize(size int) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.SetEventWorkerPoolSize(size)
}

func (r *TypedRuntimePlugin) SetEventTimeout(timeout time.Duration) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.SetEventTimeout(timeout)
}

func (r *TypedRuntimePlugin) GetEventStats() map[string]any {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetEventStats()
}

func (r *TypedRuntimePlugin) WithPluginContext(pluginName string) plugins.Runtime {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.WithPluginContext(pluginName)
}

func (r *TypedRuntimePlugin) GetCurrentPluginContext() string {
	runtime, err := r.compatRuntime()
	if err != nil {
		return ""
	}
	return runtime.GetCurrentPluginContext()
}

func (r *TypedRuntimePlugin) SetConfig(conf config.Config) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return
	}
	runtime.SetConfig(conf)
}

func (r *TypedRuntimePlugin) GetResourceInfo(name string) (*plugins.ResourceInfo, error) {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil, err
	}
	return runtime.GetResourceInfo(name)
}

func (r *TypedRuntimePlugin) ListResources() []*plugins.ResourceInfo {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.ListResources()
}

func (r *TypedRuntimePlugin) CleanupResources(pluginID string) error {
	runtime, err := r.compatRuntime()
	if err != nil {
		return err
	}
	return runtime.CleanupResources(pluginID)
}

func (r *TypedRuntimePlugin) GetResourceStats() map[string]any {
	runtime, err := r.compatRuntime()
	if err != nil {
		return nil
	}
	return runtime.GetResourceStats()
}
