// Package adapters provides internal adapter implementations for the Lynx framework.
// This file contains interface definitions used by adapters to bridge different components.
package adapters

import (
	"crypto/tls"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
)

// GrpcServiceProvider defines the interface for gRPC service operations.
// Implemented by adapters that provide gRPC server functionality.
type GrpcServiceProvider interface {
	// GetServer returns the gRPC server instance
	GetServer() (*kratosgrpc.Server, error)

	// GetApplicationName returns the application name
	GetApplicationName() string

	// GetLogger returns the logger instance
	GetLogger() interface{}

	// GetCertificateProvider returns the certificate provider
	GetCertificateProvider() CertificateProvider

	// GetControlPlane returns the control plane interface
	GetControlPlane() ControlPlane
}

// GrpcClientProvider defines the interface for gRPC client operations.
// Used by plugins that need to create gRPC client connections.
type GrpcClientProvider interface {
	// GetClientPlugin returns the gRPC client plugin
	GetClientPlugin() (GrpcClientPlugin, error)

	// GetClientConnection returns a gRPC client connection for a service
	GetClientConnection(serviceName string) (*grpc.ClientConn, error)

	// CreateClientConnection creates a new gRPC client connection with custom config
	CreateClientConnection(config GrpcClientConfig) (*grpc.ClientConn, error)
}

// GrpcClientPlugin defines the interface for gRPC client plugin.
type GrpcClientPlugin interface {
	// GetConnection returns a gRPC client connection for the specified service
	GetConnection(serviceName string) (*grpc.ClientConn, error)

	// CreateConnection creates a new gRPC client connection
	CreateConnection(config GrpcClientConfig) (*grpc.ClientConn, error)
}

// GrpcClientConfig represents configuration for a gRPC client connection.
type GrpcClientConfig struct {
	ServiceName    string
	Endpoint       string
	Discovery      registry.Discovery
	TLS            bool
	TLSAuthType    int32
	Timeout        int64 // in seconds
	KeepAlive      int64 // in seconds
	MaxRetries     int32
	RetryBackoff   int64 // in seconds
	MaxConnections int32
	Middleware     []interface{}
	NodeFilter     selector.NodeFilter
}

// CertificateProvider defines the interface for certificate operations.
// Used by TLS-enabled services to access certificates and keys.
type CertificateProvider interface {
	// GetCertificate returns the server certificate in PEM format
	GetCertificate() []byte

	// GetPrivateKey returns the server private key in PEM format
	GetPrivateKey() []byte

	// GetRootCA returns the root CA certificate in PEM format
	GetRootCA() []byte
}

// ControlPlane defines the interface for control plane operations.
// Provides service discovery and rate limiting functionality.
type ControlPlane interface {
	// Discovery returns the service discovery instance
	Discovery() interface{}

	// GRPCRateLimit returns the gRPC rate limit middleware
	GRPCRateLimit() interface{}
}

// GrpcSubscribeProvider defines the interface for gRPC subscription operations.
// Used for managing gRPC client connections to upstream services.
type GrpcSubscribeProvider interface {
	// BuildGrpcSubscriptions builds gRPC subscription connections
	BuildGrpcSubscriptions(cfg interface{}, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*grpc.ClientConn, error)

	// GetGrpcConnection gets a gRPC connection for a specific service
	GetGrpcConnection(serviceName string) (*grpc.ClientConn, error)

	// CloseGrpcConnection closes a gRPC connection for a specific service
	CloseGrpcConnection(serviceName string) error
}

// TLSConfigProvider defines the interface for TLS configuration.
// Used to build TLS configurations for secure connections.
type TLSConfigProvider interface {
	// BuildTLSConfig builds TLS configuration for gRPC
	BuildTLSConfig(tlsEnabled bool, authType int32) (*tls.Config, error)
}

// LoggerProvider defines the interface for logging operations.
type LoggerProvider interface {
	// GetLogger returns the logger instance
	GetLogger() interface{}

	// LogInfo logs an info message
	LogInfo(msg string, args ...interface{})

	// LogError logs an error message
	LogError(err error, msg string, args ...interface{})

	// LogWarn logs a warning message
	LogWarn(msg string, args ...interface{})
}

