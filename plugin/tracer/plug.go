package tracer

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugin.Plugin {
		return Tracer()
	})
}
