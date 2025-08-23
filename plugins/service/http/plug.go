package http

import (
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewServiceHttp()
	})
}

// GetHttpServer retrieves the HTTP server instance from the plugin manager.
// This function provides access to the underlying HTTP server for other
// parts of the application that need to register handlers or access
// server functionality.
//
// Returns:
//   - *http.Server: The configured HTTP server instance
//
// Note: This function will panic if the plugin is not properly initialized
// or if the plugin manager cannot find the HTTP plugin.
func GetHttpServer() *http.Server {
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceHttp).server
}
