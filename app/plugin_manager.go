// Package app provides core functionality for plugin management in the Lynx framework.
// 包 app 为 Lynx 框架提供插件管理的核心功能。
// It includes interfaces and implementations for managing plugin lifecycle,
// dependencies, and configuration.
// 它包含用于管理插件生命周期、依赖关系和配置的接口与实现。
package app

import (
	"fmt"
	"sort"
	"sync"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/app/log"
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

// PluginManager 统一的插件管理器接口
type PluginManager interface {
	// 基本插件管理
	LoadPlugins(config.Config) error
	UnloadPlugins()
	LoadPluginsByName(config.Config, []string) error
	UnloadPluginsByName([]string)
	GetPlugin(name string) plugins.Plugin
	PreparePlug(config config.Config) ([]plugins.Plugin, error)
	// Runtime 管理
	GetRuntime() plugins.Runtime
	SetConfig(config.Config)
	// 资源管理
	StopPlugin(pluginID string) error
	GetResourceStats() map[string]any
	ListResources() []*plugins.ResourceInfo
}

// DefaultPluginManager 统一的插件管理器实现
type DefaultPluginManager struct {
	// pluginInstances 存储已创建的插件实例
	pluginInstances sync.Map
	// pluginList 有序插件列表
	pluginList []plugins.Plugin
	// factory 泛型工厂
	factory *factory.TypedPluginFactory
	// mu 保护 pluginList
	mu sync.RWMutex
	// runtime 统一的运行时环境
	runtime plugins.Runtime
	// config 全局配置
	config config.Config
}

// NewPluginManager 创建统一的插件管理器
func NewPluginManager(pluginList ...plugins.Plugin) PluginManager {
	manager := &DefaultPluginManager{
		pluginList: make([]plugins.Plugin, 0),
		factory:    factory.GlobalTypedPluginFactory(),
		runtime:    plugins.NewTypedRuntime(),
	}

	// 注册初始插件 - 修复并发安全问题
	for _, plugin := range pluginList {
		if plugin != nil {
			// 使用原子操作确保数据一致性
			manager.pluginInstances.Store(plugin.Name(), plugin)

			// 使用写锁保护pluginList的修改
			manager.mu.Lock()
			manager.pluginList = append(manager.pluginList, plugin)
			manager.mu.Unlock()
		}
	}

	return manager
}

// SetConfig 设置全局配置
func (m *DefaultPluginManager) SetConfig(conf config.Config) {
	m.config = conf
	// 更新 runtime 的配置
	if typedRuntime, ok := m.runtime.(*plugins.TypedRuntimeImpl); ok {
		typedRuntime.SetConfig(conf)
	}
}

// GetRuntime 获取统一的运行时环境
func (m *DefaultPluginManager) GetRuntime() plugins.Runtime {
	return m.runtime
}

// GetTypedPluginFromManager 获取类型安全的插件（独立函数）
func GetTypedPluginFromManager[T plugins.Plugin](m *DefaultPluginManager, name string) (T, error) {
	var zero T

	if value, ok := m.pluginInstances.Load(name); ok {
		if typed, ok := value.(T); ok {
			return typed, nil
		}
		return zero, fmt.Errorf("plugin %s is not of expected type", name)
	}

	return zero, fmt.Errorf("plugin %s not found", name)
}

// RegisterTypedPlugin 注册类型安全的插件到管理器（独立函数）
func RegisterTypedPlugin[T plugins.Plugin](
	m *DefaultPluginManager,
	name string,
	configPrefix string,
	creator func() T,
) {
	factory.RegisterTypedPlugin(m.factory, name, configPrefix, creator)
}

// GetPlugin 获取插件（兼容旧接口）
func (m *DefaultPluginManager) GetPlugin(name string) plugins.Plugin {
	if value, ok := m.pluginInstances.Load(name); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	return nil
}

// LoadPlugins 加载插件 - 修复并发安全问题
func (m *DefaultPluginManager) LoadPlugins(conf config.Config) error {
	m.SetConfig(conf)

	// 准备插件配置
	plugins, err := m.PreparePlug(conf)
	if err != nil {
		return fmt.Errorf("failed to prepare plugins: %w", err)
	}
	if len(plugins) == 0 {
		return fmt.Errorf("no plugins prepared")
	}

	// 按依赖关系排序插件
	sortedPlugins, err := m.TopologicalSort(plugins)
	if err != nil {
		return fmt.Errorf("failed to sort plugins: %w", err)
	}

	// 加载插件
	for _, pluginWithLevel := range sortedPlugins {
		plugin := pluginWithLevel.Plugin
		// 为每个插件创建带上下文的运行时
		pluginRuntime := m.runtime.WithPluginContext(plugin.ID())

		// 初始化插件
		if err := plugin.Initialize(plugin, pluginRuntime); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", plugin.ID(), err)
		}

		// 启动插件
		if err := plugin.Start(plugin); err != nil {
			// 如果启动失败，清理资源
			m.runtime.CleanupResources(plugin.ID())
			return fmt.Errorf("failed to start plugin %s: %w", plugin.ID(), err)
		}

		// 使用sync.Map的Store方法而不是直接索引
		m.pluginInstances.Store(plugin.ID(), plugin)
	}

	return nil
}

