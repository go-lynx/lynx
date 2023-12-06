package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	plugin.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Grpc()
	})
}

func GetGRPC() *grpc.Server {
	return app.Lynx().PlugManager().GetPlugin(name).(*ServiceGrpc).grpc
}
