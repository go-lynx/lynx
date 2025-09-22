package grpc

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/grpc"
)

// ClientIntegration provides integration with app/subscribe
type ClientIntegration struct {
	clientPlugin  *ClientPlugin
	discovery     registry.Discovery
	routerFactory func(string) selector.NodeFilter
}

// NewGrpcClientIntegration creates a new gRPC client integration
func NewGrpcClientIntegration(discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) *ClientIntegration {
	return &ClientIntegration{
		discovery:     discovery,
		routerFactory: routerFactory,
	}
}

// SetClientPlugin sets the gRPC client plugin
func (g *ClientIntegration) SetClientPlugin(plugin *ClientPlugin) {
	g.clientPlugin = plugin
}

// BuildGrpcSubscriptions builds gRPC subscription connections using the client plugin
func (g *ClientIntegration) BuildGrpcSubscriptions(cfg *conf.Subscriptions) (map[string]*grpc.ClientConn, error) {
	if g.clientPlugin == nil {
		return nil, fmt.Errorf("gRPC client plugin not initialized")
	}

	// Initialize the connection map to store gRPC connections
	conns := make(map[string]*grpc.ClientConn)

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

		// Build client configuration
		clientConfig := ClientConfig{
			ServiceName: name,
			Discovery:   g.discovery,
			TLS:         item.GetTls(),
			Timeout:     g.clientPlugin.conf.GetDefaultTimeout().AsDuration(),
			KeepAlive:   g.clientPlugin.conf.GetDefaultKeepAlive().AsDuration(),
			Middleware:  g.clientPlugin.getDefaultMiddleware(),
		}

		// Add node router factory if provided
		if g.routerFactory != nil {
			clientConfig.NodeFilter = g.routerFactory(name)
		}

		// Configure TLS if enabled
		if item.GetTls() {
			clientConfig.TLS = true
			// Note: TLS configuration would be handled by the client plugin
			// based on the certificate management system
		}

		// Create connection using the client plugin
		conn, err := g.clientPlugin.CreateConnection(clientConfig)
		if err != nil {
			if item.GetRequired() {
				return nil, fmt.Errorf("required grpc subscription failed: %s, error: %v", name, err)
			}
			log.Warnf("grpc subscription created failed: %s, error: %v", name, err)
			continue
		}

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

// GetConnection gets a connection for a specific service
func (g *ClientIntegration) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	if g.clientPlugin == nil {
		return nil, fmt.Errorf("gRPC client plugin not initialized")
	}

	return g.clientPlugin.GetConnection(serviceName)
}

// CloseConnection closes a connection for a specific service
func (g *ClientIntegration) CloseConnection(serviceName string) error {
	if g.clientPlugin == nil {
		return fmt.Errorf("gRPC client plugin not initialized")
	}

	return g.clientPlugin.connectionPool.CloseConnection(serviceName)
}

// GetConnectionStatus returns the status of all connections
func (g *ClientIntegration) GetConnectionStatus() map[string]string {
	if g.clientPlugin == nil {
		return make(map[string]string)
	}

	return g.clientPlugin.GetConnectionStatus()
}

// GetConnectionCount returns the number of active connections
func (g *ClientIntegration) GetConnectionCount() int {
	if g.clientPlugin == nil {
		return 0
	}

	return g.clientPlugin.GetConnectionCount()
}

// HealthCheck performs health check on all connections
func (g *ClientIntegration) HealthCheck() error {
	if g.clientPlugin == nil {
		return fmt.Errorf("gRPC client plugin not initialized")
	}

	return g.clientPlugin.CheckHealth()
}

// GetMetrics returns client metrics
func (g *ClientIntegration) GetMetrics() *ClientMetrics {
	if g.clientPlugin == nil {
		return nil
	}

	return g.clientPlugin.metrics
}
