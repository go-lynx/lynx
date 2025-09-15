package subscribe

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/service/grpc"
	ggrpc "google.golang.org/grpc"
)

// BuildGrpcSubscriptionsWithPlugin builds gRPC subscription connections using the gRPC client plugin
// This is the new integrated version that uses the gRPC client plugin
func BuildGrpcSubscriptionsWithPlugin(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*ggrpc.ClientConn, error) {
	// Get the gRPC client plugin
	clientPlugin, err := grpc.GetGrpcClientPlugin()
	if err != nil {
		log.Warnf("gRPC client plugin not available, falling back to legacy method: %v", err)
		return BuildGrpcSubscriptions(cfg, discovery, routerFactory)
	}

	// Create integration instance
	integration := grpc.NewGrpcClientIntegration(discovery, routerFactory)
	integration.SetClientPlugin(clientPlugin)

	// Build subscriptions using the plugin
	return integration.BuildGrpcSubscriptions(cfg)
}

// BuildGrpcSubscriptionsLegacy builds gRPC subscription connections using the legacy method
// This is kept for backward compatibility
func BuildGrpcSubscriptionsLegacy(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*ggrpc.ClientConn, error) {
	return BuildGrpcSubscriptions(cfg, discovery, routerFactory)
}

// GetGrpcConnection gets a gRPC connection for a specific service using the plugin
func GetGrpcConnection(serviceName string) (*ggrpc.ClientConn, error) {
	// Try to get connection using the plugin first
	conn, err := grpc.GetGrpcClientConnection(serviceName)
	if err != nil {
		log.Warnf("Failed to get gRPC connection from plugin for service %s: %v", serviceName, err)
		return nil, err
	}
	return conn, nil
}

// CloseGrpcConnection closes a gRPC connection for a specific service
func CloseGrpcConnection(serviceName string) error {
	return grpc.CloseGrpcClientConnection(serviceName)
}

// GetGrpcConnectionStatus returns the status of all gRPC connections
func GetGrpcConnectionStatus() (map[string]string, error) {
	return grpc.GetGrpcClientConnectionStatus()
}

// GetGrpcConnectionCount returns the number of active gRPC connections
func GetGrpcConnectionCount() (int, error) {
	return grpc.GetGrpcClientConnectionCount()
}

// HealthCheckGrpcConnections performs health check on all gRPC connections
func HealthCheckGrpcConnections() error {
	clientPlugin, err := grpc.GetGrpcClientPlugin()
	if err != nil {
		return fmt.Errorf("gRPC client plugin not available: %v", err)
	}

	return clientPlugin.CheckHealth()
}

// GetGrpcMetrics returns gRPC client metrics
func GetGrpcMetrics() (*grpc.ClientMetrics, error) {
	clientPlugin, err := grpc.GetGrpcClientPlugin()
	if err != nil {
		return nil, fmt.Errorf("gRPC client plugin not available: %v", err)
	}

	return clientPlugin.metrics, nil
}

// InitializeGrpcClientIntegration initializes the gRPC client integration
// This should be called during application startup
func InitializeGrpcClientIntegration() error {
	// Check if gRPC client plugin is available
	_, err := grpc.GetGrpcClientPlugin()
	if err != nil {
		log.Warnf("gRPC client plugin not available: %v", err)
		return err
	}

	log.Infof("gRPC client integration initialized successfully")
	return nil
}

// CreateGrpcConnectionWithConfig creates a gRPC connection with custom configuration
func CreateGrpcConnectionWithConfig(config grpc.ClientConfig) (*ggrpc.ClientConn, error) {
	return grpc.CreateGrpcClientConnection(config)
}

// GetGrpcClientPlugin returns the gRPC client plugin instance
func GetGrpcClientPlugin() (*grpc.ClientPlugin, error) {
	return grpc.GetGrpcClientPlugin()
}

