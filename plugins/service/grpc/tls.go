package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/grpc"
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
// certificateProvider is a local interface matching the application's provider.
// It intentionally mirrors method names without importing the app/interfaces package
// to avoid cross-module import issues when plugins are built as separate modules.
type certificateProvider interface {
    GetCertificate() []byte
    GetPrivateKey() []byte
    // Root CA method name in adapters: GetRootCA
    GetRootCA() []byte
}

func (g *Service) tlsLoad() (grpc.ServerOption, error) {
	// Load the X.509 certificate and private key pair from the application certificate provider
	provider := g.getCertProvider()
	if provider == nil {
		return nil, fmt.Errorf("certificate provider not configured")
	}
	cp, ok := provider.(certificateProvider)
	if !ok {
		return nil, fmt.Errorf("invalid certificate provider type: %T", provider)
	}

	certPEM := cp.GetCertificate()
	keyPEM := cp.GetPrivateKey()
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, fmt.Errorf("server certificate or private key not provided")
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X509 key pair: %w", err)
	}

	// Root CA for optional mTLS
	var certPool *x509.CertPool
	if caPEM := cp.GetRootCA(); len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to append root CA certificate to pool")
		}
		certPool = pool
	}

	// Enforce secure defaults
	tlsConf := &tls.Config{
		Certificates:             []tls.Certificate{tlsCert},
		ClientCAs:                certPool,
		ClientAuth:               tls.ClientAuthType(g.conf.GetTlsAuthType()),
		MinVersion:               tls.VersionTLS12,
		MaxVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		// Forward secrecy suites
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	return grpc.TLSConfig(tlsConf), nil
}
