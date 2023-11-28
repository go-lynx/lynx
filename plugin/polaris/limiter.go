package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/conf"
)

func HttpRateLimit(lynx *conf.Lynx) middleware.Middleware {
	app.GetHelper().Infof("Synchronizing [HTTP] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}

func GrpcRateLimit(lynx *conf.Lynx) middleware.Middleware {
	app.GetHelper().Infof("Synchronizing [GRPC] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}