// UnloadPlugins 卸载所有插件
func (m *DefaultPluginManager) UnloadPlugins() {
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	for _, plugin := range m.pluginList {
		if plugin == nil {
			continue
		}
		if err := plugin.Stop(plugin); err != nil {
			if app := Lynx(); app != nil {
				log.Errorf("Failed to unload plugin %s: %v", plugin.Name(), err)
			}
		}
	}
}

// LoadPluginsByName 按名称加载插件
func (m *DefaultPluginManager) LoadPluginsByName(conf config.Config, pluginNames []string) error {
	m.SetConfig(conf)

	// 准备插件配置
	plugins, err := m.PreparePlug(conf)
	if err != nil {
		return err
	}

	// 过滤指定名称的插件
	var targetPlugins []plugins.Plugin
	pluginMap := make(map[string]plugins.Plugin)
	for _, plugin := range plugins {
		pluginMap[plugin.ID()] = plugin
	}

	for _, name := range pluginNames {
		if plugin, exists := pluginMap[name]; exists {
			targetPlugins = append(targetPlugins, plugin)
		} else {
			return fmt.Errorf("plugin %s not found", name)
		}
	}

	// 按依赖关系排序插件
	sortedPlugins, err := m.TopologicalSort(targetPlugins)
	if err != nil {
		return fmt.Errorf("failed to sort plugins: %w", err)
	}

	// 加载插件
	for _, plugin := range sortedPlugins {
		// 为每个插件创建带上下文的运行时
		pluginRuntime := m.runtime.WithPluginContext(plugin.ID())

		// 初始化插件
		if err := plugin.Initialize(plugin, pluginRuntime); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", plugin.ID(), err)
		}

		// 启动插件
		if err := plugin.Start(plugin); err != nil {
			// 如果启动失败，清理资源
			m.runtime.CleanupResources(plugin.ID())
			return fmt.Errorf("failed to start plugin %s: %w", plugin.ID(), err)
		}

		// 使用sync.Map的Store方法而不是直接索引
		m.pluginInstances.Store(plugin.ID(), plugin)
	}

	return nil
}

// UnloadPluginsByName 按名称卸载插件
func (m *DefaultPluginManager) UnloadPluginsByName(names []string) {
	if m == nil || len(names) == 0 {
		return
	}

	for _, name := range names {
		if pluginObj, ok := m.pluginInstances.Load(name); ok {
			if plugin, ok := pluginObj.(plugins.Plugin); ok && plugin != nil {
				if err := plugin.Stop(plugin); err != nil {
					if app := Lynx(); app != nil {
						log.Errorf("Failed to unload plugin %s: %v", name, err)
					}
				}
				m.pluginInstances.Delete(name)
			}
		}
	}

	// 更新插件列表
	m.mu.Lock()
	defer m.mu.Unlock()

	newList := make([]plugins.Plugin, 0, len(m.pluginList))
	for _, p := range m.pluginList {
		if !containsName(names, p.Name()) {
			newList = append(newList, p)
		}
	}
	m.pluginList = newList
}

