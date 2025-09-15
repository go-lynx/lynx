package grpc

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init function registers the gRPC service plugin to the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new GrpcService instance and registers it to the package grpc

func init() {
	// Call the RegisterPlugin method of the global plugin factory for plugin registration
	// Pass in the plugin name, configuration prefix, and a function that returns a plugins.Plugin interface instance
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new GrpcService instance
		return NewGrpcService()
	})
}

// GetGrpcServer gets the gRPC server instance from the plugin manager.
// This function provides access to the underlying gRPC server for other parts of the application
// that may need to register services or use server functionality.
//
// Returns:
//   - *grpc.Server: Configured gRPC server instance
//   - error: Any error that occurred while retrieving the server
func GetGrpcServer() (*grpc.Server, error) {
	// Get the plugin with the specified name from the application's plugin manager
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("gRPC plugin not found")
	}

	// Convert to *GrpcService type
	grpcPlugin, ok := plugin.(*GrpcService)
	if !ok {
		return nil, fmt.Errorf("invalid gRPC plugin type")
	}

	// Check if server is initialized
	if grpcPlugin.server == nil {
		return nil, fmt.Errorf("gRPC server not initialized")
	}

	return grpcPlugin.server, nil
}
