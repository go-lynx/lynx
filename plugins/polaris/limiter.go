package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
)

// MiddlewareAdapter 中间件适配器
// 职责：提供 HTTP/gRPC 限流中间件和路由中间件

// HTTPRateLimit 创建 HTTP 限流中间件
// 从 Polaris 获取 HTTP 限流策略，并应用到 HTTP 请求处理流程中
func (p *PlugPolaris) HTTPRateLimit() middleware.Middleware {
	if !p.IsInitialized() {
		log.Warnf("Polaris plugin not initialized, returning nil HTTP rate limit middleware")
		return nil
	}

	log.Infof("Synchronizing [HTTP] rate limit policy")

	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// GRPCRateLimit 创建 gRPC 限流中间件
// 从 Polaris 获取 gRPC 限流策略，并应用到 gRPC 请求处理流程中
func (p *PlugPolaris) GRPCRateLimit() middleware.Middleware {
	if !p.IsInitialized() {
		log.Warnf("Polaris plugin not initialized, returning nil gRPC rate limit middleware")
		return nil
	}

	log.Infof("Synchronizing [GRPC] rate limit policy")

	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}
