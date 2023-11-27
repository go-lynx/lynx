package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
)

func HttpRateLimit(lynx *Lynx) middleware.Middleware {
	dfLog.Infof("Synchronizing [HTTP] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}

func GrpcRateLimit(lynx *Lynx) middleware.Middleware {
	dfLog.Infof("Synchronizing [GRPC] rate limit policy")
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}
