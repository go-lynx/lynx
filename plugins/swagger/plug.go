package swagger

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	// Register Swagger plugin to global plugin registry
	factory.GlobalPluginRegistry().RegisterPlugin(
		pluginName,
		confPrefix,
		func() plugins.Plugin {
			return NewSwaggerPlugin()
		},
	)
}
