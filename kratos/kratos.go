// Package kratos provides integration with the Kratos framework
package kratos

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	lynxapp "github.com/go-lynx/lynx"
)

// Options holds the configuration for creating a Kratos application.
// At least one of GRPCServer or HTTPServer should be set; otherwise the
// resulting app exposes no transports.
type Options struct {
	// App supplies app metadata (ID, name, version) and must be non-nil.
	App *lynxapp.LynxApp
	// GRPCServer is registered as a transport when non-nil.
	GRPCServer *grpc.Server
	// HTTPServer is registered as a transport when non-nil.
	HTTPServer *http.Server
	// Registrar handles service registration with the discovery backend.
	Registrar registry.Registrar
}

// NewKratos builds a Kratos application from opts, wiring in whichever of the
// gRPC and HTTP servers are provided. It returns an error if opts.App, its
// host/name, or the global logger are missing.
func NewKratos(opts Options) (*kratos.App, error) {
	if opts.App == nil {
		return nil, fmt.Errorf("lynx app cannot be nil")
	}
	// Validate required fields
	if opts.App.Host() == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if opts.App.Name() == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	if log.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// The host doubles as the Kratos application ID.
	kratosOpts := []kratos.Option{
		kratos.ID(opts.App.Host()),
		kratos.Name(opts.App.Name()),
		kratos.Version(opts.App.Version()),
		kratos.Metadata(map[string]string{
			"host":    opts.App.Host(),
			"version": opts.App.Version(),
		}),
		kratos.Logger(log.Logger),
		kratos.Registrar(opts.Registrar),
	}

	var serverList []transport.Server
	if opts.GRPCServer != nil {
		serverList = append(serverList, opts.GRPCServer)
	}
	if opts.HTTPServer != nil {
		serverList = append(serverList, opts.HTTPServer)
	}
	if len(serverList) > 0 {
		kratosOpts = append(kratosOpts, kratos.Server(serverList...))
	}

	return kratos.New(kratosOpts...), nil
}

// ProvideKratosOptions assembles an Options value from its components, intended
// as a Wire provider.
func ProvideKratosOptions(
	app *lynxapp.LynxApp,
	grpcServer *grpc.Server,
	httpServer *http.Server,
	registrar registry.Registrar,
) Options {
	return Options{
		App:        app,
		GRPCServer: grpcServer,
		HTTPServer: httpServer,
		Registrar:  registrar,
	}
}
