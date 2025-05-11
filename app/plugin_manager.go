// Package app provides core functionality for plugin management in the Lynx framework.
// 包 app 为 Lynx 框架提供插件管理的核心功能。
// It includes interfaces and implementations for managing plugin lifecycle,
// dependencies, and configuration.
// 它包含用于管理插件生命周期、依赖关系和配置的接口与实现。
package app

import (
	"fmt"
	factory2 "github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/app/log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
)

// LynxPluginManager defines the interface for managing Lynx plugins.
// LynxPluginManager 定义了管理 Lynx 插件的接口。
// It provides methods for loading, unloading, and managing plugin lifecycle.
// 它提供了加载、卸载和管理插件生命周期的方法。
type LynxPluginManager interface {
	// LoadPlugins loads and initializes all registered plugins using the provided configuration.
	// LoadPlugins 使用提供的配置加载并初始化所有已注册的插件。
	// This method should be called during application startup.
	// 此方法应在应用程序启动时调用。
	LoadPlugins(config.Config)

	// UnloadPlugins gracefully unloads all registered plugins.
	// UnloadPlugins 优雅地卸载所有已注册的插件。
	// This method should be called during application shutdown.
	// 此方法应在应用程序关闭时调用。
	UnloadPlugins()

	// LoadPluginsByName loads specific plugins by their names.
	// LoadPluginsByName 根据插件名称加载特定的插件。
	// Parameters:
	//   - names: List of plugin names to load
	//   - names: 要加载的插件名称列表
	//   - conf: Configuration to use for plugin initialization
	//   - conf: 用于插件初始化的配置
	LoadPluginsByName([]string, config.Config)

	// UnloadPluginsByName unloads specific plugins by their names.
	// UnloadPluginsByName 根据插件名称卸载特定的插件。
	// This allows for selective plugin unloading without affecting others.
	// 这允许在不影响其他插件的情况下选择性地卸载插件。
	UnloadPluginsByName([]string)

	// GetPlugin retrieves a plugin instance by its name.
	// GetPlugin 根据插件名称获取插件实例。
	// Returns nil if the plugin is not found.
	// 如果未找到插件，则返回 nil。
	GetPlugin(name string) plugins.Plugin

	// PreparePlug prepares plugins for loading and returns the names of successfully prepared plugins.
	// PreparePlug 为插件加载做准备，并返回成功准备的插件名称列表。
	// This method is called before actual plugin loading to ensure all prerequisites are met.
	// 此方法在实际加载插件之前调用，以确保满足所有先决条件。
	PreparePlug(config config.Config) []string
}

// DefaultLynxPluginManager is the default implementation of LynxPluginManager.
// DefaultLynxPluginManager 是 LynxPluginManager 接口的默认实现。
// It manages plugin lifecycle and dependencies using a topological sorting approach.
// 它使用拓扑排序的方法管理插件的生命周期和依赖关系。
type DefaultLynxPluginManager struct {
	// pluginMap stores plugins indexed by their names for quick lookup
	// pluginMap 以插件名称为键存储插件实例，便于快速查找。
	pluginMap map[string]plugins.Plugin
	// pluginList maintains the ordered list of plugins
	// pluginList 维护一个有序的插件列表。
	pluginList []plugins.Plugin
	// factory is used to create plugin instances
	// factory 用于创建插件实例。
	factory factory2.PluginFactory
}

