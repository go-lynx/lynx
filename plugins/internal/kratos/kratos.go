// Package kratos provides integration with the Kratos framework
package kratos

import (
	"fmt"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// ServerType represents the type of server to be created
type ServerType int

const (
	// BothServers indicates both HTTP and gRPC servers should be created
	BothServers ServerType = iota
	// GRPCServer indicates only gRPC server should be created
	GRPCServer
	// HTTPServer indicates only HTTP server should be created
	HTTPServer
)

// Options holds the configuration for creating a Kratos application
type Options struct {
	// Logger for the application
	Logger log.Logger
	// GRPCServer instance
	GRPCServer *grpc.Server
	// HTTPServer instance
	HTTPServer *http.Server
	// Registrar for service registration
	Registrar registry.Registrar
	// ServerType specifies which servers to create
	Type ServerType
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
	// Validate required parameters
	if opts.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if opts.Registrar == nil {
		return nil, fmt.Errorf("registrar is required")
	}

	// Validate server configuration based on ServerType
	switch opts.Type {
	case BothServers:
		if opts.GRPCServer == nil || opts.HTTPServer == nil {
			return nil, fmt.Errorf("both GRPC and HTTP servers are required for BothServers type")
		}
	case GRPCServer:
		if opts.GRPCServer == nil {
			return nil, fmt.Errorf("GRPC server is required for GRPCServer type")
		}
	case HTTPServer:
		if opts.HTTPServer == nil {
			return nil, fmt.Errorf("HTTP server is required for HTTPServer type")
		}
	default:
		return nil, fmt.Errorf("invalid server type: %d", opts.Type)
	}

	// Prepare base options for Kratos application
	kratosOpts := []kratos.Option{
		kratos.ID(app.GetHost()),
		kratos.Name(app.GetName()),
		kratos.Version(app.GetVersion()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(opts.Logger),
		kratos.Registrar(opts.Registrar),
	}

	// Add servers based on ServerType
	switch opts.Type {
	case BothServers:
		kratosOpts = append(kratosOpts, kratos.Server(opts.GRPCServer, opts.HTTPServer))
	case GRPCServer:
		kratosOpts = append(kratosOpts, kratos.Server(opts.GRPCServer))
	case HTTPServer:
		kratosOpts = append(kratosOpts, kratos.Server(opts.HTTPServer))
	}

	// Create and return the Kratos application
	return kratos.New(kratosOpts...), nil
}
