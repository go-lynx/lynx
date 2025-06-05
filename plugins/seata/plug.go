package seata

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init 是 Go 语言的初始化函数，在包被加载时自动执行。
// 此函数的作用是将 Seata 客户端插件注册到全局插件工厂中。
func init() {
	// 调用全局插件工厂实例的 RegisterPlugin 方法进行插件注册。
	// 第一个参数 pluginName 是插件的唯一名称，用于在系统中标识该插件。
	// 第二个参数 confPrefix 是配置前缀，用于从配置文件中读取该插件的相关配置。
	// 第三个参数是一个匿名函数，该函数返回一个实现了 plugins.Plugin 接口的实例。
	// 通过调用 NewSeataClient 函数创建一个新的 Seata 客户端插件实例并返回。
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewTxSeataClient()
	})
}
