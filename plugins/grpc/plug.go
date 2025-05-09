package grpc

import (
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init registers the gRPC server plugin with the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new instance of ServiceGrpc and registers it with the
// plugin factory using the configured plugin name and configuration prefix.
func init() {
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewServiceGrpc()
	})
}

// GetGrpcServer retrieves the gRPC server instance from the plugin manager.
// This function provides access to the underlying gRPC server for other
// parts of the application that need to register services or access
// server functionality.
//
// Returns:
//   - *grpc.Server: The configured gRPC server instance
//
// Note: This function will panic if the plugin is not properly initialized
// or if the plugin manager cannot find the gRPC plugin.
func GetGrpcServer() *grpc.Server {
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceGrpc).server
}
