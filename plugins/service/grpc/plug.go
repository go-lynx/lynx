package grpc

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/grpc"
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
func GetGrpcServer(pluginManager interface{}) (*grpc.Server, error) {
	// Get the plugin with the specified name from the application's plugin manager
	// Note: In real implementation, type assertion would be needed here
	// For now, return an error to indicate this needs to be implemented
	return nil, fmt.Errorf("GetGrpcServer needs to be implemented with proper plugin manager interface")
}
