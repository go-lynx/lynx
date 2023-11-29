package redis

import (
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	plugin.GlobalPluginFactory().Register(plugName, func() plugin.Plugin {
		return Redis()
	})
}
