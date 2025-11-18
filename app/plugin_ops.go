// Package app: plugin manager operations (load/unload, resources, helpers).
package app

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/subscribe"
	"github.com/go-lynx/lynx/plugins"
)

// LoadPlugins loads and starts all plugins.
func (m *DefaultPluginManager[T]) LoadPlugins(conf config.Config) error {
	m.SetConfig(conf)

	preparedPlugins, err := m.PreparePlug(conf)
	if err != nil {
		return fmt.Errorf("failed to prepare plugins: %w", err)
	}
	if len(preparedPlugins) == 0 {
		return fmt.Errorf("no plugins prepared")
	}

	sortedPlugins, err := m.TopologicalSort(preparedPlugins)
	if err != nil {
		// Provide detailed error information for debugging
		return fmt.Errorf("failed to sort plugins (dependency resolution failed): %w. "+
			"This usually indicates circular dependencies or missing required dependencies. "+
			"Please check plugin dependency declarations", err)
	}

	if err := m.loadSortedPluginsByLevel(sortedPlugins); err != nil {
		return err
	}

	if Lynx() != nil && Lynx().bootConfig != nil && Lynx().bootConfig.Lynx != nil && Lynx().bootConfig.Lynx.Subscriptions != nil {
		disc := Lynx().GetControlPlane().NewServiceDiscovery()
		if disc != nil {
			routerFactory := func(service string) selector.NodeFilter {
				return Lynx().GetControlPlane().NewNodeRouter(service)
			}
			conns, err := subscribe.BuildGrpcSubscriptions(Lynx().bootConfig.Lynx.Subscriptions, disc, routerFactory)
			if err != nil {
				return fmt.Errorf("build grpc subscriptions failed: %w", err)
			}
			Lynx().grpcSubs = conns
		} else {
			log.Warnf("service discovery is nil, skip building grpc subscriptions")
		}
	}

	return nil
}

// UnloadPlugins stops and unloads all plugins.
func (m *DefaultPluginManager[T]) UnloadPlugins() {
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	// Emit plugin manager shutdown event
	m.emitPluginManagerShutdownEvent()

	timeout := m.getStopTimeout()

	var ordered []plugins.Plugin
	sorted, err := m.TopologicalSort(m.pluginList)
	if err != nil {
		// Topological sort failed - provide detailed error information
		log.Errorf("topological sort failed during unload: %v", err)
		log.Errorf("This may indicate circular dependencies or missing dependencies")
		log.Errorf("Attempting to unload plugins in reverse order as fallback")
		// Still try to unload in reverse order, but log the error
		ordered = make([]plugins.Plugin, len(m.pluginList))
		copy(ordered, m.pluginList)
		// Reverse the order
		for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
			ordered[i], ordered[j] = ordered[j], ordered[i]
		}
	} else {
		tmp := make([]plugins.Plugin, 0, len(sorted))
		for _, w := range sorted {
			if w.Plugin != nil {
				tmp = append(tmp, w.Plugin)
			}
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			ordered = append(ordered, tmp[i])
		}
	}

	for _, plugin := range ordered {
		p := plugin
		if p == nil {
			continue
		}

		// Emit plugin unloading event
		m.emitPluginUnloadEvent(p.ID(), p.Name())

		if err := m.safeStopPlugin(p, timeout); err != nil {
			log.Errorf("Failed to unload plugin %s: %v", p.Name(), err)
			// Emit error event
			m.emitPluginErrorEvent(p.ID(), p.Name(), "unload", err)
		}
		if err := m.runtime.CleanupResources(p.ID()); err != nil {
			log.Errorf("Failed to cleanup resources for plugin %s: %v", p.Name(), err)
			// Emit resource cleanup error event
			m.emitResourceCleanupErrorEvent(p.ID(), p.Name(), err)
		}
		m.pluginInstances.Delete(p.Name())
	}

	m.mu.Lock()
	m.pluginList = nil
	m.mu.Unlock()
}

// LoadPluginsByName loads a subset of plugins by Name().
func (m *DefaultPluginManager[T]) LoadPluginsByName(conf config.Config, pluginNames []string) error {
	m.SetConfig(conf)

	preparedPlugins, err := m.PreparePlug(conf)
	if err != nil {
		return err
	}

	var targetPlugins []plugins.Plugin
	pluginMap := make(map[string]plugins.Plugin)
	for _, plugin := range preparedPlugins {
		pluginMap[plugin.Name()] = plugin
	}

	for _, name := range pluginNames {
		if plugin, exists := pluginMap[name]; exists {
			targetPlugins = append(targetPlugins, plugin)
		} else {
			return fmt.Errorf("plugin %s not found", name)
		}
	}

	sortedPlugins, err := m.TopologicalSort(targetPlugins)
	if err != nil {
		// Provide detailed error information for debugging
		return fmt.Errorf("failed to sort plugins (dependency resolution failed): %w. "+
			"This usually indicates circular dependencies or missing required dependencies. "+
			"Please check plugin dependency declarations", err)
	}

	if err := m.loadSortedPluginsByLevel(sortedPlugins); err != nil {
		return err
	}
	return nil
}

