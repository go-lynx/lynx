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

// GetHttpServer 获取 HTTP 服务端实例（安全版本，无 panic）。
// 返回：
//   - *http.Server: 若存在则返回实例，否则为 nil
//   - error: 找不到插件或类型不匹配等错误
func GetHttpServer() (*http.Server, error) {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if service, ok := plugin.(*ServiceHttp); ok && service != nil {
		return service.server, nil
	}
	return nil, fmt.Errorf("failed to get HTTP server: plugin not found or type assertion failed")
}
