package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
)

// tlsLoad creates and configures TLS settings for the HTTP server.
// It performs the following operations:
//   - Loads the X.509 certificate and private key pair
//   - Creates a certificate pool and adds the root CA certificate
//   - Configures TLS settings including client authentication type
//
// Returns:
//   - http.ServerOption: A configured TLS option for the HTTP server
//   - error: Any error that occurred during TLS configuration
func (h *ServiceHttp) tlsLoad() (http.ServerOption, error) {
	// Get the certificate provider
	certProvider := app.Lynx().Certificate()
	if certProvider == nil {
		return nil, fmt.Errorf("certificate provider not configured")
	}

	// Validate certificate provider has required data
	if len(certProvider.GetCertificate()) == 0 {
		return nil, fmt.Errorf("certificate data is empty")
	}
	if len(certProvider.GetPrivateKey()) == 0 {
		return nil, fmt.Errorf("private key data is empty")
	}

	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(certProvider.GetCertificate(), certProvider.GetPrivateKey())
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
	}

	// Create certificate pool and add root CA
	certPool := x509.NewCertPool()
	hasClientCAs := false
	rootCACert := certProvider.GetRootCACertificate()
	if len(rootCACert) > 0 {
		if !certPool.AppendCertsFromPEM(rootCACert) {
			log.Warnf("Failed to append root CA certificate to pool, continuing without client certificate verification")
		} else {
			hasClientCAs = true
			log.Infof("Root CA certificate successfully added to certificate pool")
		}
	} else {
		log.Warnf("No root CA certificate provided, client certificate verification will be disabled")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ServerName:   app.GetName(),
		ClientAuth:   tls.ClientAuthType(h.conf.GetTlsAuthType()),
	}

	// Only set ClientCAs if we have a valid certificate pool
	if hasClientCAs {
		tlsConfig.ClientCAs = certPool
	}

	log.Infof("TLS configuration created successfully with client auth type: %d", h.conf.GetTlsAuthType())
	return http.TLSConfig(tlsConfig), nil
}
