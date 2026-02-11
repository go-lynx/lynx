package subscribe

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/log"
	ggrpc "google.golang.org/grpc"
)

// ClientTLSProviders provides TLS-related dependencies for gRPC client connections.
// Pass to BuildGrpcSubscriptions when subscriptions use TLS.
// - ConfigProvider: loads CA certificate from control plane by name/group (used when caName is configured)
// - DefaultRootCA: returns application's root CA (used when caName is not set)
type ClientTLSProviders struct {
	ConfigProvider func(name, group string) (config.Source, error)
	DefaultRootCA  func() []byte
}

// BuildGrpcSubscriptions builds gRPC subscription connections based on configuration.
// When subscriptions use TLS, pass tlsProviders from control plane and certificate provider.
// tlsProviders can be nil if no subscription uses TLS.
// Returns map where key is service name, value is reusable gRPC ClientConn.
func BuildGrpcSubscriptions(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter, tlsProviders *ClientTLSProviders) (map[string]*ggrpc.ClientConn, error) {
	// Initialize the connection map to store gRPC connections
	conns := make(map[string]*ggrpc.ClientConn)

	// Return early if no gRPC subscriptions are configured
	if cfg == nil || len(cfg.GetGrpc()) == 0 {
		return conns, nil
	}

	// Iterate through each gRPC subscription configuration
	for _, item := range cfg.GetGrpc() {
		// Get the service name from configuration
		name := item.GetService()

		// Skip empty service names
		if name == "" {
			log.Warnf("skip empty grpc subscription entry")
			continue
		}

		// Build subscription options
		opts := []Option{
			WithServiceName(name),
			WithDiscovery(discovery),
		}

		// Add node router factory if provided
		if routerFactory != nil {
			opts = append(opts, WithNodeRouterFactory(routerFactory))
		}

		// Enable TLS if configured
		if item.GetTls() {
			opts = append(opts, EnableTls())
			// Inject TLS providers when available
			if tlsProviders != nil {
				if tlsProviders.ConfigProvider != nil {
					opts = append(opts, WithConfigProvider(tlsProviders.ConfigProvider))
				}
				if tlsProviders.DefaultRootCA != nil {
					opts = append(opts, WithDefaultRootCA(tlsProviders.DefaultRootCA))
				}
			}
		}

		// Add root CA file name if configured
		if cn := item.GetCaName(); cn != "" {
			opts = append(opts, WithRootCAFileName(cn))
		}

		// Add root CA file group if configured
		if cg := item.GetCaGroup(); cg != "" {
			opts = append(opts, WithRootCAFileGroup(cg))
		}

		// Mark as required subscription if configured
		if item.GetRequired() {
			opts = append(opts, Required())
		}

		// Create and execute the subscription
		sub := NewGrpcSubscribe(opts...)
		conn := sub.Subscribe()

		// Handle connection failure
		if conn == nil {
			if item.GetRequired() {
				return nil, fmt.Errorf("required grpc subscription failed: %s", name)
			}
			log.Warnf("grpc subscription created nil conn: %s", name)
			continue
		}

		// Warm-up: Simple connection state check (optional)
		state := conn.GetState()
		log.Infof("grpc subscription established: service=%s state=%s", name, state.String())

		// Store the successful connection
		conns[name] = conn
	}

	return conns, nil
}