// NewLynxPluginManager creates a new instance of the default plugin manager.
// NewLynxPluginManager 创建一个默认插件管理器的新实例。
// Parameters:
//   - plugins: Optional list of plugins to initialize with
//   - plugins: 可选的用于初始化的插件列表
//
// Returns:
//   - LynxPluginManager: Initialized plugin manager instance
//   - LynxPluginManager: 初始化后的插件管理器实例
func NewLynxPluginManager(pluginList ...plugins.Plugin) LynxPluginManager {
	m := &DefaultLynxPluginManager{
		pluginList: make([]plugins.Plugin, 0),
		pluginMap:  make(map[string]plugins.Plugin),
		factory:    factory2.GlobalPluginFactory(),
	}

	// Initialize plugin map and list if pluginList are provided
	// 如果提供了插件列表，则初始化 pluginMap 和 pluginList
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
// PluginWithLevel 表示一个带有拓扑依赖级别的插件。
// Used internally for dependency sorting and plugin initialization order.
// 内部用于依赖排序和插件初始化顺序。
type PluginWithLevel struct {
	// Plugin is the actual plugin instance
	// Plugin 是实际的插件实例。
	plugins.Plugin
	// level represents the dependency depth (higher means more dependencies)
	// level 表示依赖深度（值越高意味着依赖越多）。
	level int
}

// TopologicalSort performs a topological sort on the plugin list based on their dependencies.
// TopologicalSort 根据插件的依赖关系对插件列表进行拓扑排序。
// It returns a sorted list of plugins with their dependency levels, where plugins at the same
// level can be initialized in parallel.
// 它返回一个按依赖级别排序的插件列表，同一级别的插件可以并行初始化。
//
// The function uses a depth-first search algorithm to:
// 该函数使用深度优先搜索算法来：
// 1. Build a dependency graph
// 1. 构建依赖图
// 2. Detect cyclic dependencies
// 2. 检测循环依赖
// 3. Assign dependency levels to plugins
// 3. 为插件分配依赖级别
// 4. Sort plugins based on their dependency order
// 4. 根据插件的依赖顺序进行排序
//
// Parameters:
//   - pluginList: List of plugins to be sorted
//   - pluginList: 要排序的插件列表
//
// Returns:
//   - []PluginWithLevel: Sorted list of plugins with their dependency levels
//   - []PluginWithLevel: 按依赖级别排序的插件列表
//   - error: Error if cyclic dependency is detected or if a required plugin is missing
//   - error: 如果检测到循环依赖或缺少必需的插件，则返回错误
func (m *DefaultLynxPluginManager) TopologicalSort(pluginList []plugins.Plugin) ([]PluginWithLevel, error) {
	// Build a map from plugin name to the actual plugin instance
	// 构建一个从插件名称到实际插件实例的映射
	nameToPlugin := make(map[string]plugins.Plugin)
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		nameToPlugin[p.Name()] = p
	}

	// Build the dependency graph as an adjacency list
	// 以邻接表的形式构建依赖图
	graph := make(map[string][]string)
	for _, p := range pluginList {
		if p == nil {
			continue
		}

		// Check if plugin implements DependencyAware interface
		// 检查插件是否实现了 DependencyAware 接口
		depAware, ok := p.(plugins.DependencyAware)
		if !ok {
			// Plugin doesn't implement DependencyAware, treat it as having no dependencies
			// 插件未实现 DependencyAware 接口，将其视为没有依赖
			continue
		}

		// Get plugin dependencies using DependencyAware interface
		// 使用 DependencyAware 接口获取插件的依赖
		dependencies := depAware.GetDependencies()
		if dependencies == nil {
			continue
		}

		// Validate and add dependencies to the graph
		// 验证依赖并将其添加到图中
		for _, dep := range dependencies {
			// Validate dependency object
			// 验证依赖对象
			if dep.ID == "" {
				return nil, fmt.Errorf("plugin %s has an invalid dependency with empty ID", p.Name())
			}

			// Check if dependency exists
			// 检查依赖是否存在
			if _, exists := nameToPlugin[dep.ID]; !exists {
				if dep.Required {
					return nil, fmt.Errorf("plugin %s requires missing plugin %s", p.Name(), dep.ID)
				}
				// Skip optional dependencies that are not available
				// 跳过不可用的可选依赖
				continue
			}

			// Add to dependency graph
			// 添加到依赖图中
			graph[p.Name()] = append(graph[p.Name()], dep.ID)
		}
	}

	// Perform topological sort using depth-first search
	// 使用深度优先搜索进行拓扑排序
	result := make([]PluginWithLevel, 0, len(pluginList))
	visited := make(map[string]bool)
	level := make(map[string]int)
	inProgress := make(map[string]bool) // Track nodes being visited in current DFS path
	// 跟踪当前深度优先搜索路径中正在访问的节点

	var visit func(string) error
	visit = func(name string) error {
		// Check for cyclic dependencies
		// 检查是否存在循环依赖
		if inProgress[name] {
			return fmt.Errorf("cyclic dependency detected involving plugin %s", name)
		}

		// Skip if already fully visited
		// 如果已经完全访问过，则跳过
		if visited[name] {
			return nil
		}

		inProgress[name] = true
		defer func() { inProgress[name] = false }()

		maxLevel := 0

		// Visit all dependencies first
		// 先访问所有依赖
		for _, dep := range graph[name] {
			if err := visit(dep); err != nil {
				return fmt.Errorf("failed to visit dependency %s: %w", dep, err)
			}
			if level[dep] > maxLevel {
				maxLevel = level[dep]
			}
		}

		// Set the level and add to result
		// 设置级别并添加到结果列表中
		level[name] = maxLevel + 1
		result = append(result, PluginWithLevel{nameToPlugin[name], level[name]})
		visited[name] = true

		return nil
	}

	// Visit all plugins to build the sorted list
	// 访问所有插件以构建排序后的列表
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		if err := visit(p.Name()); err != nil {
			return nil, fmt.Errorf("failed to sort plugins: %w", err)
		}
	}

	return result, nil
}

