package rds

import (
	"github.com/go-lynx/lynx/plugin"
)

func Registry(factory plugin.Factory) {
	factory.Register(name, func() plugin.Plugin {
		return Rds()
	})
}
