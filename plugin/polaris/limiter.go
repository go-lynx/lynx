package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
)

// HttpRateLimit 方法用于创建一个 HTTP 限流中间件
func (p *PlugPolaris) HttpRateLimit() middleware.Middleware {
	// 使用 Lynx 应用的 Helper 记录正在同步 [HTTP] 限流策略的信息
	app.Lynx().Helper().Infof("Synchronizing [HTTP] rate limit policy")
	// 调用 GetPolaris().Limiter 方法获取一个限流器实例，并设置服务名称和命名空间
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.Name()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// GrpcRateLimit 方法用于创建一个 gRPC 限流中间件
func (p *PlugPolaris) GrpcRateLimit() middleware.Middleware {
	// 使用 Lynx 应用的 Helper 记录正在同步 [GRPC] 限流策略的信息
	app.Lynx().Helper().Infof("Synchronizing [GRPC] rate limit policy")
	// 调用 GetPolaris().Limiter 方法获取一个限流器实例，并设置服务名称和命名空间
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.Name()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}