// contains checks if a plugin exists in the sorted result list.
// contains 检查一个插件是否存在于排序后的结果列表中。
// Used internally by TopologicalSort to detect cyclic dependencies.
// 供 TopologicalSort 内部用于检测循环依赖。
func contains(slice []PluginWithLevel, plugin plugins.Plugin) bool {
	// 如果插件实例为 nil，直接返回 false
	if plugin == nil {
		return false
	}
	// 遍历切片，检查每个元素的插件实例是否与目标插件相同
	for _, v := range slice {
		if v.Plugin == plugin {
			return true
		}
	}
	// 未找到匹配的插件，返回 false
	return false
}

// LoadPlugins loads all registered plugins in dependency order.
// LoadPlugins 按照依赖顺序加载所有已注册的插件。
// It performs topological sorting before loading to ensure correct initialization order.
// 它在加载前进行拓扑排序，以确保正确的初始化顺序。
func (m *DefaultLynxPluginManager) LoadPlugins(conf config.Config) {
	// Prepare plugins
	// 准备插件
	m.PreparePlug(conf)

	// 如果插件管理器为 nil 或者没有已注册的插件，直接返回
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	// Sort plugins by dependencies
	// 根据依赖关系对插件进行排序
	sortedPlugins, err := m.TopologicalSort(m.pluginList)
	if err != nil {
		// 如果获取 Lynx 应用实例不为 nil，记录排序失败的错误日志
		if app := Lynx(); app != nil {
			log.Errorf("Failed to sort plugins: %v", err)
		}
		return
	}

	// Load plugins in sorted order
	// 按排序后的顺序加载插件
	for _, plugin := range sortedPlugins {
		// 如果插件实例为 nil，跳过当前循环
		if plugin.Plugin == nil {
			continue
		}

		// 初始化插件配置
		runtime := &RuntimePlugin{}
		if err := plugin.Initialize(plugin, runtime); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to initialize plugin %s: %v", plugin.Name(), err)
			}
			return
		}

		// 启动插件，如果启动失败，记录错误日志并返回
		if err := plugin.Start(plugin); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to start plugin %s: %v", plugin.Name(), err)
			}
			return
		}
	}
}

