package redis

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/redis/go-redis/v9"
)

// init 函数是 Go 语言的特殊函数，在包被加载时自动执行。
// 此函数的作用是将 Redis 客户端插件注册到全局插件工厂中。
func init() {
	// 调用全局插件工厂的 RegisterPlugin 方法进行插件注册。
	// 第一个参数 pluginName 为插件的唯一名称，用于标识该插件。
	// 第二个参数 confPrefix 是配置前缀，用于从配置中读取该插件相关的配置信息。
	// 第三个参数是一个匿名函数，该函数返回一个 plugins.Plugin 接口类型的实例，
	// 通过调用 NewRedisClient 函数创建一个新的 Redis 客户端插件实例。
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewRedisClient()
	})
}

// GetRedis 函数用于获取 Redis 客户端实例。
// 它通过全局 Lynx 应用实例获取插件管理器，再根据插件名称获取对应的插件实例，
// 最后将插件实例转换为 *PlugRedis 类型并返回其 rdb 字段，即 Redis 客户端。
func GetRedis() *redis.Client {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugRedis).rdb
}
