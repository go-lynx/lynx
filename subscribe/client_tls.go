package subscribe

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/tls/conf"
)

// buildClientTLSConfig builds TLS configuration for gRPC client connections.
// It loads the root CA and returns a tls.Config for verifying upstream server certificates.
// Returns nil if TLS is not enabled.
func (g *GrpcSubscribe) buildClientTLSConfig() (*tls.Config, error) {
	// Check if TLS is enabled, return nil if not enabled
	if !g.tls {
		return nil, nil
	}

	// Create a new certificate pool for storing root certificates
	certPool := x509.NewCertPool()
	var rootCA []byte

	// Check if root CA certificate name is specified
	if g.caName != "" {
		// Requires configProvider to be injected by upper layer
		if g.configProvider == nil {
			return nil, fmt.Errorf("tls: configProvider is nil while caName is set")
		}
		// if group is empty, use the name as the group name.
		// If root CA certificate file group is not specified, use the root CA certificate name as the group name
		if g.caGroup == "" {
			g.caGroup = g.caName
		}
		// Get configuration information through injected provider
		s, err := g.configProvider(g.caName, g.caGroup)
		if err != nil {
			// If getting configuration information fails, return error
			return nil, err
		}
		// Create a new configuration instance and set the configuration source obtained from control plane
		c := config.New(
			config.WithSource(s),
		)
		// Load configuration information
		if err := c.Load(); err != nil {
			// If loading configuration information fails, return error
			return nil, err
		}
		// Define a Cert struct variable for storing certificate information scanned from configuration
		var cert conf.Cert
		// Scan configuration information into Cert struct variable
		if err := c.Scan(&cert); err != nil {
			// If scanning configuration information fails, return error
			return nil, err
		}
		// Convert root CA certificate information obtained from configuration to byte slice
		rootCA = []byte(cert.GetRootCA())
	} else {
		// Use the root certificate of the current application directly
		// If root CA certificate name is not specified, get it through injected defaultRootCA
		if g.defaultRootCA == nil {
			return nil, fmt.Errorf("tls: defaultRootCA provider is nil while caName is empty")
		}
		rootCA = g.defaultRootCA()
	}
	// Add root certificate to certificate pool, trigger panic if addition fails
	if !certPool.AppendCertsFromPEM(rootCA) {
		return nil, fmt.Errorf("failed to load root certificate")
	}
	// Return configured TLS configuration instance, set server name and root certificate pool
	return &tls.Config{ServerName: g.caName, RootCAs: certPool}, nil
}
