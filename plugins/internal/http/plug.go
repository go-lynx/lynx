package http

import (
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewServiceHttp()
	})
}

func GetServer() *http.Server {
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceHttp).server
}
