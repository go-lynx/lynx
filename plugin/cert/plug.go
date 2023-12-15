package db

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	factory.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Cert()
	})
}
