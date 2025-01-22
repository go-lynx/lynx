package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
)

// HTTPRateLimit method is used to create an HTTP rate-limiting middleware
func (p *PlugPolaris) HTTPRateLimit() middleware.Middleware {
	// Use the Lynx application's Helper to record information about the HTTP rate-limiting policy being synchronized
	app.Lynx().GetLogHelper().Infof("Synchronizing [HTTP] rate limit policy")
	// Call the GetPolaris().RateLimiter method to obtain a rate limiter instance and set the service name and namespace
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// GRPCRateLimit method is used to create a gRPC rate-limiting middleware
func (p *PlugPolaris) GRPCRateLimit() middleware.Middleware {
	// Use the Lynx application's Helper to record information about the gRPC rate-limiting policy being synchronized
	app.Lynx().GetLogHelper().Infof("Synchronizing [GRPC] rate limit policy")
	// Call the GetPolaris().RateLimiter method to obtain a rate limiter instance and set the service name and namespace
	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}
