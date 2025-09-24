package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
)

// tlsLoad creates and configures TLS settings for the gRPC server.
// It performs the following operations:
//   - Loads the X.509 certificate and private key pair
//   - Creates a certificate pool and adds the root CA certificate
//   - Configures TLS settings including client authentication type
//
// Returns:
//   - grpc.ServerOption: A configured TLS option for the gRPC server
//   - error: Any error that occurred during TLS configuration
func (g *Service) tlsLoad() (grpc.ServerOption, error) {
	// Prefer injected provider; fallback to app-level certificate provider
	var cp app.CertificateProvider
	if prov := g.getCertProvider(); prov != nil {
		if typed, ok := prov.(app.CertificateProvider); ok {
			cp = typed
		}
	}
	if cp == nil {
		cp = app.Lynx().Certificate()
	}
	if cp == nil {
		return nil, fmt.Errorf("certificate provider not configured")
	}

	if len(cp.GetCertificate()) == 0 {
		return nil, fmt.Errorf("certificate data is empty")
	}
	if len(cp.GetPrivateKey()) == 0 {
		return nil, fmt.Errorf("private key data is empty")
	}

	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(cp.GetCertificate(), cp.GetPrivateKey())
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
	}

	// Create certificate pool and add root CA if provided
	var certPool *x509.CertPool
	rootCA := cp.GetRootCACertificate()
	if len(rootCA) > 0 {
		certPool = x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(rootCA) {
			log.Warnf("Failed to append root CA certificate to pool, continuing without client certificate verification")
			certPool = nil
		} else {
			log.Infof("Root CA certificate successfully added to certificate pool")
		}
	} else {
		log.Warnf("No root CA certificate provided, client certificate verification may be disabled")
	}

	// Configure the TLS settings for the gRPC server.
	cfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ServerName:   g.getAppName(),
		ClientAuth:   tls.ClientAuthType(g.conf.GetTlsAuthType()),
	}
	if certPool != nil {
		cfg.ClientCAs = certPool
	}
	return grpc.TLSConfig(cfg), nil
}
