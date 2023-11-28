package redis

import (
	"github.com/go-lynx/lynx/plugin"
)

func Registry(factory plugin.Factory) {
	factory.Register(plugName, func() plugin.Plugin {
		return Redis()
	})
}
