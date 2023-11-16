package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/conf"
)

func HttpRateLimit(c *conf.Server) middleware.Middleware {
	dfLog.Infof("Synchronizing [HTTP] rate limit policy")
	return polaris.Ratelimit(Polaris().Limiter(
		polaris.WithLimiterService(c.Name),
		polaris.WithLimiterNamespace(c.Polaris.Namespace),
	))
}

func GrpcRateLimit(c *conf.Server) middleware.Middleware {
	dfLog.Infof("Synchronizing [GRPC] rate limit policy")
	return polaris.Ratelimit(Polaris().Limiter(
		polaris.WithLimiterService(c.Name),
		polaris.WithLimiterNamespace(c.Polaris.Namespace),
	))
}
