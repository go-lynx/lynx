package apollo

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/log"
)

// HTTPRateLimit implements the RateLimiter interface for HTTP rate limiting
// Apollo is a configuration center, not a rate limiting service, so this returns nil
func (p *PlugApollo) HTTPRateLimit() middleware.Middleware {
	log.Debugf("Apollo plugin does not support HTTP rate limiting, returning nil")
	return nil
}

// GRPCRateLimit implements the RateLimiter interface for gRPC rate limiting
// Apollo is a configuration center, not a rate limiting service, so this returns nil
func (p *PlugApollo) GRPCRateLimit() middleware.Middleware {
	log.Debugf("Apollo plugin does not support gRPC rate limiting, returning nil")
	return nil
}

// NewServiceRegistry implements the ServiceRegistry interface for service registration
// Apollo is a configuration center, not a service registry, so this returns nil
func (p *PlugApollo) NewServiceRegistry() registry.Registrar {
	log.Debugf("Apollo plugin does not support service registration, returning nil")
	return nil
}

// NewServiceDiscovery implements the ServiceRegistry interface for service discovery
// Apollo is a configuration center, not a service discovery service, so this returns nil
func (p *PlugApollo) NewServiceDiscovery() registry.Discovery {
	log.Debugf("Apollo plugin does not support service discovery, returning nil")
	return nil
}

// NewNodeRouter implements the RouteManager interface for service routing
// Apollo is a configuration center, not a routing service, so this returns nil
func (p *PlugApollo) NewNodeRouter(serviceName string) selector.NodeFilter {
	log.Debugf("Apollo plugin does not support service routing, returning nil for service: %s", serviceName)
	return nil
}

