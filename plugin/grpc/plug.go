package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugin.Plugin {
		return Grpc()
	})
}

func GetServer() *grpc.Server {
	return app.Lynx().PlugManager().GetPlugin(name).(*ServiceGrpc).grpc
}
