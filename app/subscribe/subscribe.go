package subscribe

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app/log"
	gGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// GrpcSubscribe represents a struct for subscribing to GRPC services.
// Contains service name, service discovery instance, TLS enablement, root CA filename and file group information.
type GrpcSubscribe struct {
	svcName   string             // Name of the GRPC service to subscribe to
	discovery registry.Discovery // Service discovery instance for discovering service nodes
	tls       bool               // Whether TLS encryption is enabled
	caName    string             // Root CA certificate filename
	caGroup   string             // Group that the root CA certificate file belongs to
	required  bool               // Whether it's a strongly dependent upstream service, will be checked at startup
	// Optional: Node router factory, injected by upper layer to avoid direct dependency on app package
	routerFactory func(service string) selector.NodeFilter
	// Optional: Configuration source provider, injected by upper layer for getting config source by name/group
	configProvider func(name, group string) (config.Source, error)
	// Optional: Default RootCA provider, injected by upper layer for directly obtaining application RootCA
	defaultRootCA func() []byte
}

// Option defines a function type for configuring GrpcSubscribe instances.
type Option func(o *GrpcSubscribe)

// WithServiceName returns an Option function for setting the name of the GRPC service to subscribe to.
func WithServiceName(svcName string) Option {
	return func(o *GrpcSubscribe) {
		o.svcName = svcName
	}
}

// WithDiscovery returns an Option function for setting the service discovery instance.
func WithDiscovery(discovery registry.Discovery) Option {
	return func(o *GrpcSubscribe) {
		o.discovery = discovery
	}
}

// EnableTls returns an Option function for enabling TLS encryption.
func EnableTls() Option {
	return func(o *GrpcSubscribe) {
		o.tls = true
	}
}

// WithRootCAFileName returns an Option function for setting the root CA certificate filename.
func WithRootCAFileName(caName string) Option {
	return func(o *GrpcSubscribe) {
		o.caName = caName
	}
}

// WithRootCAFileGroup returns an Option function for setting the group that the root CA certificate file belongs to.
func WithRootCAFileGroup(caGroup string) Option {
	return func(o *GrpcSubscribe) {
		o.caGroup = caGroup
	}
}

// Required returns an Option function for setting the service as a strongly dependent upstream service.
func Required() Option {
	return func(o *GrpcSubscribe) {
		o.required = true
	}
}

// WithNodeRouterFactory injects node router factory (optional)
func WithNodeRouterFactory(f func(string) selector.NodeFilter) Option {
	return func(o *GrpcSubscribe) {
		o.routerFactory = f
	}
}

// WithConfigProvider injects configuration source provider (name, group) -> config.Source
func WithConfigProvider(f func(name, group string) (config.Source, error)) Option {
	return func(o *GrpcSubscribe) {
		o.configProvider = f
	}
}

// WithDefaultRootCA injects default RootCA provider
func WithDefaultRootCA(f func() []byte) Option {
	return func(o *GrpcSubscribe) {
		o.defaultRootCA = f
	}
}

// NewGrpcSubscribe creates a new GrpcSubscribe instance using the provided options.
// If no options are provided, default configuration will be used.
func NewGrpcSubscribe(opts ...Option) *GrpcSubscribe {
	gs := &GrpcSubscribe{
		tls: false, // Default to not enabling TLS encryption
	}
	// Apply provided option configurations
	for _, o := range opts {
		o(gs)
	}
	return gs
}

// Subscribe subscribes to the specified GRPC service and returns a gGrpc.ClientConn connection instance.
// Returns nil if service name is empty.
func (g *GrpcSubscribe) Subscribe() *gGrpc.ClientConn {
	if g.svcName == "" {
		return nil
	}
	// Prepare TLS config if needed
	var tlsConf *tls.Config
	if g.tls {
		conf, err := g.tlsLoad()
		if err != nil {
			// Log error and return nil (handled by caller for required case)
			log.Error(err)
			return nil
		}
		tlsConf = conf
	}
	// Configure gRPC client options
	opts := []grpc.ClientOption{
		grpc.WithEndpoint("discovery:///" + g.svcName), // Set service discovery endpoint
		grpc.WithDiscovery(g.discovery),                // Set service discovery instance
		grpc.WithMiddleware(
			logging.Client(log.Logger), // Add logging middleware
			tracing.Client(),           // Add tracing middleware
		),
		// Set TLS configuration only when enabled
		// Note: nil TLS config should not be passed
	}
	if tlsConf != nil {
		opts = append(opts, grpc.WithTLSConfig(tlsConf))
	}
	opts = append(opts, grpc.WithNodeFilter(g.nodeFilter())) // Set node filter

	var conn *gGrpc.ClientConn
	var err error
	// Use a dial timeout context to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if g.tls {
		// When TLS is enabled, use secure connection
		conn, err = grpc.Dial(ctx, opts...)
	} else {
		// When TLS is not enabled, use insecure connection
		conn, err = grpc.DialInsecure(ctx, opts...)
	}
	if err != nil {
		// Log error and throw exception
		log.Error(err)
		return nil
	}

	// If marked as required, block until connection is Ready or timeout
	if g.required && conn != nil {
		waitTimeout := 10 * time.Second
		waitCtx, cancel := context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()
		conn.Connect()
		for {
			state := conn.GetState()
			if state == connectivity.Ready {
				break
			}
			if !conn.WaitForStateChange(waitCtx, state) {
				// timeout or context done
				log.Error(fmt.Errorf("grpc subscribe connection to %s not ready within %v (last_state=%s)", g.svcName, waitTimeout, state.String()))
				_ = conn.Close()
				return nil
			}
		}
	}
	return conn
}

// Internal: Create node filter based on injected factory
func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if g.routerFactory == nil {
		return nil
	}
	return g.routerFactory(g.svcName)
}
