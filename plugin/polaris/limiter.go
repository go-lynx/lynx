package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
)

func (p *PlugPolaris) HttpRateLimit() middleware.Middleware {
	app.Lynx().Helper().Infof("Synchronizing [HTTP] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.Name()),
		polaris.WithLimiterNamespace(GetPlugPolaris().conf.Namespace),
	))
}

func (p *PlugPolaris) GrpcRateLimit() middleware.Middleware {
	app.Lynx().Helper().Infof("Synchronizing [GRPC] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.Name()),
		polaris.WithLimiterNamespace(GetPlugPolaris().conf.Namespace),
	))
}
