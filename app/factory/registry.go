package factory

import (
	"fmt"
	"sync"
	"github.com/go-lynx/lynx/plugins"
)

// Global registry instance
// 全局插件注册表实例，用于实现单例模式
var (
	globalPluginRegistry *PluginRegistry
	once                 sync.Once
)

// GlobalPluginRegistry returns the singleton instance of the plugin registry.
// GlobalPluginRegistry 返回插件注册表的单例实例。
func GlobalPluginRegistry() Registry {
	once.Do(func() {
		globalPluginRegistry = newDefaultPluginRegistry()
	})
	return globalPluginRegistry
}

// PluginRegistry implements the Registry interface.
// PluginRegistry 实现了 Registry 接口。
type PluginRegistry struct {
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

// newDefaultPluginRegistry initializes a new instance of PluginRegistry.
// newDefaultPluginRegistry 初始化一个新的 PluginRegistry 实例。
func newDefaultPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		configToPlugins: make(map[string][]string),
		pluginCreators:  make(map[string]func() plugins.Plugin),
	}
}

// RegisterPlugin registers a new plugin with its configuration prefix and creation function.
// Panics if a plugin with the same name is already registered.
// RegisterPlugin 使用插件的配置前缀和创建函数注册一个新插件。
// 如果同名插件已注册，则触发 panic。
func (r *PluginRegistry) RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin) {
	if _, exists := r.pluginCreators[name]; exists {
		panic(fmt.Errorf("plugin already registered: %s", name))
	}

	r.pluginCreators[name] = creator

	pluginList := r.configToPlugins[configPrefix]
	if pluginList == nil {
		r.configToPlugins[configPrefix] = []string{name}
	} else {
		r.configToPlugins[configPrefix] = append(pluginList, name)
	}
}

// UnregisterPlugin removes a plugin from both the creator map and configuration mapping.
// UnregisterPlugin 从创建函数映射和配置映射中移除一个插件。
func (r *PluginRegistry) UnregisterPlugin(name string) {
	// Remove from creator map
	// 从创建函数映射中移除插件
	delete(r.pluginCreators, name)

	// Remove from configuration mapping
	// 从配置映射中移除插件
	for prefix, pluginList := range r.configToPlugins {
		for i, plugin := range pluginList {
			if plugin == name {
				// Remove the plugin from the slice
				// 从切片中移除插件
				r.configToPlugins[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// If no pluginList left for this prefix, remove the prefix entry
				// 如果该前缀下没有剩余插件，则移除该前缀条目
				if len(r.configToPlugins[prefix]) == 0 {
					delete(r.configToPlugins, prefix)
				}
				break
			}
		}
	}
}

// GetPluginRegistry returns the current mapping of configuration prefixes to plugin names.
// GetPluginRegistry 返回当前配置前缀到插件名称的映射。
func (r *PluginRegistry) GetPluginRegistry() map[string][]string {
	return r.configToPlugins
}

// CreatePlugin creates a new instance of a plugin by its name.
// Returns an error if the plugin is not registered.
// CreatePlugin 根据插件名称创建一个新的插件实例。
// 如果插件未注册，则返回错误。
func (r *PluginRegistry) CreatePlugin(name string) (plugins.Plugin, error) {
	creator, exists := r.pluginCreators[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return creator(), nil
}

// HasPlugin checks if a plugin is registered in the registry.
// HasPlugin 检查插件是否在注册表中注册。
func (r *PluginRegistry) HasPlugin(name string) bool {
	_, exists := r.pluginCreators[name]
	return exists
}
