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
	"github.com/go-lynx/lynx/log"
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
		tls: false,
	}
	for _, o := range opts {
		o(gs)
	}
	return gs
}

// Subscribe dials the configured gRPC service via discovery and returns the
// client connection, or nil on failure (the error is logged). When the service
// is marked Required, it blocks until the connection reaches Ready or a 10s
// timeout elapses, closing and returning nil on timeout.
func (g *GrpcSubscribe) Subscribe() *gGrpc.ClientConn {
	if g.svcName == "" {
		return nil
	}
	var tlsConf *tls.Config
	if g.tls {
		conf, err := g.buildClientTLSConfig()
		if err != nil {
			log.Error(err)
			return nil
		}
		tlsConf = conf
	}
	opts := []grpc.ClientOption{
		grpc.WithEndpoint("discovery:///" + g.svcName),
		grpc.WithDiscovery(g.discovery),
		grpc.WithMiddleware(
			logging.Client(log.Logger),
			tracing.Client(),
		),
	}
	// Only pass a TLS config when one was built; a nil config must not be passed.
	if tlsConf != nil {
		opts = append(opts, grpc.WithTLSConfig(tlsConf))
	}
	opts = append(opts, grpc.WithNodeFilter(g.nodeFilter()))

	var conn *gGrpc.ClientConn
	var err error
	// Bound the dial so a missing/unreachable service cannot hang startup.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if g.tls {
		conn, err = grpc.Dial(ctx, opts...)
	} else {
		conn, err = grpc.DialInsecure(ctx, opts...)
	}
	if err != nil {
		log.Error(err)
		return nil
	}

	// A required dependency must be reachable before we proceed, so wait for Ready.
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
				log.Error(fmt.Errorf("grpc subscribe connection to %s not ready within %v (last_state=%s)", g.svcName, waitTimeout, state.String()))
				_ = conn.Close()
				return nil
			}
		}
	}
	return conn
}

func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if g.routerFactory == nil {
		return nil
	}
	return g.routerFactory(g.svcName)
}
