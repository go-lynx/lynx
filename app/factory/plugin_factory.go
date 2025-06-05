// Package factory provides functionality for creating and managing plugins in the Lynx framework.
package factory

import (
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
