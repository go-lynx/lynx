package factory

import (
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// TypedFactory 类型安全的插件工厂
type TypedFactory struct {
	// creators 存储插件创建函数
	creators map[string]func() plugins.Plugin
	// configMapping 配置前缀映射
	configMapping map[string][]string
	// mu 读写锁保护并发访问
	mu sync.RWMutex
}

// NewTypedFactory 创建类型安全的插件工厂
func NewTypedFactory() *TypedFactory {
	return &TypedFactory{
		creators:      make(map[string]func() plugins.Plugin),
		configMapping: make(map[string][]string),
	}
}

// RegisterTypedPlugin 注册类型安全的插件
func RegisterTypedPlugin[T plugins.Plugin](
	factory *TypedFactory,
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
func GetTypedPlugin[T plugins.Plugin](factory *TypedFactory, name string) (T, error) {
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
func CreateTypedPlugin[T plugins.Plugin](factory *TypedFactory, name string) (T, error) {
	return GetTypedPlugin[T](factory, name)
}

// HasPlugin 检查插件是否存在
func (f *TypedFactory) HasPlugin(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.creators[name]
	return exists
}

// GetConfigMapping 获取配置映射
func (f *TypedFactory) GetConfigMapping() map[string][]string {
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
func (f *TypedFactory) CreatePlugin(name string) (plugins.Plugin, error) {
	f.mu.RLock()
	creator, exists := f.creators[name]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	return creator(), nil
}

// GetPluginRegistry 获取插件注册表（兼容旧接口）
func (f *TypedFactory) GetPluginRegistry() map[string][]string {
	return f.GetConfigMapping()
}

// UnregisterPlugin 注销插件
func (f *TypedFactory) UnregisterPlugin(name string) {
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
	globalTypedFactory *TypedFactory
	typedFactoryOnce   sync.Once
)

// GlobalTypedFactory 获取全局类型安全的插件工厂
func GlobalTypedFactory() *TypedFactory {
	typedFactoryOnce.Do(func() {
		globalTypedFactory = NewTypedFactory()
	})
	return globalTypedFactory
}