// TopologicalSort 对插件进行拓扑排序
func (m *DefaultPluginManager) TopologicalSort(pluginList []plugins.Plugin) ([]PluginWithLevel, error) {
	// 构建一个从插件名称到实际插件实例的映射
	nameToPlugin := make(map[string]plugins.Plugin)
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		nameToPlugin[p.Name()] = p
	}

	// 以邻接表的形式构建依赖图
	graph := make(map[string][]string)
	for _, p := range pluginList {
		if p == nil {
			continue
		}

		// 检查插件是否实现了 DependencyAware 接口
		depAware, ok := p.(plugins.DependencyAware)
		if !ok {
			// 插件未实现 DependencyAware 接口，将其视为没有依赖
			continue
		}

		// 使用 DependencyAware 接口获取插件的依赖
		dependencies := depAware.GetDependencies()
		if dependencies == nil {
			continue
		}

		// 验证依赖并将其添加到图中
		for _, dep := range dependencies {
			// 验证依赖对象
			if dep.ID == "" {
				return nil, fmt.Errorf("plugin %s has an invalid dependency with empty ID", p.Name())
			}

			// 检查依赖是否存在
			if _, exists := nameToPlugin[dep.ID]; !exists {
				if dep.Required {
					return nil, fmt.Errorf("plugin %s requires missing plugin %s", p.Name(), dep.ID)
				}
				// 跳过不可用的可选依赖
				continue
			}

			// 添加依赖关系到图中
			graph[p.Name()] = append(graph[p.Name()], dep.ID)
		}
	}

	// 使用深度优先搜索进行拓扑排序
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	levels := make(map[string]int)
	var result []PluginWithLevel

	var dfs func(string) error
	dfs = func(node string) error {
		if temp[node] {
			return fmt.Errorf("cyclic dependency detected involving plugin %s", node)
		}
		if visited[node] {
			return nil
		}

		temp[node] = true
		defer func() { temp[node] = false }()

		// 计算当前节点的依赖级别
		maxLevel := -1
		for _, dep := range graph[node] {
			if err := dfs(dep); err != nil {
				return err
			}
			if levels[dep] > maxLevel {
				maxLevel = levels[dep]
			}
		}

		levels[node] = maxLevel + 1
		visited[node] = true

		// 将插件添加到结果中
		if plugin, exists := nameToPlugin[node]; exists {
			result = append(result, PluginWithLevel{
				Plugin: plugin,
				level:  levels[node],
			})
		}

		return nil
	}

	// 对所有插件执行深度优先搜索
	for _, p := range pluginList {
		if p == nil {
			continue
		}
		if err := dfs(p.Name()); err != nil {
			return nil, err
		}
	}

	// 按级别排序结果
	sort.Slice(result, func(i, j int) bool {
		return result[i].level < result[j].level
	})

	return result, nil
}

// containsName checks if a name exists in the string slice.
// containsName 检查一个名称是否存在于字符串切片中。
func containsName(slice []string, name string) bool {
	for _, item := range slice {
		if item == name {
			return true
		}
	}
	return false
}

// LynxPluginManager 保持向后兼容的插件管理器接口
type LynxPluginManager = PluginManager

// DefaultLynxPluginManager 保持向后兼容的插件管理器实现
type DefaultLynxPluginManager = DefaultPluginManager

// NewLynxPluginManager 创建插件管理器（向后兼容）
func NewLynxPluginManager(pluginList ...plugins.Plugin) LynxPluginManager {
	return NewPluginManager(pluginList...)
}

// TypedPluginManager 保持向后兼容的泛型插件管理器接口
type TypedPluginManager = PluginManager

// DefaultTypedPluginManager 保持向后兼容的泛型插件管理器实现
type DefaultTypedPluginManager = DefaultPluginManager

// NewTypedPluginManager 创建泛型插件管理器（向后兼容）
func NewTypedPluginManager(pluginList ...plugins.Plugin) TypedPluginManager {
	return NewPluginManager(pluginList...)
}

// RegisterTypedPluginToManager 保持向后兼容的泛型函数
func RegisterTypedPluginToManager[T plugins.Plugin](
	m *DefaultTypedPluginManager,
	name string,
	configPrefix string,
	creator func() T,
) {
	RegisterTypedPlugin(m, name, configPrefix, creator)
}

// StopPlugin 停止指定插件
func (m *DefaultPluginManager) StopPlugin(pluginID string) error {
	plugin, exists := m.pluginInstances.Load(pluginID)
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// 停止插件
	if err := plugin.(plugins.Plugin).Stop(plugin.(plugins.Plugin)); err != nil {
		return fmt.Errorf("failed to stop plugin %s: %w", pluginID, err)
	}

	// 清理插件资源
	if err := m.runtime.CleanupResources(pluginID); err != nil {
		return fmt.Errorf("failed to cleanup resources for plugin %s: %w", pluginID, err)
	}

	m.pluginInstances.Delete(pluginID)
	return nil
}

// GetResourceStats 获取资源统计信息
func (m *DefaultPluginManager) GetResourceStats() map[string]any {
	return m.runtime.GetResourceStats()
}

// ListResources 列出所有资源
func (m *DefaultPluginManager) ListResources() []*plugins.ResourceInfo {
	return m.runtime.ListResources()
}
