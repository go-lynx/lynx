package grpc

import (
	"fmt"

	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

// init function registers the gRPC client plugin to the global plugin factory
func init() {
	factory.GlobalTypedFactory().RegisterPlugin("grpc.client", "lynx.grpc.client", func() plugins.Plugin {
		return NewGrpcClientPlugin()
	})
}

// GetGrpcClientPlugin gets the gRPC client plugin instance from the plugin manager
func GetGrpcClientPlugin(pluginManager interface{}) (*ClientPlugin, error) {
	// Note: In real implementation, type assertion would be needed here
	// For now, return an error to indicate this needs to be implemented
	return nil, fmt.Errorf("GetGrpcClientPlugin needs to be implemented with proper plugin manager interface")
}

// GetGrpcClientConnection gets a gRPC client connection for the specified service
func GetGrpcClientConnection(serviceName string, pluginManager interface{}) (*grpc.ClientConn, error) {
	plugin, err := GetGrpcClientPlugin(pluginManager)
	if err != nil {
		return nil, err
	}

	return plugin.GetConnection(serviceName)
}

// CreateGrpcClientConnection creates a new gRPC client connection with custom configuration
func CreateGrpcClientConnection(config ClientConfig, pluginManager interface{}) (*grpc.ClientConn, error) {
	plugin, err := GetGrpcClientPlugin(pluginManager)
	if err != nil {
		return nil, err
	}

	return plugin.CreateConnection(config)
}

// CloseGrpcClientConnection closes a gRPC client connection
func CloseGrpcClientConnection(serviceName string, pluginManager interface{}) error {
	plugin, err := GetGrpcClientPlugin(pluginManager)
	if err != nil {
		return err
	}

	return plugin.CloseConnection(serviceName)
}

// GetGrpcClientConnectionStatus returns the status of all gRPC client connections
func GetGrpcClientConnectionStatus(pluginManager interface{}) (map[string]string, error) {
	plugin, err := GetGrpcClientPlugin(pluginManager)
	if err != nil {
		return nil, err
	}

	return plugin.GetConnectionStatus(), nil
}

// GetGrpcClientConnectionCount returns the number of active gRPC client connections
func GetGrpcClientConnectionCount(pluginManager interface{}) (int, error) {
	plugin, err := GetGrpcClientPlugin(pluginManager)
	if err != nil {
		return 0, err
	}

	return plugin.GetConnectionCount(), nil
}
