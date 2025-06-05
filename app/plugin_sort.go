// Package app provides core functionality for plugin management in the Lynx framework.
package app

import (
	"fmt"
	"sort"

	"github.com/go-lynx/lynx/plugins"
)

// PluginWithLevel represents a plugin with its dependency level in the topology.
// PluginWithLevel 表示一个带有拓扑依赖级别的插件。
// Used internally for dependency sorting and plugin initialization order.
// 内部用于依赖排序和插件初始化顺序。
type PluginWithLevel struct {
	plugins.Plugin
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
	tempResult := make(map[int][]PluginWithLevel) // Intermediate result grouped by level
	visited := make(map[string]bool)
	level := make(map[string]int)
	inProgress := make(map[string]bool) // Track nodes being visited in current DFS path
	// 跟踪当前深度优先搜索路径中正在访问的节点

	var visit func(string) error
	visit = func(name string) error {
		// Check for cyclic dependencies
		// 检查是否存在循环依赖
		if inProgress[name] {
			return fmt.Errorf("cyclic dependency detected for plugin %s", name)
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

		// Set the level and group by level for later sorting
		// 设置级别并按级别分组以供后续排序
		plugin := nameToPlugin[name]
		level[name] = maxLevel + 1
		tempResult[level[name]] = append(tempResult[level[name]], PluginWithLevel{Plugin: plugin, level: level[name]})
		visited[name] = true

		return nil
	}

	// Visit all plugins to build the level-sorted list
	// 访问所有插件以构建按级别排序的列表
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		if err := visit(p.Name()); err != nil {
			return nil, fmt.Errorf("failed to sort plugins: %w", err)
		}
	}

	// Flatten and sort each level group by descending weight
	// 将每个级别的分组按权重降序排序并扁平化结果
	finalResult := make([]PluginWithLevel, 0, len(pluginList))
	levels := make([]int, 0, len(tempResult))
	for lvl := range tempResult {
		levels = append(levels, lvl)
	}
	sort.Ints(levels)

	for _, lvl := range levels {
		pluginsAtLevel := tempResult[lvl]
		sort.SliceStable(pluginsAtLevel, func(i, j int) bool {
			pi := pluginsAtLevel[i].Plugin.Weight()
			pj := pluginsAtLevel[j].Plugin.Weight()
			return pi > pj // Higher weight comes first
		})
		finalResult = append(finalResult, pluginsAtLevel...)
	}

	return finalResult, nil
}
