package plugin

import (
	"github.com/go-kratos/kratos/v2/config"
)

// Plugin 接口定义了插件的加载和卸载方法
type Plugin interface {
	// Load 方法用于加载插件，接收一个配置对象作为参数，并返回插件实例和错误信息
	Load(config.Value) (Plugin, error)
	// Unload 方法用于卸载插件，并返回错误信息
	Unload() error
	// SupportPlugin 接口嵌入到 Plugin 接口中，用于提供插件的支持功能
	SupportPlugin
}

// SupportPlugin 接口定义了插件的支持功能，包括插件名称、权重、依赖项和配置前缀
type SupportPlugin interface {
	// Name 方法返回插件的名称
	Name() string
	// Weight 方法返回插件的权重
	Weight() int
	// DependsOn 方法返回插件的依赖项列表
	DependsOn(config.Value) []string
	// ConfPrefix 方法返回插件的配置前缀
	ConfPrefix() string
}
