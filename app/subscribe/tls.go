package subscribe

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/tls/conf"
)

// tlsLoad method is used to load TLS configuration. Returns nil if TLS is not enabled.
// This method attempts to obtain the root certificate and add it to the certificate pool, ultimately returning a configured tls.Config instance.
func (g *GrpcSubscribe) tlsLoad() *tls.Config {
	// Check if TLS is enabled, return nil if not enabled
	if !g.tls {
		return nil
	}

	// Create a new certificate pool for storing root certificates
	certPool := x509.NewCertPool()
	var rootCA []byte

	// Check if root CA certificate name is specified
	if g.caName != "" {
		// Requires configProvider to be injected by upper layer
		if g.configProvider == nil {
			panic("tls: configProvider is nil while caName is set")
		}
		// if group is empty, use the name as the group name.
		// If root CA certificate file group is not specified, use the root CA certificate name as the group name
		if g.caGroup == "" {
			g.caGroup = g.caName
		}
		// Get configuration information through injected provider
		s, err := g.configProvider(g.caName, g.caGroup)
		if err != nil {
			// If getting configuration information fails, trigger panic
			panic(err)
		}
		// Create a new configuration instance and set the configuration source obtained from control plane
		c := config.New(
			config.WithSource(s),
		)
		// Load configuration information
		if err := c.Load(); err != nil {
			// If loading configuration information fails, trigger panic
			panic(err)
		}
		// Define a Cert struct variable for storing certificate information scanned from configuration
		var t conf.Cert
		// Scan configuration information into Cert struct variable
		if err := c.Scan(&t); err != nil {
			// If scanning configuration information fails, trigger panic
			panic(err)
		}
		// Convert root CA certificate information obtained from configuration to byte slice
		rootCA = []byte(t.GetRootCA())
	} else {
		// Use the root certificate of the current application directly
		// If root CA certificate name is not specified, get it through injected defaultRootCA
		if g.defaultRootCA == nil {
			panic("tls: defaultRootCA provider is nil while caName is empty")
		}
		rootCA = g.defaultRootCA()
	}
	// Add root certificate to certificate pool, trigger panic if addition fails
	if !certPool.AppendCertsFromPEM(rootCA) {
		panic("Failed to load root certificate")
	}
	// Return configured TLS configuration instance, set server name and root certificate pool
	return &tls.Config{ServerName: g.caName, RootCAs: certPool}
}
