package redis

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/redis/go-redis/v9"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewRedisClient()
	})
}

func GetRedis() *redis.Client {
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*PlugRedis).rdb
}
