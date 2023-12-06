package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	plugin.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Polaris()
	})
}

func GetPolaris() *polaris.Polaris {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugPolaris).polaris
}

func GetPlugPolaris() *PlugPolaris {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugPolaris)
}
