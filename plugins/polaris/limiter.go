package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
)

// HTTPRateLimit 方法用于创建一个 HTTP 限流中间件。
// 该中间件会从 Polaris 获取 HTTP 限流策略，并应用到 HTTP 请求处理流程中。
// 返回值为一个实现了 middleware.Middleware 接口的中间件实例。
func (p *PlugPolaris) HTTPRateLimit() middleware.Middleware {
	// 使用 Lynx 应用的日志辅助器记录正在同步 HTTP 限流策略的信息
	log.Infof("Synchronizing [HTTP] rate limit policy")
	// 调用 GetPolaris().Limiter 方法获取一个限流实例，同时设置服务名称和命名空间
	// 服务名称通过 app.GetName() 获取，命名空间从插件配置中获取
	// 最后调用 polaris.RateLimit 方法将限流实例转换为中间件
	return polaris.Ratelimit(GetPolaris().Limiter(
		// 设置限流服务名称为当前应用的名称
		polaris.WithLimiterService(app.GetName()),
		// 设置限流服务的命名空间为插件配置中的命名空间
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// GRPCRateLimit 方法用于创建一个 gRPC 限流中间件。
// 该中间件会从 Polaris 获取 gRPC 限流策略，并应用到 gRPC 请求处理流程中。
// 返回值为一个实现了 middleware.Middleware 接口的中间件实例。
func (p *PlugPolaris) GRPCRateLimit() middleware.Middleware {
	// 使用 Lynx 应用的日志辅助器记录正在同步 gRPC 限流策略的信息
	log.Infof("Synchronizing [GRPC] rate limit policy")
	// 调用 GetPolaris().Limiter 方法获取一个限流实例，同时设置服务名称和命名空间
	// 服务名称通过 app.GetName() 获取，命名空间从插件配置中获取
	// 最后调用 polaris.RateLimit 方法将限流实例转换为中间件
	return polaris.Ratelimit(GetPolaris().Limiter(
		// 设置限流服务名称为当前应用的名称
		polaris.WithLimiterService(app.GetName()),
		// 设置限流服务的命名空间为插件配置中的命名空间
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}
