package db

import (
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	plugin.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Cert()
	})
}
