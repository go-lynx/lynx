package tracer

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init 是 Go 语言的特殊函数，在包被初始化时自动执行，且仅执行一次。
// 此函数的作用是将链路跟踪插件注册到全局插件工厂中。
func init() {
	// 调用全局插件工厂的 RegisterPlugin 方法进行插件注册。
	// 第一个参数 pluginName 是插件的名称，用于唯一标识该插件。
	// 第二个参数 confPrefix 是配置前缀，用于在配置文件中定位该插件的配置项。
	// 第三个参数是一个匿名函数，该函数返回一个实现了 plugins.Plugin 接口的实例。
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// 调用 NewPlugTracer 函数创建一个新的链路跟踪插件实例并返回。
		return NewPlugTracer()
	})
}
