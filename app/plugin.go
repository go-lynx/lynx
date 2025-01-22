// Package app provides core functionality for plugin management in the Lynx framework.
// It includes interfaces and implementations for managing plugin lifecycle,
// dependencies, and configuration.
package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

// LynxPluginManager defines the interface for managing Lynx plugins.
// It provides methods for loading, unloading, and managing plugin lifecycle.
type LynxPluginManager interface {
	// LoadPlugins loads and initializes all registered plugins using the provided configuration.
	// This method should be called during application startup.
	LoadPlugins(config.Config)

	// UnloadPlugins gracefully unloads all registered plugins.
	// This method should be called during application shutdown.
	UnloadPlugins()

	// LoadPluginsByName loads specific plugins by their names.
	// Parameters:
	//   - names: List of plugin names to load
	//   - conf: Configuration to use for plugin initialization
	LoadPluginsByName([]string, config.Config)

	// UnloadPluginsByName unloads specific plugins by their names.
	// This allows for selective plugin unloading without affecting others.
	UnloadPluginsByName([]string)

	// GetPlugin retrieves a plugin instance by its name.
	// Returns nil if the plugin is not found.
	GetPlugin(name string) plugins.Plugin

	// PreparePlug prepares plugins for loading and returns the names of successfully prepared plugins.
	// This method is called before actual plugin loading to ensure all prerequisites are met.
	PreparePlug(config config.Config) []string
}

// DefaultLynxPluginManager is the default implementation of LynxPluginManager.
// It manages plugin lifecycle and dependencies using a topological sorting approach.
type DefaultLynxPluginManager struct {
	// pluginMap stores plugins indexed by their names for quick lookup
	pluginMap map[string]plugins.Plugin
	// pluginList maintains the ordered list of plugins
	pluginList []plugins.Plugin
	// factory is used to create plugin instances
	factory factory.PluginFactory
}

// NewLynxPluginManager creates a new instance of the default plugin manager.
// Parameters:
//   - plugins: Optional list of plugins to initialize with
//
// Returns:
//   - LynxPluginManager: Initialized plugin manager instance
func NewLynxPluginManager(pluginList ...plugins.Plugin) LynxPluginManager {
	m := &DefaultLynxPluginManager{
		pluginList: make([]plugins.Plugin, 0),
		pluginMap:  make(map[string]plugins.Plugin),
		factory:    factory.GlobalPluginFactory(),
	}

	// Initialize plugin map and list if pluginList are provided
	if pluginList != nil && len(pluginList) > 0 {
		m.pluginList = append(m.pluginList, pluginList...)
		for _, p := range pluginList {
			if p != nil {
				m.pluginMap[p.Name()] = p
			}
		}
	}
	return m
}

// PluginWithLevel represents a plugin with its dependency level in the topology.
// Used internally for dependency sorting and plugin initialization order.
type PluginWithLevel struct {
	// Plugin is the actual plugin instance
	plugins.Plugin
	// level represents the dependency depth (higher means more dependencies)
	level int
}

// TopologicalSort performs a topological sort on plugins based on their dependencies.
// This ensures plugins are loaded in the correct order, respecting their dependencies.
//
// Parameters:
//   - plugins: List of plugins to sort
//
// Returns:
//   - []PluginWithLevel: Sorted list of plugins with their dependency levels
//   - error: Any error that occurred during sorting
func (m *DefaultLynxPluginManager) TopologicalSort(pluginList []plugins.Plugin) ([]PluginWithLevel, error) {
	// Build a map from plugin name to the actual plugin instance
	nameToPlugin := make(map[string]plugins.Plugin)
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		nameToPlugin[p.Name()] = p
	}

	// Build the dependency graph as an adjacency list
	graph := make(map[string][]string)
	for _, p := range pluginList {
		if p == nil {
			continue
		}

		// Get plugin dependencies based on configuration
		var dependencies []string
		if app := Lynx(); app != nil {
			if conf := app.GetGlobalConfig(); conf != nil {
				dependencies = p.GetDependencies(conf.Value(p.ConfPrefix()))
			} else {
				dependencies = p.DependsOn(nil)
			}
		} else {
			dependencies = p.DependsOn(nil)
		}

		// Validate and add dependencies to the graph
		for _, dep := range dependencies {
			if _, exists := nameToPlugin[dep]; !exists {
				return nil, fmt.Errorf("plugin %s depends on unknown plugin %s", p.Name(), dep)
			}
			graph[p.Name()] = append(graph[p.Name()], dep)
		}
	}

	// Perform topological sort using depth-first search
	result := make([]PluginWithLevel, 0, len(pluginList))
	visited := make(map[string]bool)
	level := make(map[string]int)

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			if !contains(result, nameToPlugin[name]) {
				return fmt.Errorf("cyclic dependency detected involving plugin %s", name)
			}
			return nil
		}

		visited[name] = true
		maxLevel := 0

		// Visit all dependencies first
		for _, dep := range graph[name] {
			if err := visit(dep); err != nil {
				return fmt.Errorf("failed to visit dependency %s: %w", dep, err)
			}
			if level[dep] > maxLevel {
				maxLevel = level[dep]
			}
		}

		// Set the level and add to result
		level[name] = maxLevel + 1
		result = append(result, PluginWithLevel{nameToPlugin[name], level[name]})
		return nil
	}

	// Visit all pluginList to build the sorted list
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		if err := visit(p.Name()); err != nil {
			return nil, fmt.Errorf("failed to sort pluginList: %w", err)
		}
	}
	return result, nil
}

