package http

import (
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugin.Plugin {
		return Http()
	})
}

func GetHTTP() *http.Server {
	return app.Lynx().PlugManager().GetPlugin(name).(*ServiceHttp).http
}