// UnloadPlugins safely unloads all registered plugins.
// UnloadPlugins 安全地卸载所有已注册的插件。
// It handles errors during unloading without interrupting the unload process.
// 它在卸载过程中处理错误，且不会中断卸载流程。
func (m *DefaultLynxPluginManager) UnloadPlugins() {
	// 如果插件管理器为 nil 或者没有已注册的插件，直接返回
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	// 遍历插件列表，依次卸载插件
	for _, plugin := range m.pluginList {
		// 如果插件实例为 nil，跳过当前循环
		if plugin == nil {
			continue
		}
		// 停止插件，如果停止失败，记录错误日志
		if err := plugin.Stop(plugin); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to unload plugin %s: %v", plugin.Name(), err)
			}
		}
	}
}

// LoadPluginsByName loads specific plugins by their names.
// LoadPluginsByName 根据插件名称加载特定的插件。
// Parameters:
//   - names: List of plugin names to load
//   - names: 要加载的插件名称列表
//   - conf: Configuration to use for loading
//   - conf: 用于加载的配置
func (m *DefaultLynxPluginManager) LoadPluginsByName(names []string, conf config.Config) {
	// 如果插件管理器为 nil、名称列表为空或配置为 nil，直接返回
	if m == nil || len(names) == 0 || conf == nil {
		return
	}

	// Collect plugins to load
	// 收集要加载的插件
	var pluginsToLoad []plugins.Plugin
	for _, name := range names {
		// 如果插件存在且不为 nil，添加到待加载列表
		if plugin, exists := m.pluginMap[name]; exists && plugin != nil {
			pluginsToLoad = append(pluginsToLoad, plugin)
		}
	}

	// 如果没有要加载的插件，直接返回
	if len(pluginsToLoad) == 0 {
		return
	}

	// Sort and load plugins
	// 对插件进行排序并加载
	sortedPlugins, err := m.TopologicalSort(pluginsToLoad)
	if err != nil {
		// 如果获取 Lynx 应用实例不为 nil，记录排序失败的错误日志
		if app := Lynx(); app != nil {
			log.Errorf("Failed to sort plugins for loading: %v", err)
		}
		return
	}

	// 按排序后的顺序加载插件
	for _, plugin := range sortedPlugins {
		// 如果插件实例为 nil，跳过当前循环
		if plugin.Plugin == nil {
			continue
		}

		// 初始化插件配置
		runtime := &RuntimePlugin{}
		if err := plugin.Initialize(plugin, runtime); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to initialize plugin %s: %v", plugin.Name(), err)
			}
			return
		}

		// 启动插件，如果启动失败，记录错误日志并返回
		if err := plugin.Start(plugin); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to load plugin %s: %v", plugin.Name(), err)
			}
			return
		}
	}
}

// UnloadPluginsByName safely unloads specific plugins by their names.
// UnloadPluginsByName 安全地卸载指定名称的插件。
// It handles errors during unloading without interrupting the unload process.
// 它在卸载过程中处理错误，且不会中断卸载流程。
func (m *DefaultLynxPluginManager) UnloadPluginsByName(names []string) {
	// 如果插件管理器为 nil 或名称列表为空，直接返回
	if m == nil || len(names) == 0 {
		return
	}

	// 遍历名称列表，卸载对应的插件
	for _, name := range names {
		// 如果插件存在且不为 nil，尝试卸载
		if plugin, exists := m.pluginMap[name]; exists && plugin != nil {
			// 停止插件，如果停止失败，记录错误日志
			if err := plugin.Stop(plugin); err != nil {
				if app := Lynx(); app != nil {
					log.Errorf("Failed to unload plugin %s: %v", name, err)
				}
			}
		}
	}
}

// GetPlugin retrieves a plugin instance by its name.
// GetPlugin 根据插件名称获取插件实例。
// Returns nil if the plugin manager is nil, the name is empty, or the plugin doesn't exist.
// 如果插件管理器为 nil、名称为空或插件不存在，则返回 nil。
func (m *DefaultLynxPluginManager) GetPlugin(name string) plugins.Plugin {
	// 如果插件管理器为 nil 或名称为空，直接返回 nil
	if m == nil || name == "" {
		return nil
	}
	// 从插件映射中获取插件实例
	return m.pluginMap[name]
}
