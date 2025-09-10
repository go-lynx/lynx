// Package grpc provides a gRPC server plugin for the Lynx framework.
// It implements the necessary interfaces to integrate with the Lynx plugin system
// and provides functionality for setting up and managing a gRPC server with various
// middleware options and TLS support.
package grpc

import (
	"context"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata constants define the basic information about the gRPC plugin
const (
	// pluginName is the unique identifier for the gRPC server plugin
	pluginName = "grpc.server"

	// pluginVersion indicates the current version of the plugin
	pluginVersion = "v2.0.0"

	// pluginDescription provides a brief description of the plugin's functionality
	pluginDescription = "grpc server plugin for lynx framework"

	// confPrefix is the configuration prefix used for loading gRPC settings
	confPrefix = "lynx.grpc"
)

// ServiceGrpc represents the gRPC server plugin implementation.
// It embeds the BasePlugin for common plugin functionality and maintains
// the gRPC server instance along with its configuration.
type ServiceGrpc struct {
	// Embed Lynx framework's base plugin, inheriting common plugin functionality
	*plugins.BasePlugin
	// gRPC server instance
	server *grpc.Server
	// gRPC server configuration information
	conf *conf.Grpc
}

// NewServiceGrpc creates and initializes a new instance of the gRPC server plugin.
// It sets up the base plugin with the appropriate metadata and returns a pointer
// to the ServiceGrpc structure.
func NewServiceGrpc() *ServiceGrpc {
	return &ServiceGrpc{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			10,
		),
	}
}

// InitializeResources implements the plugin initialization interface.
// It loads and validates the gRPC server configuration from the runtime environment.
// If no configuration is provided, it sets up default values for the server.
func (g *ServiceGrpc) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	g.conf = &conf.Grpc{}

	// Scan and load gRPC configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(g.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	defaultConf := &conf.Grpc{
		// Default network protocol is TCP
		Network: "tcp",
		// Default listening address is :9090
		Addr: ":9090",
		// TLS is disabled by default
		TlsEnable: false,
		// No client authentication by default
		TlsAuthType: 0,
		// Default timeout is 10 seconds
		Timeout: &durationpb.Duration{Seconds: 10},
	}

	// Use default values for unset fields
	if g.conf.Network == "" {
		g.conf.Network = defaultConf.Network
	}
	if g.conf.Addr == "" {
		g.conf.Addr = defaultConf.Addr
	}
	if g.conf.Timeout == nil {
		g.conf.Timeout = defaultConf.Timeout
	}

	return nil
}

// StartupTasks implements the plugin startup interface.
// It configures and starts the gRPC server with all necessary middleware and options,
// including tracing, logging, rate limiting, validation, and recovery handlers.
func (g *ServiceGrpc) StartupTasks() error {
	// Log gRPC service startup
	log.Infof("starting grpc service")

	var middlewares []middleware.Middleware

	// Add base middleware
	middlewares = append(middlewares,
		// Configure tracing middleware with application name as tracer name
		tracing.Server(tracing.WithTracerName(app.GetName())),
		// Configure logging middleware using Lynx framework's logger
		logging.Server(log.Logger),
		// Configure validation middleware
		validate.ProtoValidate(),
		// Configure recovery middleware to handle panics during request processing
		recovery.Recovery(
			recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
				log.ErrorCtx(ctx, err)
				return nil
			}),
		),
	)
	// Configure rate limiting middleware using Lynx framework's control plane HTTP rate limit strategy
	// If there is a rate limiting middleware, append it
	if rl := app.Lynx().GetControlPlane().GRPCRateLimit(); rl != nil {
		middlewares = append(middlewares, rl)
	}
	gMiddlewares := grpc.Middleware(middlewares...)

	// Define gRPC server options list
	opts := []grpc.ServerOption{
		gMiddlewares,
	}

	// Configure server options based on configuration
	if g.conf.Network != "" {
		// Set network protocol
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	if g.conf.Addr != "" {
		// Set listening address
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	if g.conf.Timeout != nil {
		// Set timeout
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	if g.conf.GetTlsEnable() {
		// If TLS is enabled, add TLS configuration options
		opts = append(opts, g.tlsLoad())
	}

	// Create gRPC server instance
	g.server = grpc.NewServer(opts...)
	// Log successful gRPC service startup
	log.Infof("grpc service successfully started")
	return nil
}

// CleanupTasks implements the plugin cleanup interface.
// It gracefully stops the gRPC server and performs necessary cleanup operations.
// If the server is nil or already stopped, it will return nil.
func (g *ServiceGrpc) CleanupTasks() error {
	if g.server == nil {
		return nil
	}
	// Gracefully stop the gRPC server
	// Use timeout to avoid indefinite blocking on shutdown
	ctx, cancel := context.WithTimeout(context.Background(), g.conf.GetTimeout().AsDuration())
	defer cancel()
	if err := g.server.Stop(ctx); err != nil {
		// If stopping fails, return plugin error with error information
		return plugins.NewPluginError(g.ID(), "Stop", "Failed to stop gRPC server", err)
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
	// Convert the incoming configuration to *conf.Grpc type and update server configuration
	g.conf = c.(*conf.Grpc)
	return nil
}

// CheckHealth implements the health check interface for the gRPC server.
// It performs necessary health checks and updates the provided health report
// with the current status of the server.
func (g *ServiceGrpc) CheckHealth() error {
	return nil
}