// DependencyInjector defines the interface for dependency injection.
// Manages registration and retrieval of various providers.
type DependencyInjector interface {
	// RegisterGrpcServiceProvider registers a gRPC service provider
	RegisterGrpcServiceProvider(provider GrpcServiceProvider)

	// RegisterGrpcClientProvider registers a gRPC client provider
	RegisterGrpcClientProvider(provider GrpcClientProvider)

	// RegisterGrpcSubscribeProvider registers a gRPC subscribe provider
	RegisterGrpcSubscribeProvider(provider GrpcSubscribeProvider)

	// RegisterTLSConfigProvider registers a TLS config provider
	RegisterTLSConfigProvider(provider TLSConfigProvider)

	// RegisterLoggerProvider registers a logger provider
	RegisterLoggerProvider(provider LoggerProvider)

	// GetGrpcServiceProvider returns the registered gRPC service provider
	GetGrpcServiceProvider() GrpcServiceProvider

	// GetGrpcClientProvider returns the registered gRPC client provider
	GetGrpcClientProvider() GrpcClientProvider

	// GetGrpcSubscribeProvider returns the registered gRPC subscribe provider
	GetGrpcSubscribeProvider() GrpcSubscribeProvider

	// GetTLSConfigProvider returns the registered TLS config provider
	GetTLSConfigProvider() TLSConfigProvider

	// GetLoggerProvider returns the registered logger provider
	GetLoggerProvider() LoggerProvider
}

// DefaultDependencyInjector provides a default implementation of DependencyInjector.
type DefaultDependencyInjector struct {
	grpcServiceProvider   GrpcServiceProvider
	grpcClientProvider    GrpcClientProvider
	grpcSubscribeProvider GrpcSubscribeProvider
	tlsConfigProvider     TLSConfigProvider
	loggerProvider        LoggerProvider
}

// NewDefaultDependencyInjector creates a new default dependency injector.
func NewDefaultDependencyInjector() *DefaultDependencyInjector {
	return &DefaultDependencyInjector{}
}

// RegisterGrpcServiceProvider registers a gRPC service provider.
func (d *DefaultDependencyInjector) RegisterGrpcServiceProvider(provider GrpcServiceProvider) {
	d.grpcServiceProvider = provider
}

// RegisterGrpcClientProvider registers a gRPC client provider.
func (d *DefaultDependencyInjector) RegisterGrpcClientProvider(provider GrpcClientProvider) {
	d.grpcClientProvider = provider
}

// RegisterGrpcSubscribeProvider registers a gRPC subscribe provider.
func (d *DefaultDependencyInjector) RegisterGrpcSubscribeProvider(provider GrpcSubscribeProvider) {
	d.grpcSubscribeProvider = provider
}

// RegisterTLSConfigProvider registers a TLS config provider.
func (d *DefaultDependencyInjector) RegisterTLSConfigProvider(provider TLSConfigProvider) {
	d.tlsConfigProvider = provider
}

// RegisterLoggerProvider registers a logger provider.
func (d *DefaultDependencyInjector) RegisterLoggerProvider(provider LoggerProvider) {
	d.loggerProvider = provider
}

// GetGrpcServiceProvider returns the registered gRPC service provider.
func (d *DefaultDependencyInjector) GetGrpcServiceProvider() GrpcServiceProvider {
	return d.grpcServiceProvider
}

// GetGrpcClientProvider returns the registered gRPC client provider.
func (d *DefaultDependencyInjector) GetGrpcClientProvider() GrpcClientProvider {
	return d.grpcClientProvider
}

// GetGrpcSubscribeProvider returns the registered gRPC subscribe provider.
func (d *DefaultDependencyInjector) GetGrpcSubscribeProvider() GrpcSubscribeProvider {
	return d.grpcSubscribeProvider
}

// GetTLSConfigProvider returns the registered TLS config provider.
func (d *DefaultDependencyInjector) GetTLSConfigProvider() TLSConfigProvider {
	return d.tlsConfigProvider
}

// GetLoggerProvider returns the registered logger provider.
func (d *DefaultDependencyInjector) GetLoggerProvider() LoggerProvider {
	return d.loggerProvider
}
