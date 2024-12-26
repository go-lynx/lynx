package grpc

import (
	"context"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/grpc/conf"
)

var (
	name       = "grpc"
	confPrefix = "lynx.grpc"
)

type ServiceGrpc struct {
	grpc   *grpc.Server
	conf   *conf.Grpc
	weight int
}

type Option func(g *ServiceGrpc)

func Weight(w int) Option {
	return func(g *ServiceGrpc) {
		g.weight = w
	}
}

func Config(c *conf.Grpc) Option {
	return func(g *ServiceGrpc) {
		g.conf = c
	}
}

func (g *ServiceGrpc) Load(b config.Value) (plugin.Plugin, error) {
	// 解析配置到 g.conf 中
	err := b.Scan(g.conf)
	if err != nil {
		return nil, err
	}

	// 打印初始化 gRPC 服务的日志
	app.Lynx().Helper().Infof("Initializing GRPC service")

	// 创建一个切片，用于存储 gRPC 服务器的选项
	opts := []grpc.ServerOption{
		// 使用 tracing 中间件，设置追踪器名称为应用程序名称
		grpc.Middleware(
			tracing.Server(tracing.WithTracerName(app.Name())),
			// 使用 logging 中间件，记录服务器端的日志
			logging.Server(app.Lynx().Logger()),
			// 使用自定义的 gRPC 限流中间件
			app.Lynx().ControlPlane().GrpcRateLimit(),
			// 使用 validate 中间件，对请求进行验证
			validate.Validator(),
			// 异常恢复中间件，用于处理程序崩溃后的恢复
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
		),
	}

	// 如果配置了网络类型，则添加到选项中
	if g.conf.Network != "" {
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	// 如果配置了地址，则添加到选项中
	if g.conf.Addr != "" {
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	// 如果配置了超时时间，则添加到选项中
	if g.conf.Timeout != nil {
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	// 如果配置了 TLS，则加载 TLS 配置并添加到选项中
	if g.conf.GetTls() {
		opts = append(opts, g.tlsLoad())
	}

	// 创建一个新的 gRPC 服务器实例
	g.grpc = grpc.NewServer(opts...)
	// 打印 gRPC 服务初始化成功的日志
	app.Lynx().Helper().Infof("GRPC service successfully initialized")
	return g, nil
}

// Unload 方法用于停止并关闭 gRPC 服务器。
func (g *ServiceGrpc) Unload() error {
	// 检查 gRPC 服务器实例是否存在，如果不存在则直接返回 nil。
	if g.grpc == nil {
		return nil
	}
	// 调用 gRPC 服务器的 Stop 方法来停止服务器，并传入一个 nil 参数。
	// 如果 Stop 方法返回错误，则记录错误信息。
	if err := g.grpc.Stop(nil); err != nil {
		// 使用 app.Lynx().Helper() 记录错误信息。
		app.Lynx().Helper().Error(err)
	}
	// 记录一条信息，指示 gRPC 资源正在被关闭。
	app.Lynx().Helper().Info("message", "Closing the GRPC resources")
	// 返回 nil，表示卸载过程成功，没有发生错误。
	return nil
}

func Grpc(opts ...Option) plugin.Plugin {
	s := &ServiceGrpc{
		weight: 500,
		conf:   &conf.Grpc{},
	}
	for _, option := range opts {
		option(s)
	}
	return s
}
