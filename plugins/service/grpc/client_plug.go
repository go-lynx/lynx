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

// GetGrpcClientPlugin retrieves the gRPC client plugin from the plugin manager
func GetGrpcClientPlugin(pluginManager interface{}) (*ClientPlugin, error) {
	if pluginManager == nil {
		return nil, fmt.Errorf("plugin manager cannot be nil")
	}

	// Try to create plugin from factory
	plugin, err := factory.GlobalTypedFactory().CreatePlugin("grpc.client")
	if err != nil {
		return nil, fmt.Errorf("gRPC client plugin not found in factory: %w", err)
	}

	// Type assertion to ClientPlugin
	clientPlugin, ok := plugin.(*ClientPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin is not a ClientPlugin instance")
	}

	return clientPlugin, nil
}

// GetOrCreateGrpcClientPlugin gets existing plugin or creates a new one
func GetOrCreateGrpcClientPlugin() *ClientPlugin {
	plugin, err := factory.GlobalTypedFactory().CreatePlugin("grpc.client")
	if err == nil {
		if clientPlugin, ok := plugin.(*ClientPlugin); ok {
			return clientPlugin
		}
	}
	
	// Create new plugin if not found
	return NewGrpcClientPlugin()
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

	return plugin.connectionPool.CloseConnection(serviceName)
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
