// Package factory provides functionality for creating and managing plugins in the Lynx framework.
package factory

import (
	"fmt"

	"github.com/go-lynx/lynx/plugins"
)

// Global factory instance
// 全局插件工厂实例，用于实现单例模式
var (
	globalPluginFactory *LynxPluginFactory
)

// PluginFactory defines the complete interface for plugin management,
// combining both creation and registry capabilities.
// PluginFactory 定义了插件管理的完整接口，结合了插件创建和注册功能。
type PluginFactory interface {
	PluginCreator
	PluginRegistry
}

// PluginCreator defines the interface for creating plugin instances.
// PluginCreator 定义了创建插件实例的接口。
type PluginCreator interface {
	// CreatePlugin instantiates a new plugin instance by its name.
	// Returns an error if the plugin cannot be created.
	// CreatePlugin 根据插件名称实例化一个新的插件实例。
	// 如果插件无法创建，则返回错误。
	CreatePlugin(name string) (plugins.Plugin, error)
}

// PluginRegistry defines the interface for managing plugin registrations.
// PluginRegistry 定义了管理插件注册的接口。
type PluginRegistry interface {
	// RegisterPlugin adds a new plugin to the registry with its configuration prefix
	// and creation function.
	// RegisterPlugin 使用插件的配置前缀和创建函数将新插件添加到注册表中。
	RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin)

	// UnregisterPlugin removes a plugin from the registry.
	// UnregisterPlugin 从注册表中移除一个插件。
	UnregisterPlugin(name string)

	// GetPluginRegistry returns the mapping of configuration prefixes to plugin names.
	// GetPluginRegistry 返回配置前缀到插件名称的映射。
	GetPluginRegistry() map[string][]string

	// HasPlugin checks if a plugin is registered with the given name.
	// HasPlugin 检查具有给定名称的插件是否已注册。
	HasPlugin(name string) bool
}

// GlobalPluginFactory returns the singleton instance of the plugin factory.
// GlobalPluginFactory 返回插件工厂的单例实例。
func GlobalPluginFactory() PluginFactory {
	if globalPluginFactory == nil {
		globalPluginFactory = newDefaultPluginFactory()
	}
	return globalPluginFactory
}

// LynxPluginFactory implements the PluginFactory interface.
// LynxPluginFactory 实现了 PluginFactory 接口。
type LynxPluginFactory struct {
	// configToPlugins maps configuration prefixes to their associated plugin names.
	// Example: "http" -> ["http_server", "http_client"]
	// configToPlugins 将配置前缀映射到关联的插件名称。
	// 示例: "http" -> ["http_server", "http_client"]
	configToPlugins map[string][]string

	// pluginCreators stores the creation functions for each plugin.
	// Maps plugin names to their respective creation functions.
	// pluginCreators 存储每个插件的创建函数。
	// 将插件名称映射到各自的创建函数。
	pluginCreators map[string]func() plugins.Plugin
}

// newDefaultPluginFactory initializes a new instance of LynxPluginFactory.
// newDefaultPluginFactory 初始化一个新的 LynxPluginFactory 实例。
func newDefaultPluginFactory() *LynxPluginFactory {
	return &LynxPluginFactory{
		configToPlugins: make(map[string][]string),
		pluginCreators:  make(map[string]func() plugins.Plugin),
	}
}

// RegisterPlugin registers a new plugin with its configuration prefix and creation function.
// Panics if a plugin with the same name is already registered.
// RegisterPlugin 使用插件的配置前缀和创建函数注册一个新插件。
// 如果同名插件已注册，则触发 panic。
func (f *LynxPluginFactory) RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin) {
	if _, exists := f.pluginCreators[name]; exists {
		panic(fmt.Errorf("plugin already registered: %s", name))
	}

	f.pluginCreators[name] = creator

	pluginList := f.configToPlugins[configPrefix]
	if pluginList == nil {
		f.configToPlugins[configPrefix] = []string{name}
	} else {
		f.configToPlugins[configPrefix] = append(pluginList, name)
	}
}

// UnregisterPlugin removes a plugin from both the creator map and configuration mapping.
// UnregisterPlugin 从创建函数映射和配置映射中移除一个插件。
func (f *LynxPluginFactory) UnregisterPlugin(name string) {
	// Remove from creator map
	// 从创建函数映射中移除插件
	delete(f.pluginCreators, name)

	// Remove from configuration mapping
	// 从配置映射中移除插件
	for prefix, pluginList := range f.configToPlugins {
		for i, plugin := range pluginList {
			if plugin == name {
				// Remove the plugin from the slice
				// 从切片中移除插件
				f.configToPlugins[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// If no pluginList left for this prefix, remove the prefix entry
				// 如果该前缀下没有剩余插件，则移除该前缀条目
				if len(f.configToPlugins[prefix]) == 0 {
					delete(f.configToPlugins, prefix)
				}
				break
			}
		}
	}
}

// GetPluginRegistry returns the current mapping of configuration prefixes to plugin names.
// GetPluginRegistry 返回当前配置前缀到插件名称的映射。
func (f *LynxPluginFactory) GetPluginRegistry() map[string][]string {
	return f.configToPlugins
}

// CreatePlugin creates a new instance of a plugin by its name.
// Returns an error if the plugin is not registered.
// CreatePlugin 根据插件名称创建一个新的插件实例。
// 如果插件未注册，则返回错误。
func (f *LynxPluginFactory) CreatePlugin(name string) (plugins.Plugin, error) {
	creator, exists := f.pluginCreators[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return creator(), nil
}

// HasPlugin checks if a plugin is registered in the factory.
// HasPlugin 检查插件是否在工厂中注册。
func (f *LynxPluginFactory) HasPlugin(name string) bool {
	_, exists := f.pluginCreators[name]
	return exists
}
