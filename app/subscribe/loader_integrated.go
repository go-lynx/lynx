package subscribe

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/app/log"
	ggrpc "google.golang.org/grpc"
)

// BuildGrpcSubscriptionsWithPlugin builds gRPC subscription connections using the gRPC client plugin
// This is the new integrated version that uses the gRPC client plugin
func BuildGrpcSubscriptionsWithPlugin(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter, pluginManager interface{}) (map[string]*ggrpc.ClientConn, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, fall back to legacy method to avoid circular dependency
	log.Warnf("gRPC client plugin integration temporarily disabled due to circular dependency refactoring")
	return BuildGrpcSubscriptions(cfg, discovery, routerFactory)
}

// BuildGrpcSubscriptionsLegacy builds gRPC subscription connections using the legacy method
// This is kept for backward compatibility
func BuildGrpcSubscriptionsLegacy(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*ggrpc.ClientConn, error) {
	return BuildGrpcSubscriptions(cfg, discovery, routerFactory)
}

// GetGrpcConnection gets a gRPC connection for a specific service using the plugin
func GetGrpcConnection(serviceName string, pluginManager interface{}) (*ggrpc.ClientConn, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("GetGrpcConnection temporarily disabled due to circular dependency refactoring")
	return nil, fmt.Errorf("GetGrpcConnection needs to be implemented with proper dependency injection")
}

// CloseGrpcConnection closes a gRPC connection for a specific service
func CloseGrpcConnection(serviceName string, pluginManager interface{}) error {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("CloseGrpcConnection temporarily disabled due to circular dependency refactoring")
	return fmt.Errorf("CloseGrpcConnection needs to be implemented with proper dependency injection")
}

// GetGrpcConnectionStatus returns the status of all gRPC connections
func GetGrpcConnectionStatus(pluginManager interface{}) (map[string]string, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("GetGrpcConnectionStatus temporarily disabled due to circular dependency refactoring")
	return nil, fmt.Errorf("GetGrpcConnectionStatus needs to be implemented with proper dependency injection")
}

// GetGrpcConnectionCount returns the number of active gRPC connections
func GetGrpcConnectionCount(pluginManager interface{}) (int, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("GetGrpcConnectionCount temporarily disabled due to circular dependency refactoring")
	return 0, fmt.Errorf("GetGrpcConnectionCount needs to be implemented with proper dependency injection")
}

// HealthCheckGrpcConnections performs health check on all gRPC connections
func HealthCheckGrpcConnections(pluginManager interface{}) error {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("HealthCheckGrpcConnections temporarily disabled due to circular dependency refactoring")
	return fmt.Errorf("HealthCheckGrpcConnections needs to be implemented with proper dependency injection")
}

// GetGrpcMetrics returns gRPC client metrics
func GetGrpcMetrics(pluginManager interface{}) (*interface{}, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("GetGrpcMetrics temporarily disabled due to circular dependency refactoring")
	return nil, fmt.Errorf("GetGrpcMetrics needs to be implemented with proper dependency injection")
}

// InitializeGrpcClientIntegration initializes the gRPC client integration
// This should be called during application startup
func InitializeGrpcClientIntegration(pluginManager interface{}) error {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("InitializeGrpcClientIntegration temporarily disabled due to circular dependency refactoring")
	return fmt.Errorf("InitializeGrpcClientIntegration needs to be implemented with proper dependency injection")
}

// CreateGrpcConnectionWithConfig creates a gRPC connection with custom configuration
func CreateGrpcConnectionWithConfig(config interface{}, pluginManager interface{}) (*ggrpc.ClientConn, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("CreateGrpcConnectionWithConfig temporarily disabled due to circular dependency refactoring")
	return nil, fmt.Errorf("CreateGrpcConnectionWithConfig needs to be implemented with proper dependency injection")
}

// GetGrpcClientPlugin returns the gRPC client plugin instance
func GetGrpcClientPlugin(pluginManager interface{}) (interface{}, error) {
	// Note: This function needs to be refactored to use dependency injection
	// For now, return an error to indicate this needs to be implemented
	log.Warnf("GetGrpcClientPlugin temporarily disabled due to circular dependency refactoring")
	return nil, fmt.Errorf("GetGrpcClientPlugin needs to be implemented with proper dependency injection")
}
