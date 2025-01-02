package seata

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(name, confPrefix, func() plugins.Plugin {
		return Seata()
	})
}
