// Package factory provides functionality for creating and managing plugins in the Lynx framework.
package factory

import (
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
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

// TypedPluginFactory 泛型插件工厂
type TypedPluginFactory struct {
	// creators 存储插件创建函数
	creators map[string]func() plugins.Plugin
	// configMapping 配置前缀映射
	configMapping map[string][]string
	// mu 读写锁保护并发访问
	mu sync.RWMutex
}

// NewTypedPluginFactory 创建泛型插件工厂
func NewTypedPluginFactory() *TypedPluginFactory {
	return &TypedPluginFactory{
		creators:      make(map[string]func() plugins.Plugin),
		configMapping: make(map[string][]string),
	}
}

// RegisterTypedPlugin 注册泛型插件
func RegisterTypedPlugin[T plugins.Plugin](
	factory *TypedPluginFactory,
	name string,
	configPrefix string,
	creator func() T,
) {
	factory.mu.Lock()
	defer factory.mu.Unlock()

	// 检查是否已经注册
	if _, exists := factory.creators[name]; exists {
		panic(fmt.Errorf("plugin already registered: %s", name))
	}

	// 跨前缀重复名检测（防御性）：如果该 name 已经出现在其他前缀的映射中，记录告警
	for prefix, names := range factory.configMapping {
		if prefix == configPrefix {
			continue
		}
		for _, n := range names {
			if n == name {
				log.Warnf("plugin name '%s' registered under multiple prefixes: existing='%s', new='%s'", name, prefix, configPrefix)
			}
		}
	}

	// 存储创建函数
	factory.creators[name] = func() plugins.Plugin {
		return creator()
	}

	// 配置映射
	if factory.configMapping[configPrefix] == nil {
		factory.configMapping[configPrefix] = make([]string, 0)
	}
	// 前缀内去重，避免重复追加
	exists := false
	for _, n := range factory.configMapping[configPrefix] {
		if n == name {
			exists = true
			break
		}
	}
	if !exists {
		factory.configMapping[configPrefix] = append(factory.configMapping[configPrefix], name)
	}
}

// GetTypedPlugin 获取类型安全的插件实例
func GetTypedPlugin[T plugins.Plugin](factory *TypedPluginFactory, name string) (T, error) {
	var zero T

	factory.mu.RLock()
	creator, exists := factory.creators[name]
	factory.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("plugin %s not found", name)
	}

	plugin := creator()
	typed, ok := plugin.(T)
	if !ok {
		return zero, fmt.Errorf("plugin %s failed type assertion", name)
	}

	return typed, nil
}

// CreateTypedPlugin 创建类型安全的插件实例
func CreateTypedPlugin[T plugins.Plugin](factory *TypedPluginFactory, name string) (T, error) {
	return GetTypedPlugin[T](factory, name)
}

// HasPlugin 检查插件是否存在
func (f *TypedPluginFactory) HasPlugin(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.creators[name]
	return exists
}

// GetConfigMapping 获取配置映射
func (f *TypedPluginFactory) GetConfigMapping() map[string][]string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// 创建副本以避免并发修改
	result := make(map[string][]string)
	for k, v := range f.configMapping {
		result[k] = make([]string, len(v))
		copy(result[k], v)
	}
	return result
}

// CreatePlugin 创建插件实例（兼容旧接口）
func (f *TypedPluginFactory) CreatePlugin(name string) (plugins.Plugin, error) {
	f.mu.RLock()
	creator, exists := f.creators[name]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	return creator(), nil
}

// GetPluginRegistry 获取插件注册表（兼容旧接口）
func (f *TypedPluginFactory) GetPluginRegistry() map[string][]string {
	return f.GetConfigMapping()
}

// UnregisterPlugin 注销插件
func (f *TypedPluginFactory) UnregisterPlugin(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 从创建器映射中删除
	delete(f.creators, name)

	// 从配置映射中删除
	for prefix, pluginList := range f.configMapping {
		for i, plugin := range pluginList {
			if plugin == name {
				// 移除插件
				f.configMapping[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// 如果该前缀下没有插件了，删除前缀
				if len(f.configMapping[prefix]) == 0 {
					delete(f.configMapping, prefix)
				}
				break
			}
		}
	}
}

// Global typed factory instance
var (
	globalTypedFactory *TypedPluginFactory
	typedFactoryOnce   sync.Once
)

// GlobalTypedPluginFactory 获取全局泛型插件工厂
func GlobalTypedPluginFactory() *TypedPluginFactory {
	typedFactoryOnce.Do(func() {
		globalTypedFactory = NewTypedPluginFactory()
	})
	return globalTypedFactory
}
