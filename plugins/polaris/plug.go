package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
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

// GetPolaris 函数用于从应用的插件管理器中获取 Polaris 实例。
// 该实例可用于与 Polaris 服务进行交互，如服务发现、配置管理等。
// 返回值为 *polaris.Polaris 类型的指针，指向获取到的 Polaris 实例。
func GetPolaris() *polaris.Polaris {
	// 从应用的插件管理器中获取指定名称的插件实例，
	// 并将其类型断言为 *PlugPolaris，然后返回其内部的 polaris 字段。
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*PlugPolaris).polaris
}

// GetPlugin 函数用于从应用的插件管理器中获取 PlugPolaris 插件实例。
// 该实例可用于调用插件提供的各种方法。
// 返回值为 *PlugPolaris 类型的指针，指向获取到的插件实例。
func GetPlugin() *PlugPolaris {
	// 从应用的插件管理器中获取指定名称的插件实例，
	// 并将其类型断言为 *PlugPolaris 后返回。
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugPolaris)
}
