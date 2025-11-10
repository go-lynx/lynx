package http

import (
	"fmt"
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

// GetHttpServer retrieves the HTTP server instance (safe version, no panic).
// Returns:
//   - *http.Server: the instance if present, otherwise nil
//   - error: when plugin is not found or type assertion fails
func GetHttpServer() (*http.Server, error) {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if service, ok := plugin.(*ServiceHttp); ok && service != nil {
		return service.server, nil
	}
	return nil, fmt.Errorf("failed to get HTTP server: plugin not found or type assertion failed")
}
