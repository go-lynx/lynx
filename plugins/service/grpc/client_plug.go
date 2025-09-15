package grpc

import (
	"fmt"

	"github.com/go-lynx/lynx/app"
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
func GetGrpcClientPlugin() (*ClientPlugin, error) {
	plugin := app.Lynx().GetPluginManager().GetPlugin("grpc.client")
	if plugin == nil {
		return nil, fmt.Errorf("gRPC client plugin not found")
	}

	clientPlugin, ok := plugin.(*ClientPlugin)
	if !ok {
		return nil, fmt.Errorf("invalid gRPC client plugin type")
	}

	return clientPlugin, nil
}

// GetGrpcClientConnection gets a gRPC client connection for the specified service
func GetGrpcClientConnection(serviceName string) (*grpc.ClientConn, error) {
	plugin, err := GetGrpcClientPlugin()
	if err != nil {
		return nil, err
	}

	return plugin.GetConnection(serviceName)
}

// CreateGrpcClientConnection creates a new gRPC client connection with custom configuration
func CreateGrpcClientConnection(config ClientConfig) (*grpc.ClientConn, error) {
	plugin, err := GetGrpcClientPlugin()
	if err != nil {
		return nil, err
	}

	return plugin.CreateConnection(config)
}

// CloseGrpcClientConnection closes a gRPC client connection
func CloseGrpcClientConnection(serviceName string) error {
	plugin, err := GetGrpcClientPlugin()
	if err != nil {
		return err
	}

	return plugin.CloseConnection(serviceName)
}

// GetGrpcClientConnectionStatus returns the status of all gRPC client connections
func GetGrpcClientConnectionStatus() (map[string]string, error) {
	plugin, err := GetGrpcClientPlugin()
	if err != nil {
		return nil, err
	}

	return plugin.GetConnectionStatus(), nil
}

// GetGrpcClientConnectionCount returns the number of active gRPC client connections
func GetGrpcClientConnectionCount() (int, error) {
	plugin, err := GetGrpcClientPlugin()
	if err != nil {
		return 0, err
	}

	return plugin.GetConnectionCount(), nil
}
