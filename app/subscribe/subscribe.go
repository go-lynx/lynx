package subscribe

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app/log"
	gGrpc "google.golang.org/grpc"
)

// GrpcSubscribe 表示一个用于订阅 GRPC 服务的结构体。
// 包含服务名称、服务发现实例、是否启用 TLS、根 CA 文件名和文件组等信息。
type GrpcSubscribe struct {
	svcName   string             // 要订阅的 GRPC 服务的名称
	discovery registry.Discovery // 服务发现实例，用于发现服务节点
	tls       bool               // 是否启用 TLS 加密
	caName    string             // 根 CA 证书的文件名
	caGroup   string             // 根 CA 证书文件所属的组
	required  bool               // 是否强依赖的上游服务，启动时会做检查
	// 可选：节点路由器工厂，由上层注入，避免直接依赖 app 包
	routerFactory func(service string) selector.NodeFilter
	// 可选：配置源提供者，由上层注入，用于按 name/group 获取配置源
	configProvider func(name, group string) (config.Source, error)
	// 可选：默认 RootCA 提供者，由上层注入，用于直接获得应用 RootCA
	defaultRootCA func() []byte
}

// Option 定义一个函数类型，用于配置 GrpcSubscribe 实例。
type Option func(o *GrpcSubscribe)

// WithServiceName 返回一个 Option 函数，用于设置要订阅的 GRPC 服务的名称。
func WithServiceName(svcName string) Option {
	return func(o *GrpcSubscribe) {
		o.svcName = svcName
	}
}

// WithDiscovery 返回一个 Option 函数，用于设置服务发现实例。
func WithDiscovery(discovery registry.Discovery) Option {
	return func(o *GrpcSubscribe) {
		o.discovery = discovery
	}
}

// EnableTls 返回一个 Option 函数，用于启用 TLS 加密。
func EnableTls() Option {
	return func(o *GrpcSubscribe) {
		o.tls = true
	}
}

// WithRootCAFileName 返回一个 Option 函数，用于设置根 CA 证书的文件名。
func WithRootCAFileName(caName string) Option {
	return func(o *GrpcSubscribe) {
		o.caName = caName
	}
}

// WithRootCAFileGroup 返回一个 Option 函数，用于设置根 CA 证书文件所属的组。
func WithRootCAFileGroup(caGroup string) Option {
	return func(o *GrpcSubscribe) {
		o.caGroup = caGroup
	}
}

// Required 返回一个 Option 函数，用于设置服务为强依赖的上游服务。
func Required() Option {
	return func(o *GrpcSubscribe) {
		o.required = true
	}
}

// WithNodeRouterFactory 注入节点路由器工厂（可选）
func WithNodeRouterFactory(f func(string) selector.NodeFilter) Option {
	return func(o *GrpcSubscribe) {
		o.routerFactory = f
	}
}

// WithConfigProvider 注入配置源提供者（name, group) -> config.Source
func WithConfigProvider(f func(name, group string) (config.Source, error)) Option {
	return func(o *GrpcSubscribe) {
		o.configProvider = f
	}
}

// WithDefaultRootCA 注入默认 RootCA 提供者
func WithDefaultRootCA(f func() []byte) Option {
	return func(o *GrpcSubscribe) {
		o.defaultRootCA = f
	}
}

// NewGrpcSubscribe 使用提供的选项创建一个新的 GrpcSubscribe 实例。
// 如果没有提供选项，将使用默认配置。
func NewGrpcSubscribe(opts ...Option) *GrpcSubscribe {
	gs := &GrpcSubscribe{
		tls: false, // 默认不启用 TLS 加密
	}
	// 应用提供的选项配置
	for _, o := range opts {
		o(gs)
	}
	return gs
}

// Subscribe 订阅指定的 GRPC 服务，并返回一个 gGrpc.ClientConn 连接实例。
// 如果服务名称为空，则返回 nil。
func (g *GrpcSubscribe) Subscribe() *gGrpc.ClientConn {
	if g.svcName == "" {
		return nil
	}
	// 配置 gRPC 客户端选项
	opts := []grpc.ClientOption{
		grpc.WithEndpoint("discovery:///" + g.svcName), // 设置服务发现的端点
		grpc.WithDiscovery(g.discovery),                // 设置服务发现实例
		grpc.WithMiddleware(
			logging.Client(log.Logger), // 添加日志中间件
			tracing.Client(),           // 添加链路追踪中间件
		),
		grpc.WithTLSConfig(g.tlsLoad()),     // 设置 TLS 配置
		grpc.WithNodeFilter(g.nodeFilter()), // 设置节点过滤器
	}
	var conn *gGrpc.ClientConn
	var err error
	if g.tls {
		// 启用 TLS 时，使用安全连接
		conn, err = grpc.Dial(context.Background(), opts...)
	} else {
		// 未启用 TLS 时，使用非安全连接
		conn, err = grpc.DialInsecure(context.Background(), opts...)
	}
	if err != nil {
		// 记录错误日志并抛出异常
		log.Error(err)
		panic(err)
	}
	return conn
}

// 内部：基于注入的工厂创建节点过滤器
func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if g.routerFactory == nil {
		return nil
	}
	return g.routerFactory(g.svcName)
}
