package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// tlsLoad creates and configures TLS settings for the gRPC server.
// It performs the following operations:
//   - Loads the X.509 certificate and private key pair
//   - Creates a certificate pool and adds the root CA certificate
//   - Configures TLS settings including client authentication type
//
// The method will panic if:
//   - The certificate and key pair cannot be loaded
//   - The root CA certificate cannot be added to the certificate pool
//
// Returns:
//   - grpc.ServerOption: A configured TLS option for the Http server
func (h *ServiceHttp) tlsLoad() http.ServerOption {
	// Get the certificate provider
	certProvider := app.Lynx().Certificate()
	if certProvider == nil {
		panic("certificate provider not configured")
	}

	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(certProvider.GetCertificate(), certProvider.GetPrivateKey())
	if err != nil {
		panic(fmt.Errorf("failed to load X509 key pair: %v", err))
	}

	// Create certificate pool and add root CA
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certProvider.GetRootCACertificate()) {
		panic("failed to append root CA certificate to pool")
	}

	return http.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ServerName:   app.GetName(),
		ClientAuth:   tls.ClientAuthType(h.conf.GetTlsAuthType()),
	})
}
