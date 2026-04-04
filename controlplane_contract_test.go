package lynx

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/plugins"
)

type contractTestRateLimiterPlane struct{}
type contractTestServiceRegistry struct {
	discovery registry.Discovery
}
type contractTestDiscovery struct{}
type contractTestWatcher struct{}

func (contractTestRateLimiterPlane) HTTPRateLimit() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			return next(ctx, req)
		}
	}
}

func (contractTestRateLimiterPlane) GRPCRateLimit() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			return next(ctx, req)
		}
	}
}

func (contractTestRateLimiterPlane) ControlPlaneCapabilities() []ControlPlaneCapability {
	return []ControlPlaneCapability{
		ControlPlaneCapabilityRateLimit,
		ControlPlaneCapabilityTrafficProtection,
	}
}

func (s contractTestServiceRegistry) NewServiceRegistry() registry.Registrar {
	return nil
}

func (s contractTestServiceRegistry) NewServiceDiscovery() registry.Discovery {
	return s.discovery
}

func (contractTestDiscovery) GetService(context.Context, string) ([]*registry.ServiceInstance, error) {
	return nil, nil
}

func (contractTestDiscovery) Watch(context.Context, string) (registry.Watcher, error) {
	return contractTestWatcher{}, nil
}

func (contractTestWatcher) Next() ([]*registry.ServiceInstance, error) { return nil, nil }
func (contractTestWatcher) Stop() error                                { return nil }

func TestRegisterControlPlaneCapabilityResources_PublishesExplicitAliases(t *testing.T) {
	rt := plugins.NewSimpleRuntime()
	plane := contractTestRateLimiterPlane{}

	if err := RegisterControlPlaneCapabilityResources(rt, "fake.provider", plane); err != nil {
		t.Fatalf("register capability aliases: %v", err)
	}

	providerResource, err := rt.GetSharedResource("fake.provider")
	if err != nil {
		t.Fatalf("resolve provider resource: %v", err)
	}
	if _, ok := providerResource.(contractTestRateLimiterPlane); !ok {
		t.Fatalf("provider resource type = %T, want contractTestRateLimiterPlane", providerResource)
	}

	rateLimitResource, err := rt.GetSharedResource(ControlPlaneCapabilityResourceName("fake.provider", ControlPlaneCapabilityRateLimit))
	if err != nil {
		t.Fatalf("resolve rate_limit alias: %v", err)
	}
	if _, ok := rateLimitResource.(RateLimiter); !ok {
		t.Fatalf("rate_limit alias type = %T, want lynx.RateLimiter", rateLimitResource)
	}

	trafficProtectionResource, err := rt.GetSharedResource(ControlPlaneCapabilityResourceName("fake.provider", ControlPlaneCapabilityTrafficProtection))
	if err != nil {
		t.Fatalf("resolve traffic_protection alias: %v", err)
	}
	if _, ok := trafficProtectionResource.(contractTestRateLimiterPlane); !ok {
		t.Fatalf("traffic_protection alias type = %T, want contractTestRateLimiterPlane", trafficProtectionResource)
	}
}

func TestDefaultAppGetServiceDiscovery_UsesServiceRegistryCapability(t *testing.T) {
	resetGlobalState()
	cfg := createTestConfig(t)

	app, err := NewStandaloneApp(cfg)
	if err != nil {
		t.Fatalf("create standalone app: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
		resetGlobalState()
	})

	expected := contractTestDiscovery{}
	if err := app.SetServiceRegistry(contractTestServiceRegistry{discovery: expected}); err != nil {
		t.Fatalf("set service registry capability: %v", err)
	}
	SetDefaultApp(app)

	discovery, err := GetServiceDiscovery()
	if err != nil {
		t.Fatalf("resolve discovery from default app: %v", err)
	}
	if _, ok := discovery.(contractTestDiscovery); !ok {
		t.Fatalf("default app discovery type = %T, want contractTestDiscovery", discovery)
	}
}

func TestCompositeControlPlane_GRPCRateLimit_UsesRateLimiterCapability(t *testing.T) {
	resetGlobalState()
	cfg := createTestConfig(t)

	app, err := NewStandaloneApp(cfg)
	if err != nil {
		t.Fatalf("create standalone app: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
		resetGlobalState()
	})

	if err := app.SetRateLimiter(contractTestRateLimiterPlane{}); err != nil {
		t.Fatalf("set rate limiter capability: %v", err)
	}

	controlPlane := app.GetControlPlane()
	if controlPlane == nil {
		t.Fatal("expected composite control plane to exist after attaching rate limiter capability")
	}
	if controlPlane.GRPCRateLimit() == nil {
		t.Fatal("expected gRPC rate limit middleware from composite control plane")
	}
}
