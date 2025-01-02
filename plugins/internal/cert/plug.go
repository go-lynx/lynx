package cert

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugins.Plugin {
		return Cert()
	})
}
