package redis

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/redis/go-redis/v9"
)

func init() {
	plugin.GlobalPluginFactory().Register(plugName, func() plugin.Plugin {
		return Redis()
	})
}

func GetRedis() *redis.Client {
	return app.Lynx().PlugManager().GetPlugin(plugName).(*PlugRedis).rdb
}
