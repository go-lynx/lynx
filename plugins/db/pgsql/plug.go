package pgsql

import (
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
)

// init 是 Go 语言的初始化函数，在包被加载时自动执行。
// 此函数的作用是将 PgSQL 客户端插件注册到全局插件工厂中。
func init() {
	// 获取全局插件工厂实例，并调用其 RegisterPlugin 方法进行插件注册。
	// 第一个参数 pluginName 是插件的名称，用于唯一标识该插件。
	// 第二个参数 confPrefix 是配置前缀，用于从配置中读取插件相关配置。
	// 第三个参数是一个匿名函数，该函数返回一个 plugins.Plugin 接口类型的实例，
	// 这里调用 NewPgsqlClient 函数创建一个新的 PgSQL 客户端插件实例。
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewPgsqlClient()
	})
}

// GetDriver 函数用于获取 PgSQL 客户端的数据库驱动实例。
// 返回值为 *sql.Driver 类型，即数据库驱动指针。
func GetDriver() *sql.Driver {
	// 从全局 Lynx 应用实例中获取插件管理器，
	// 再通过插件管理器根据插件名称获取对应的插件实例，
	// 最后将获取到的插件实例转换为 *DBPgsqlClient 类型，
	// 并返回其 dri 字段，即数据库驱动实例。
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetDriver()
}

// GetStats 获取连接池统计信息
func GetStats() *ConnectionPoolStats {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetStats()
}

// GetConfig 获取当前配置
func GetConfig() *conf.Pgsql {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetConfig()
}

// IsConnected 检查是否已连接
func IsConnected() bool {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return false
	}
	return plugin.(*DBPgsqlClient).IsConnected()
}

// CheckHealth 执行健康检查
func CheckHealth() error {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return fmt.Errorf("pgsql plugin not found")
	}
	return plugin.(*DBPgsqlClient).CheckHealth()
}
