package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init function registers the gRPC server plugin to the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new ServiceGrpc instance and registers it to the plugin factory with the configured plugin name and configuration prefix.
func init() {
	// Call the RegisterPlugin method of the global plugin factory for plugin registration
	// Pass in the plugin name, configuration prefix, and a function that returns a plugins.Plugin interface instance
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new ServiceGrpc instance
		return NewServiceGrpc()
	})
}

// GetGrpcServer gets the gRPC server instance from the plugin manager.
// This function provides access to the underlying gRPC server for other parts of the application
// that may need to register services or use server functionality.
//
// Returns:
//   - *grpc.Server: Configured gRPC server instance
//
// Note: This function will panic if the plugin is not properly initialized or if the plugin manager cannot find the gRPC plugin.
func GetGrpcServer() *grpc.Server {
	// Get the plugin with the specified name from the application's plugin manager,
	// convert it to *ServiceGrpc type, and return its server field
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceGrpc).server
}
