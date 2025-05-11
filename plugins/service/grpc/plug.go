package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init 函数用于将 gRPC 服务器插件注册到全局插件工厂中。
// 当该包被导入时，此函数会自动调用。
// 它创建一个新的 ServiceGrpc 实例，并使用配置好的插件名称和配置前缀将其注册到插件工厂。
func init() {
	// 调用全局插件工厂的 RegisterPlugin 方法进行插件注册
	// 传入插件名称、配置前缀和一个返回 plugins.Plugin 接口实例的函数
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// 创建并返回一个新的 ServiceGrpc 实例
		return NewServiceGrpc()
	})
}

// GetGrpcServer 从插件管理器中获取 gRPC 服务器实例。
// 该函数为应用程序的其他部分提供对底层 gRPC 服务器的访问，
// 这些部分可能需要注册服务或使用服务器功能。
//
// 返回值:
//   - *grpc.Server: 配置好的 gRPC 服务器实例
//
// 注意: 如果插件未正确初始化，或者插件管理器找不到 gRPC 插件，此函数会触发 panic。
func GetGrpcServer() *grpc.Server {
	// 从应用程序的插件管理器中获取指定名称的插件，
	// 并将其转换为 *ServiceGrpc 类型，然后返回其 server 字段
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceGrpc).server
}
