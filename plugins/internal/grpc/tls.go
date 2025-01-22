package grpc

import (
	"crypto/tls"
	"crypto/x509"
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
	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(app.Lynx().Cert().GetCrt(), app.Lynx().Cert().GetKey())
	if err != nil {
		panic(err)
	}

	// Create certificate pool and add root CA
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(app.Lynx().Cert().GetRootCA()) {
		panic(err)
	}

	// Configure and return TLS settings
	return grpc.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ServerName:   app.GetName(),
		ClientAuth:   tls.ClientAuthType(g.conf.GetTlsAuthType()),
	})
}