// UnloadPluginsByName unloads a subset of plugins by Name().
func (m *DefaultPluginManager[T]) UnloadPluginsByName(names []string) {
	if m == nil || len(names) == 0 {
		return
	}

	timeout := m.getStopTimeout()

	var subset []plugins.Plugin
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	m.pluginInstances.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		if _, wanted := nameSet[name]; !wanted {
			return true
		}
		if p, ok2 := value.(plugins.Plugin); ok2 && p != nil {
			subset = append(subset, p)
		}
		return true
	})

	if len(subset) == 0 {
		for _, n := range names {
			log.Infof("plugin %s not found, skip unload", n)
		}
		return
	}

	var ordered []plugins.Plugin
	sorted, err := m.TopologicalSort(subset)
	if err != nil {
		// Topological sort failed - provide detailed error information
		log.Errorf("topological sort failed for subset unload: %v", err)
		log.Errorf("This may indicate circular dependencies or missing dependencies")
		log.Errorf("Attempting to unload plugins in given order as fallback")
		// Fallback to given order, but log the error
		for _, n := range names {
			if obj, ok := m.pluginInstances.Load(n); ok {
				if p, ok2 := obj.(plugins.Plugin); ok2 && p != nil {
					ordered = append(ordered, p)
				}
			}
		}
	} else {
		tmp := make([]plugins.Plugin, 0, len(sorted))
		for _, w := range sorted {
			if w.Plugin != nil {
				tmp = append(tmp, w.Plugin)
			}
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			ordered = append(ordered, tmp[i])
		}
	}

	for _, p := range ordered {
		if p == nil {
			continue
		}

		// Emit plugin unloading event
		m.emitPluginUnloadEvent(p.ID(), p.Name())

		if err := m.safeStopPlugin(p, timeout); err != nil {
			log.Errorf("Failed to unload plugin %s: %v", p.Name(), err)
			// Emit error event
			m.emitPluginErrorEvent(p.ID(), p.Name(), "unload", err)
		}
		if err := m.runtime.CleanupResources(p.ID()); err != nil {
			log.Errorf("Failed to cleanup resources for plugin %s: %v", p.Name(), err)
			// Emit resource cleanup error event
			m.emitResourceCleanupErrorEvent(p.ID(), p.Name(), err)
		}
		m.pluginInstances.Delete(p.Name())
	}

	m.mu.Lock()
	var newList []plugins.Plugin
	for _, item := range m.pluginList {
		if item != nil {
			if _, removed := nameSet[item.Name()]; !removed {
				newList = append(newList, item)
			}
		}
	}
	m.pluginList = newList
	m.mu.Unlock()
}

// StopPlugin stops a single plugin by Name().
func (m *DefaultPluginManager[T]) StopPlugin(pluginName string) error {
	plugin, exists := m.pluginInstances.Load(pluginName)
	if !exists {
		log.Infof("plugin %s not found, skip stop", pluginName)
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	p, ok := plugin.(plugins.Plugin)
	if !ok {
		return fmt.Errorf("invalid plugin instance for %s", pluginName)
	}
	if p == nil {
		_ = m.runtime.CleanupResources(pluginName)
		m.pluginInstances.Delete(pluginName)
		return nil
	}

	// Emit plugin stopping event
	m.emitPluginUnloadEvent(p.ID(), p.Name())

	timeout := m.getStopTimeout()
	if err := m.safeStopPlugin(p, timeout); err != nil {
		// Emit error event
		m.emitPluginErrorEvent(p.ID(), p.Name(), "stop", err)
		return fmt.Errorf("failed to stop plugin %s: %w", pluginName, err)
	}
	if err := m.runtime.CleanupResources(p.ID()); err != nil {
		// Emit resource cleanup error event
		m.emitResourceCleanupErrorEvent(p.ID(), p.Name(), err)
		return fmt.Errorf("failed to cleanup resources for plugin %s: %w", pluginName, err)
	}
	m.pluginInstances.Delete(pluginName)
	return nil
}

// GetResourceStats Resource helpers.
func (m *DefaultPluginManager[T]) GetResourceStats() map[string]any {
	return m.runtime.GetResourceStats()
}

func (m *DefaultPluginManager[T]) ListResources() []*plugins.ResourceInfo {
	return m.runtime.ListResources()
}

// ListPluginNames Public helpers for any PluginManager.
func ListPluginNames(m PluginManager) []string {
	if m == nil {
		return nil
	}
	type nameLister interface{ listPluginNamesInternal() []string }
	if l, ok := m.(nameLister); ok {
		return l.listPluginNamesInternal()
	}
	return nil
}

func Plugins(m PluginManager) []plugins.Plugin {
	if m == nil {
		return nil
	}
	type pluginsLister interface{ listPluginsInternal() []plugins.Plugin }
	if l, ok := m.(pluginsLister); ok {
		return l.listPluginsInternal()
	}
	return nil
}

// emitPluginUnloadEvent emits a plugin unload event
func (m *DefaultPluginManager[T]) emitPluginUnloadEvent(pluginID, pluginName string) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventPluginStopping,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "lifecycle",
		PluginID:  pluginID,
		Status:    plugins.StatusInactive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   "unload",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitPluginErrorEvent emits a plugin error event
func (m *DefaultPluginManager[T]) emitPluginErrorEvent(pluginID, pluginName, operation string, err error) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityHigh,
		Source:    "plugin-manager",
		Category:  "error",
		PluginID:  pluginID,
		Status:    plugins.StatusFailed,
		Timestamp: time.Now().Unix(),
		Error:     err,
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   operation,
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitResourceCleanupErrorEvent emits a resource cleanup error event
func (m *DefaultPluginManager[T]) emitResourceCleanupErrorEvent(pluginID, pluginName string, err error) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "error",
		PluginID:  pluginID,
		Status:    plugins.StatusFailed,
		Timestamp: time.Now().Unix(),
		Error:     err,
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   "resource_cleanup",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitPluginManagerShutdownEvent emits a plugin manager shutdown event
func (m *DefaultPluginManager[T]) emitPluginManagerShutdownEvent() {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventType("system.plugin_manager_shutdown"),
		Priority:  plugins.PriorityHigh,
		Source:    "plugin-manager",
		Category:  "system",
		PluginID:  "system",
		Status:    plugins.StatusInactive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"operation": "shutdown",
			"reason":    "application_close",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}
