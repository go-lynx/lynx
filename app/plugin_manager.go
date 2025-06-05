// Package app provides core functionality for plugin management in the Lynx framework.
// 包 app 为 Lynx 框架提供插件管理的核心功能。
// It includes interfaces and implementations for managing plugin lifecycle,
// dependencies, and configuration.
// 它包含用于管理插件生命周期、依赖关系和配置的接口与实现。
package app

import (
	"sync"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/app/log"
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
	pluginMap sync.Map
	// pluginList maintains the ordered list of plugins
	// pluginList 维护一个有序的插件列表。
	pluginList []plugins.Plugin
	// mu protects the pluginList
	// mu 保护 pluginList
	mu sync.RWMutex
	// factory is used to create plugin instances
	// factory 用于创建插件实例。
	factory factory.PluginFactory
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
	// 创建默认插件管理器实例
	manager := &DefaultLynxPluginManager{
		pluginList: make([]plugins.Plugin, 0),
		factory:    factory.GlobalPluginFactory(),
	}

	// 如果提供了初始插件列表，注册这些插件
	for _, plugin := range pluginList {
		if plugin == nil {
			continue
		}
		// 将插件添加到映射和列表中
		manager.pluginMap.Store(plugin.Name(), plugin)
		manager.mu.Lock()
		manager.pluginList = append(manager.pluginList, plugin)
		manager.mu.Unlock()
	}
	return manager
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
		if plugin.Status(plugin) == plugins.StatusInactive {
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
		if pluginObj, ok := m.pluginMap.Load(name); ok {
			if plugin, ok := pluginObj.(plugins.Plugin); ok && plugin != nil {
				pluginsToLoad = append(pluginsToLoad, plugin)
			}
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
		if pluginObj, ok := m.pluginMap.Load(name); ok {
			if plugin, ok := pluginObj.(plugins.Plugin); ok && plugin != nil {
				// 停止插件，如果停止失败，记录错误日志
				if err := plugin.Stop(plugin); err != nil {
					if app := Lynx(); app != nil {
						log.Errorf("Failed to unload plugin %s: %v", name, err)
					}
				}
				// 从 map 中删除插件
				m.pluginMap.Delete(name)
			}
		}
	}

	// 更新 pluginList
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 创建新的插件列表，不包含已卸载的插件
	newList := make([]plugins.Plugin, 0, len(m.pluginList))
	for _, p := range m.pluginList {
		if !containsName(names, p.Name()) {
			newList = append(newList, p)
		}
	}
	m.pluginList = newList
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
	if plugin, ok := m.pluginMap.Load(name); ok {
		return plugin.(plugins.Plugin)
	}
	return nil
}
