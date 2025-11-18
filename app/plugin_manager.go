// Package app: core definitions for plugin management.
// This file contains core interfaces, types and basic helpers.
package app

import (
	"fmt"
	"sort"
	"sync"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// Compile-time check: ensure DefaultPluginManager implements PluginManager.
var _ PluginManager = (*DefaultPluginManager[plugins.Plugin])(nil)

// PluginManager defines plugin management interfaces.
type PluginManager interface {
	// Basic plugin management
	LoadPlugins(config.Config) error
	UnloadPlugins()
	LoadPluginsByName(config.Config, []string) error
	UnloadPluginsByName([]string)
	GetPlugin(name string) plugins.Plugin
	PreparePlug(config config.Config) ([]plugins.Plugin, error)

	// Runtime and config
	GetRuntime() plugins.Runtime
	SetConfig(config.Config)

	// Resource operations
	StopPlugin(pluginName string) error
	GetResourceStats() map[string]any
	ListResources() []*plugins.ResourceInfo
}

// TypedPluginManager is an alias for PluginManager.
type TypedPluginManager = PluginManager

// DefaultPluginManager is the generic plugin manager implementation.
type DefaultPluginManager[T plugins.Plugin] struct {
	pluginInstances sync.Map // Name() -> Plugin instance
	pluginList      []plugins.Plugin
	factory         *factory.TypedFactory
	mu              sync.RWMutex
	runtime         plugins.Runtime
	config          config.Config
}

// NewPluginManager creates a generic plugin manager.
func NewPluginManager[T plugins.Plugin](pluginList ...T) *DefaultPluginManager[T] {
	manager := &DefaultPluginManager[T]{
		pluginList: make([]plugins.Plugin, 0),
		factory:    factory.GlobalTypedFactory(),
		runtime:    plugins.NewUnifiedRuntime(),
	}

	// register initial plugins
	for _, plugin := range pluginList {
		var p plugins.Plugin = plugin
		if p != nil {
			manager.pluginInstances.Store(p.Name(), p)

			manager.mu.Lock()
			manager.pluginList = append(manager.pluginList, p)
			manager.mu.Unlock()
		}
	}

	return manager
}

// NewTypedPluginManager creates a plugin manager with plugins.Plugin as T.
func NewTypedPluginManager(pluginList ...plugins.Plugin) PluginManager {
	return NewPluginManager[plugins.Plugin](pluginList...)
}

// SetConfig sets global config.
func (m *DefaultPluginManager[T]) SetConfig(conf config.Config) {
	m.config = conf
	if m.runtime != nil {
		m.runtime.SetConfig(conf)
	}
}

// GetRuntime returns the shared runtime.
func (m *DefaultPluginManager[T]) GetRuntime() plugins.Runtime {
	return m.runtime
}

// GetPlugin gets a plugin by Name().
func (m *DefaultPluginManager[T]) GetPlugin(name string) plugins.Plugin {
	if value, ok := m.pluginInstances.Load(name); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	return nil
}

// containsName checks if a name exists in the slice.
func containsName(slice []string, name string) bool {
	for _, item := range slice {
		if item == name {
			return true
		}
	}
	return false
}

// listPluginNamesInternal returns current plugin names (sorted).
// Optimized to use pluginList when available to avoid duplicate traversal
func (m *DefaultPluginManager[T]) listPluginNamesInternal() []string {
	if m == nil {
		return nil
	}

	// Use pluginList if available to avoid traversing sync.Map
	m.mu.RLock()
	if len(m.pluginList) > 0 {
		names := make([]string, 0, len(m.pluginList))
		for _, p := range m.pluginList {
			if p != nil {
				names = append(names, p.Name())
			}
		}
		m.mu.RUnlock()
		sort.Strings(names)
		return names
	}
	m.mu.RUnlock()

	// Fallback to sync.Map traversal if pluginList is empty
	names := make([]string, 0)
	m.pluginInstances.Range(func(key, value any) bool {
		if name, ok := key.(string); ok {
			names = append(names, name)
		}
		return true
	})
	sort.Strings(names)
	return names
}

// listPluginsInternal returns a copy of current plugins (read-locked).
func (m *DefaultPluginManager[T]) listPluginsInternal() []plugins.Plugin {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]plugins.Plugin, 0, len(m.pluginList))
	for _, p := range m.pluginList {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// GetTypedPluginFromManager gets a typed plugin from any PluginManager.
func GetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) (T, error) {
	var zero T
	if m == nil {
		return zero, fmt.Errorf("plugin manager is nil")
	}
	p := m.GetPlugin(name)
	if p == nil {
		return zero, fmt.Errorf("plugin %s not found", name)
	}
	if typed, ok := p.(T); ok {
		return typed, nil
	}
	return zero, fmt.Errorf("plugin %s is not of expected type", name)
}

// MustGetTypedPluginFromManager gets typed plugin or panics.
func MustGetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) T {
	p, err := GetTypedPluginFromManager[T](m, name)
	if err != nil {
		panic(err)
	}
	return p
}
