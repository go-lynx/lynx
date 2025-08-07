package polaris

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init 函数会在包被导入时自动执行，用于将 Polaris 插件注册到全局插件工厂。
// 注册后，插件管理器可以发现并加载该插件。
func init() {
	// 从全局插件工厂获取实例，调用其 RegisterPlugin 方法进行插件注册。
	// 参数 name 为插件的名称，用于唯一标识插件。
	// 参数 confPrefix 是插件配置的前缀，用于从配置文件中加载插件相关配置。
	// 最后一个参数是一个函数，返回一个实现了 plugins.Plugin 接口的实例。
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// 调用 Polaris 函数获取插件实例并返回
		return NewPolarisControlPlane()
	})
}
