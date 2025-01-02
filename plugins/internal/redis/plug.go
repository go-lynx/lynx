package redis

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/redis/go-redis/v9"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugins.Plugin {
		return Redis()
	})
}

func GetRedis() *redis.Client {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugRedis).rdb
}
