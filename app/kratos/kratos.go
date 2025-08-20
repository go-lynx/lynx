// Package kratos provides integration with the Kratos framework
package kratos

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// Options holds the configuration for creating a Kratos application
type Options struct {
	// GRPCServer instance
	GRPCServer *grpc.Server
	// HTTPServer instance
	HTTPServer *http.Server
	// Registrar for service registration
	Registrar registry.Registrar
}

// NewKratos creates a new Kratos application with the specified options.
// It supports creating applications with HTTP server, gRPC server, or both.
//
// Parameters:
//   - opts: Options struct containing all necessary configuration
//
// Returns:
//   - *kratos.App: The created Kratos application
//   - error: Any error that occurred during creation
func NewKratos(opts Options) (*kratos.App, error) {
	// Validate required fields
	if app.GetHost() == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if app.GetName() == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	if log.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Prepare base options for Kratos application
	kratosOpts := []kratos.Option{
		// Set the application ID to the host name
		kratos.ID(app.GetHost()),
		// Set the application name
		kratos.Name(app.GetName()),
		// Set the application version
		kratos.Version(app.GetVersion()),
		// Set the application metadata with basic info
		kratos.Metadata(map[string]string{
			"host":    app.GetHost(),
			"version": app.GetVersion(),
		}),
		// Set the application logger
		kratos.Logger(log.Logger),
		// Set the application registrar
		kratos.Registrar(opts.Registrar),
	}

	// Collect all available transport servers based on the provided options.
	// This function checks for the presence of both gRPC and HTTP servers
	// in the options and adds them to a list of transport servers.
	// If any servers are available, it appends them to the Kratos application options.
	//
	// Variables:
	//   - serverList: A slice to hold all available transport servers.
	var serverList []transport.Server

	// Check if a gRPC server instance is provided in the options.
	// If available, add it to the list of transport servers.
	if opts.GRPCServer != nil {
		// Add the gRPC server to the list of transport servers
		serverList = append(serverList, opts.GRPCServer)
	}

	// Check if an HTTP server instance is provided in the options.
	// If available, add it to the list of transport servers.
	if opts.HTTPServer != nil {
		// Add the HTTP server to the list of transport servers
		serverList = append(serverList, opts.HTTPServer)
	}

	// Check if there are any servers in the list.
	// If so, append them to the Kratos application options.
	if len(serverList) > 0 {
		// Add all collected servers to the Kratos application options
		kratosOpts = append(kratosOpts, kratos.Server(serverList...))
	}

	// Create and return the Kratos application
	return kratos.New(kratosOpts...), nil
}

// ProvideKratosOptions creates and returns an Options struct instance based on the provided parameters.
// This struct is used to store configuration information required for creating a Kratos application.
//
// Parameters:
//   - grpcServer: gRPC server instance.
//   - httpServer: HTTP server instance.
//   - registrar: Registrar for service registration.
//
// Returns:
//   - Options: Options struct instance containing all provided configuration information.
func ProvideKratosOptions(
	grpcServer *grpc.Server,
	httpServer *http.Server,
	registrar registry.Registrar,
) Options {
	// Return an initialized Options struct instance, assigning the provided parameters to corresponding fields
	return Options{
		GRPCServer: grpcServer,
		HTTPServer: httpServer,
		Registrar:  registrar,
	}
}
