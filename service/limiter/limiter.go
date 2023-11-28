package limiter

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/boot"
	polaris2 "github.com/go-lynx/lynx/plugin/polaris"
)

func HttpRateLimit(lynx *boot.Lynx) middleware.Middleware {
	boot.dfLog.Infof("Synchronizing [HTTP] rate limit policy")
	return polaris.Ratelimit(polaris2.GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}

func GrpcRateLimit(lynx *boot.Lynx) middleware.Middleware {
	boot.dfLog.Infof("Synchronizing [GRPC] rate limit policy")
	return polaris.Ratelimit(polaris2.GetPolaris().Limiter(
		polaris.WithLimiterService(lynx.Application.Name),
		polaris.WithLimiterNamespace(lynx.Polaris.Namespace),
	))
}
