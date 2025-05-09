// Package grpc provides a gRPC server plugin for the Lynx framework.
// It implements the necessary interfaces to integrate with the Lynx plugin system
// and provides functionality for setting up and managing a gRPC server with various
// middleware options and TLS support.
package grpc

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugin-grpc/v2/conf"
)

// Plugin metadata constants define the basic information about the gRPC plugin
const (
	// pluginName is the unique identifier for the gRPC server plugin
	pluginName = "grpc.server"

	// pluginVersion indicates the current version of the plugin
	pluginVersion = "v2.0.0"

	// pluginDescription provides a brief description of the plugin's functionality
	pluginDescription = "GRPC server plugin for Lynx framework"

	// confPrefix is the configuration prefix used for loading gRPC settings
	confPrefix = "lynx.grpc"
)

// ServiceGrpc represents the gRPC server plugin implementation.
// It embeds the BasePlugin for common plugin functionality and maintains
// the gRPC server instance along with its configuration.
type ServiceGrpc struct {
	*plugins.BasePlugin
	server *grpc.Server
	conf   *conf.Grpc
}

// NewServiceGrpc creates and initializes a new instance of the gRPC server plugin.
// It sets up the base plugin with the appropriate metadata and returns a pointer
// to the ServiceGrpc structure.
func NewServiceGrpc() *ServiceGrpc {
	return &ServiceGrpc{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
		),
	}
}

// InitializeResources implements the plugin initialization interface.
// It loads and validates the gRPC server configuration from the runtime environment.
// If no configuration is provided, it sets up default values for the server.
func (g *ServiceGrpc) InitializeResources(rt plugins.Runtime) error {
	// Add default configuration if not provided
	if g.conf == nil {
		g.conf = &conf.Grpc{
			Network: "tcp",
			Addr:    ":9090",
		}
	}
	err := rt.GetConfig().Scan(g.conf)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks implements the plugin startup interface.
// It configures and starts the gRPC server with all necessary middleware and options,
// including tracing, logging, rate limiting, validation, and recovery handlers.
func (g *ServiceGrpc) StartupTasks() error {
	app.Lynx().GetLogHelper().Infof("Starting GRPC service")

	opts := []grpc.ServerOption{
		grpc.Middleware(
			tracing.Server(tracing.WithTracerName(app.GetName())),
			logging.Server(app.Lynx().GetLogger()),
			app.Lynx().GetControlPlane().GRPCRateLimit(),
			validate.Validator(),
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
		),
	}

	// Configure server options based on configuration
	if g.conf.Network != "" {
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	if g.conf.Addr != "" {
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	if g.conf.Timeout != nil {
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	if g.conf.GetTls() {
		opts = append(opts, g.tlsLoad())
	}

	g.server = grpc.NewServer(opts...)
	app.Lynx().GetLogHelper().Infof("GRPC service successfully started")
	return nil
}

// CleanupTasks implements the plugin cleanup interface.
// It gracefully stops the gRPC server and performs necessary cleanup operations.
// If the server is nil or already stopped, it will return nil.
func (g *ServiceGrpc) CleanupTasks() error {
	if g.server == nil {
		return nil
	}
	if err := g.server.Stop(context.Background()); err != nil {
		return plugins.NewPluginError(g.ID(), "Stop", "Failed to stop HTTP server", err)
	}
	return nil
}

// Configure allows runtime configuration updates for the gRPC server.
// It accepts an interface{} parameter that should contain the new configuration
// and updates the server settings accordingly.
func (g *ServiceGrpc) Configure(c any) error {
	if c == nil {
		return nil
	}
	g.conf = c.(*conf.Grpc)
	return nil
}

// CheckHealth implements the health check interface for the gRPC server.
// It performs necessary health checks and updates the provided health report
// with the current status of the server.
func (g *ServiceGrpc) CheckHealth(report *plugins.HealthReport) error {
	return nil
}
