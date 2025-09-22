package interfaces

import (
	"crypto/tls"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
)

// GrpcServiceProvider defines the interface for gRPC service operations
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

// GrpcClientProvider defines the interface for gRPC client operations
type GrpcClientProvider interface {
	// GetClientPlugin returns the gRPC client plugin
	GetClientPlugin() (GrpcClientPlugin, error)

	// GetClientConnection returns a gRPC client connection for a service
	GetClientConnection(serviceName string) (*grpc.ClientConn, error)

	// CreateClientConnection creates a new gRPC client connection with custom config
	CreateClientConnection(config GrpcClientConfig) (*grpc.ClientConn, error)
}

// GrpcClientPlugin defines the interface for gRPC client plugin
type GrpcClientPlugin interface {
	// GetConnection returns a gRPC client connection for the specified service
	GetConnection(serviceName string) (*grpc.ClientConn, error)

	// CreateConnection creates a new gRPC client connection
	CreateConnection(config GrpcClientConfig) (*grpc.ClientConn, error)
}

// GrpcClientConfig represents configuration for a gRPC client connection
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

// CertificateProvider defines the interface for certificate operations
type CertificateProvider interface {
	// GetCertificate returns the server certificate
	GetCertificate() []byte

	// GetPrivateKey returns the server private key
	GetPrivateKey() []byte

	// GetRootCA returns the root CA certificate
	GetRootCA() []byte
}

// ControlPlane defines the interface for control plane operations
type ControlPlane interface {
	// Discovery returns the service discovery instance
	Discovery() interface{}

	// GRPCRateLimit returns the gRPC rate limit middleware
	GRPCRateLimit() interface{}
}

// GrpcSubscribeProvider defines the interface for gRPC subscription operations
type GrpcSubscribeProvider interface {
	// BuildGrpcSubscriptions builds gRPC subscription connections
	BuildGrpcSubscriptions(cfg interface{}, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*grpc.ClientConn, error)

	// GetGrpcConnection gets a gRPC connection for a specific service
	GetGrpcConnection(serviceName string) (*grpc.ClientConn, error)

	// CloseGrpcConnection closes a gRPC connection for a specific service
	CloseGrpcConnection(serviceName string) error
}

// TLSConfigProvider defines the interface for TLS configuration
type TLSConfigProvider interface {
	// BuildTLSConfig builds TLS configuration for gRPC
	BuildTLSConfig(tlsEnabled bool, authType int32) (*tls.Config, error)
}

// LoggerProvider defines the interface for logging operations
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
