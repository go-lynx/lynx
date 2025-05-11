// Package grpc provides a gRPC server plugin for the Lynx framework.
// It implements the necessary interfaces to integrate with the Lynx plugin system
// and provides functionality for setting up and managing a gRPC server with various
// middleware options and TLS support.
package grpc

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugins/service/grpc/v2/conf"
)

// Plugin metadata constants define the basic information about the gRPC plugin
// 插件元数据常量，定义了 gRPC 插件的基本信息
const (
	// pluginName is the unique identifier for the gRPC server plugin
	// pluginName 是 gRPC 服务器插件的唯一标识符
	pluginName = "grpc.server"

	// pluginVersion indicates the current version of the plugin
	// pluginVersion 表示插件的当前版本
	pluginVersion = "v2.0.0"

	// pluginDescription provides a brief description of the plugin's functionality
	// pluginDescription 简要描述了插件的功能
	pluginDescription = "grpc server plugin for lynx framework"

	// confPrefix is the configuration prefix used for loading gRPC settings
	// confPrefix 是用于加载 gRPC 配置的前缀
	confPrefix = "lynx.grpc"
)

// ServiceGrpc represents the gRPC server plugin implementation.
// It embeds the BasePlugin for common plugin functionality and maintains
// the gRPC server instance along with its configuration.
// ServiceGrpc 表示 gRPC 服务器插件的实现。
// 它嵌入了 BasePlugin 以获得通用的插件功能，并维护 gRPC 服务器实例及其配置。
type ServiceGrpc struct {
	// 嵌入 Lynx 框架的基础插件，继承通用的插件功能
	*plugins.BasePlugin
	// gRPC 服务器实例
	server *grpc.Server
	// gRPC 服务器的配置信息
	conf *conf.Grpc
}

// NewServiceGrpc creates and initializes a new instance of the gRPC server plugin.
// It sets up the base plugin with the appropriate metadata and returns a pointer
// to the ServiceGrpc structure.
// NewServiceGrpc 创建并初始化一个新的 gRPC 服务器插件实例。
// 它使用适当的元数据设置基础插件，并返回一个指向 ServiceGrpc 结构体的指针。
func NewServiceGrpc() *ServiceGrpc {
	return &ServiceGrpc{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件的唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
		),
		conf: &conf.Grpc{},
	}
}

// InitializeResources implements the plugin initialization interface.
// It loads and validates the gRPC server configuration from the runtime environment.
// If no configuration is provided, it sets up default values for the server.
// InitializeResources 实现了插件初始化接口。
// 它从运行时环境中加载并验证 gRPC 服务器配置。
// 如果未提供配置，则为服务器设置默认值。
func (g *ServiceGrpc) InitializeResources(rt plugins.Runtime) error {
	// Add default configuration if not provided
	// 如果未提供配置，则添加默认配置
	if g.conf == nil {
		g.conf = &conf.Grpc{
			// 默认网络协议为 TCP
			Network: "tcp",
			// 默认监听地址为 :9090
			Addr: ":9090",
		}
	}
	// 从运行时配置中扫描并加载 gRPC 配置
	err := rt.GetConfig().Value(confPrefix).Scan(g.conf)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks implements the plugin startup interface.
// It configures and starts the gRPC server with all necessary middleware and options,
// including tracing, logging, rate limiting, validation, and recovery handlers.
// StartupTasks 实现了插件启动接口。
// 它使用所有必要的中间件和选项配置并启动 gRPC 服务器，
// 包括链路追踪、日志记录、限流、参数验证和恢复处理。
func (g *ServiceGrpc) StartupTasks() error {
	// 记录 gRPC 服务启动日志
	log.Infof("starting grpc service")

	// 定义 gRPC 服务器的选项列表
	opts := []grpc.ServerOption{
		// 配置 gRPC 服务器的中间件
		grpc.Middleware(
			// 配置链路追踪中间件，设置追踪器名称为应用名称
			tracing.Server(tracing.WithTracerName(app.GetName())),
			// 配置日志中间件，使用 Lynx 框架的日志记录器
			logging.Server(log.Logger),
			// 配置限流中间件，使用 Lynx 框架控制平面的 gRPC 限流策略
			app.Lynx().GetControlPlane().GRPCRateLimit(),
			// 配置参数验证中间件，注意：该方法已弃用
			validate.Validator(),
			// 配置恢复中间件，处理请求处理过程中的 panic
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
		),
	}

	// Configure server options based on configuration
	// 根据配置信息配置服务器选项
	if g.conf.Network != "" {
		// 设置网络协议
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	if g.conf.Addr != "" {
		// 设置监听地址
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	if g.conf.Timeout != nil {
		// 设置超时时间
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	if g.conf.GetTls() {
		// 如果启用 TLS，添加 TLS 配置选项
		opts = append(opts, g.tlsLoad())
	}

	// 创建 gRPC 服务器实例
	g.server = grpc.NewServer(opts...)
	// 记录 gRPC 服务启动成功日志
	log.Infof("grpc service successfully started")
	return nil
}

// CleanupTasks implements the plugin cleanup interface.
// It gracefully stops the gRPC server and performs necessary cleanup operations.
// If the server is nil or already stopped, it will return nil.
// CleanupTasks 实现了插件清理接口。
// 它优雅地停止 gRPC 服务器并执行必要的清理操作。
// 如果服务器为 nil 或已经停止，则返回 nil。
func (g *ServiceGrpc) CleanupTasks() error {
	if g.server == nil {
		return nil
	}
	// 优雅地停止 gRPC 服务器
	if err := g.server.Stop(context.Background()); err != nil {
		// 若停止失败，返回包含错误信息的插件错误
		return plugins.NewPluginError(g.ID(), "Stop", "Failed to stop HTTP server", err)
	}
	return nil
}

// Configure allows runtime configuration updates for the gRPC server.
// It accepts an interface{} parameter that should contain the new configuration
// and updates the server settings accordingly.
// Configure 允许在运行时更新 gRPC 服务器的配置。
// 它接受一个 interface{} 类型的参数，该参数应包含新的配置信息，
// 并相应地更新服务器设置。
func (g *ServiceGrpc) Configure(c any) error {
	if c == nil {
		return nil
	}
	// 将传入的配置转换为 *conf.Grpc 类型并更新服务器配置
	g.conf = c.(*conf.Grpc)
	return nil
}

// CheckHealth implements the health check interface for the gRPC server.
// It performs necessary health checks and updates the provided health report
// with the current status of the server.
// CheckHealth 实现了 gRPC 服务器的健康检查接口。
// 它执行必要的健康检查，并使用服务器的当前状态更新提供的健康报告。
func (g *ServiceGrpc) CheckHealth(report *plugins.HealthReport) error {
	return nil
}
