package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(name, confPrefix, func() plugins.Plugin {
		return Polaris()
	})
}

func GetPolaris() *polaris.Polaris {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugPolaris).polaris
}

func GetPlugin() *PlugPolaris {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugPolaris)
}
