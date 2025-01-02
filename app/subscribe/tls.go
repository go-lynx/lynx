package subscribe

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins/internal/cert/conf"
)

func (g *GrpcSubscribe) tlsLoad() *tls.Config {
	if !g.tls {
		return nil
	}

	certPool := x509.NewCertPool()
	var rootCA []byte

	if g.rca != "" {
		// Obtain the root certificate of the remote file
		if app.Lynx().ControlPlane() == nil {
			return nil
		}
		// if group is empty, use the name as the group name.
		if g.group == "" {
			g.group = g.name
		}
		s, err := app.Lynx().ControlPlane().Config(g.rca, g.group)
		if err != nil {
			panic(err)
		}
		c := config.New(
			config.WithSource(s),
		)
		if err := c.Load(); err != nil {
			panic(err)
		}
		var t conf.Cert
		if err := c.Scan(&t); err != nil {
			panic(err)
		}
		rootCA = []byte(t.GetRootCA())
	} else {
		rootCA = app.Lynx().Cert().GetRootCA()
	}
	// Use the root certificate of the current application directly
	if !certPool.AppendCertsFromPEM(rootCA) {
		panic("Failed to load root certificate")
	}
	return &tls.Config{ServerName: g.name, RootCAs: certPool}
}