// contains checks if a plugin exists in the sorted result list.
// Used internally by TopologicalSort to detect cyclic dependencies.
func contains(slice []PluginWithLevel, plugin plugins.Plugin) bool {
	if plugin == nil {
		return false
	}
	for _, v := range slice {
		if v.Plugin == plugin {
			return true
		}
	}
	return false
}

// LoadPlugins loads all registered plugins in dependency order.
// It performs topological sorting before loading to ensure correct initialization order.
func (m *DefaultLynxPluginManager) LoadPlugins(conf config.Config) {
	// Prepare plugins
	m.PreparePlug(conf)

	if m == nil || len(m.pluginList) == 0 {
		return
	}

	// Sort plugins by dependencies
	sortedPlugins, err := m.TopologicalSort(m.pluginList)
	if err != nil {
		if app := Lynx(); app != nil {
			app.logHelper.Errorf("Failed to sort plugins: %v", err)
		}
		return
	}

	// Load plugins in sorted order
	for _, plugin := range sortedPlugins {
		if plugin.Plugin == nil {
			continue
		}
		if err := plugin.Start(); err != nil {
			if app := Lynx(); app != nil {
				app.logHelper.Errorf("Failed to start plugin %s: %v", plugin.Name(), err)
			}
			return
		}
	}
}

// UnloadPlugins safely unloads all registered plugins.
// It handles errors during unloading without interrupting the unload process.
func (m *DefaultLynxPluginManager) UnloadPlugins() {
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	for _, plugin := range m.pluginList {
		if plugin == nil {
			continue
		}
		if err := plugin.Stop(); err != nil {
			if app := Lynx(); app != nil {
				app.logHelper.Errorf("Failed to unload plugin %s: %v", plugin.Name(), err)
			}
		}
	}
}

// LoadPluginsByName loads specific plugins by their names.
// Parameters:
//   - names: List of plugin names to load
//   - conf: Configuration to use for loading
func (m *DefaultLynxPluginManager) LoadPluginsByName(names []string, conf config.Config) {
	if m == nil || len(names) == 0 || conf == nil {
		return
	}

	// Collect plugins to load
	var pluginsToLoad []plugins.Plugin
	for _, name := range names {
		if plugin, exists := m.pluginMap[name]; exists && plugin != nil {
			pluginsToLoad = append(pluginsToLoad, plugin)
		}
	}

	if len(pluginsToLoad) == 0 {
		return
	}

	// Sort and load plugins
	sortedPlugins, err := m.TopologicalSort(pluginsToLoad)
	if err != nil {
		if app := Lynx(); app != nil {
			app.logHelper.Errorf("Failed to sort plugins for loading: %v", err)
		}
		return
	}

	for _, plugin := range sortedPlugins {
		if plugin.Plugin == nil {
			continue
		}
		if err := plugin.Start(); err != nil {
			if app := Lynx(); app != nil {
				app.logHelper.Errorf("Failed to load plugin %s: %v", plugin.Name(), err)
			}
			return
		}
	}
}

// UnloadPluginsByName safely unloads specific plugins by their names.
// It handles errors during unloading without interrupting the unload process.
func (m *DefaultLynxPluginManager) UnloadPluginsByName(names []string) {
	if m == nil || len(names) == 0 {
		return
	}

	for _, name := range names {
		if plugin, exists := m.pluginMap[name]; exists && plugin != nil {
			if err := plugin.Stop(); err != nil {
				if app := Lynx(); app != nil {
					app.logHelper.Errorf("Failed to unload plugin %s: %v", name, err)
				}
			}
		}
	}
}

// GetPlugin retrieves a plugin instance by its name.
// Returns nil if the plugin manager is nil, the name is empty, or the plugin doesn't exist.
func (m *DefaultLynxPluginManager) GetPlugin(name string) plugins.Plugin {
	if m == nil || name == "" {
		return nil
	}
	return m.pluginMap[name]
}
