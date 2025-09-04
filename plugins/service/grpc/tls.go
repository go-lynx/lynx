package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/grpc"
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
//   - grpc.ServerOption: A configured TLS option for the gRPC server
func (g *ServiceGrpc) tlsLoad() grpc.ServerOption {
	// Load the X.509 certificate and private key pair from the paths provided by the application.
	// Get the certificate provider
	certProvider := app.Lynx().Certificate()
	if certProvider == nil {
		panic("certificate provider not configured")
	}

	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(certProvider.GetCertificate(), certProvider.GetPrivateKey())
	if err != nil {
		// If there is an error loading the certificate and key pair, panic with the error
		panic(fmt.Errorf("failed to load X509 key pair: %v", err))
	}

	// Create a new certificate pool to hold trusted root CA certificates
	certPool := x509.NewCertPool()

	// Attempt to add the root CA certificate (in PEM format) to the certificate pool
	if !certPool.AppendCertsFromPEM(certProvider.GetRootCACertificate()) {
		panic("failed to append root CA certificate to pool")
	}

	// Configure the TLS settings for the gRPC server.
	// Certificates: Set the server's certificate and private key pair.
	// ClientCAs: Set the certificate pool containing trusted root CA certificates for client authentication.
	// ServerName: Set the server name, which is retrieved from the application configuration.
	// ClientAuth: Set the client authentication type based on the configuration.
	return grpc.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ServerName:   app.GetName(),
		ClientAuth:   tls.ClientAuthType(g.conf.GetTlsAuthType()),
	})
}
