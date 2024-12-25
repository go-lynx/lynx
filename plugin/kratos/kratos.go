package kratos

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// NewKratos 函数用于创建一个新的 Kratos 应用实例
func NewKratos(logger log.Logger, gs *grpc.Server, hs *http.Server, r registry.Registrar) *kratos.App {
	// 使用 kratos.New 函数创建一个新的 Kratos 应用实例
	return kratos.New(
		// 设置应用实例的 ID 为当前应用的主机名
		kratos.ID(app.Host()),
		// 设置应用实例的名称为当前应用的名称
		kratos.Name(app.Name()),
		// 设置应用实例的版本为当前应用的版本
		kratos.Version(app.Version()),
		// 设置应用实例的元数据为空
		kratos.Metadata(map[string]string{}),
		// 设置应用实例的日志记录器为传入的 logger
		kratos.Logger(logger),
		// 设置应用实例的服务器为传入的 grpc 服务器和 http 服务器
		kratos.Server(
			gs,
			hs,
		),
		// 设置应用实例的注册器为传入的注册器
		kratos.Registrar(r),
	)
}

// NewGrpcKratos Start kratos application
func NewGrpcKratos(logger log.Logger, gs *grpc.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(app.Host()),
		kratos.Name(app.Name()),
		kratos.Version(app.Version()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
		),
		kratos.Registrar(r),
	)
}

// NewHttpKratos Start kratos application
func NewHttpKratos(logger log.Logger, hs *http.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(app.Host()),
		kratos.Name(app.Name()),
		kratos.Version(app.Version()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			hs,
		),
		kratos.Registrar(r),
	)
}
