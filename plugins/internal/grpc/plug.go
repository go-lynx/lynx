package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(name, confPrefix, func() plugins.Plugin {
		return Grpc()
	})
}

func GetServer() *grpc.Server {
	return app.Lynx().PlugManager().GetPlugin(name).(*ServiceGrpc).grpc
}
