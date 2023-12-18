package http

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

func (h *ServiceHttp) tlsLoad() http.ServerOption {
	tlsCert, err := tls.X509KeyPair(app.Lynx().Cert().GetCrt(), app.Lynx().Cert().GetKey())
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(app.Lynx().Cert().GetRootCA()) {
		panic(err)
	}

	return http.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ServerName:   app.Name(),
		ClientAuth:   tls.ClientAuthType(h.conf.GetTlsAuthType()),
	})
}
