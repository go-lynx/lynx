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
	if !g.tls {
		return nil, nil
	}

	certPool := x509.NewCertPool()
	var rootCA []byte

	if g.caName != "" {
		// A named CA is loaded from the control plane via the injected provider.
		if g.configProvider == nil {
			return nil, fmt.Errorf("tls: configProvider is nil while caName is set")
		}
		// Default the group to the CA name when no group is given.
		if g.caGroup == "" {
			g.caGroup = g.caName
		}
		s, err := g.configProvider(g.caName, g.caGroup)
		if err != nil {
			return nil, err
		}
		c := config.New(
			config.WithSource(s),
		)
		if err := c.Load(); err != nil {
			return nil, err
		}
		var cert conf.Cert
		if err := c.Scan(&cert); err != nil {
			return nil, err
		}
		rootCA = []byte(cert.GetRootCA())
	} else {
		// No named CA: fall back to the application's own root CA.
		if g.defaultRootCA == nil {
			return nil, fmt.Errorf("tls: defaultRootCA provider is nil while caName is empty")
		}
		rootCA = g.defaultRootCA()
	}
	if !certPool.AppendCertsFromPEM(rootCA) {
		return nil, fmt.Errorf("failed to load root certificate")
	}
	// ServerName must match the upstream service's certificate CN/SAN for verification.
	return &tls.Config{ServerName: g.svcName, RootCAs: certPool}, nil
}
