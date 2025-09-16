package adapters

import (
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/interfaces"
	"github.com/go-lynx/lynx/app/log"
)

// GrpcServiceAdapter adapts the app package to the GrpcServiceProvider interface
type GrpcServiceAdapter struct{}

// NewGrpcServiceAdapter creates a new gRPC service adapter
func NewGrpcServiceAdapter() *GrpcServiceAdapter {
	return &GrpcServiceAdapter{}
}

// GetServer returns the gRPC server instance
func (a *GrpcServiceAdapter) GetServer() (*kratosgrpc.Server, error) {
	// This will be implemented by the gRPC plugin
	return nil, nil
}

// GetApplicationName returns the application name
func (a *GrpcServiceAdapter) GetApplicationName() string {
	return app.GetName()
}

// GetLogger returns the logger instance
func (a *GrpcServiceAdapter) GetLogger() interface{} {
	return log.Logger
}

// GetCertificateProvider returns the certificate provider
func (a *GrpcServiceAdapter) GetCertificateProvider() interfaces.CertificateProvider {
	return &CertificateProviderAdapter{}
}

// GetControlPlane returns the control plane interface
func (a *GrpcServiceAdapter) GetControlPlane() interfaces.ControlPlane {
	return &ControlPlaneAdapter{}
}

// CertificateProviderAdapter adapts the app certificate provider
type CertificateProviderAdapter struct{}

// GetCertificate returns the server certificate
func (a *CertificateProviderAdapter) GetCertificate() []byte {
	provider := app.Lynx().Certificate()
	if provider == nil {
		return nil
	}
	return provider.GetCertificate()
}

// GetPrivateKey returns the server private key
func (a *CertificateProviderAdapter) GetPrivateKey() []byte {
	provider := app.Lynx().Certificate()
	if provider == nil {
		return nil
	}
	return provider.GetPrivateKey()
}

// GetRootCA returns the root CA certificate
func (a *CertificateProviderAdapter) GetRootCA() []byte {
	provider := app.Lynx().Certificate()
	if provider == nil {
		return nil
	}
	return provider.GetRootCACertificate()
}

// ControlPlaneAdapter adapts the app control plane
type ControlPlaneAdapter struct{}

// Discovery returns the service discovery instance
func (a *ControlPlaneAdapter) Discovery() interface{} {
	controlPlane := app.Lynx().GetControlPlane()
	if controlPlane == nil {
		return nil
	}
	return controlPlane.NewServiceDiscovery()
}

// GRPCRateLimit returns the gRPC rate limit middleware
func (a *ControlPlaneAdapter) GRPCRateLimit() interface{} {
	controlPlane := app.Lynx().GetControlPlane()
	if controlPlane == nil {
		return nil
	}
	return controlPlane.GRPCRateLimit()
}
