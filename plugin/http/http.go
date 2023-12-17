package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/http/conf"
)

var (
	name       = "http"
	confPrefix = "lynx.http"
)

type ServiceHttp struct {
	http   *http.Server
	conf   *conf.Http
	weight int
}

type Option func(h *ServiceHttp)

func Weight(w int) Option {
	return func(h *ServiceHttp) {
		h.weight = w
	}
}

func Config(c *conf.Http) Option {
	return func(h *ServiceHttp) {
		h.conf = c
	}
}

func (h *ServiceHttp) Name() string {
	return name
}

func (h *ServiceHttp) DependsOn() []string {
	return nil
}

func (h *ServiceHttp) ConfPrefix() string {
	return confPrefix
}

func (h *ServiceHttp) Weight() int {
	return h.weight
}

func (h *ServiceHttp) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(h.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing HTTP service")

	var opts = []http.ServerOption{
		http.Middleware(
			tracing.Server(tracing.WithTracerName(app.Name())),
			logging.Server(app.Lynx().Logger()),
			app.Lynx().ControlPlane().HttpRateLimit(),
			validate.Validator(),
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			ResponsePack(),
		),
		http.ResponseEncoder(ResponseEncoder),
	}

	if h.conf.Network != "" {
		opts = append(opts, http.Network(h.conf.Network))
	}
	if h.conf.Addr != "" {
		opts = append(opts, http.Address(h.conf.Addr))
	}
	if h.conf.Timeout != nil {
		opts = append(opts, http.Timeout(h.conf.Timeout.AsDuration()))
	}
	if h.conf.GetTls() {
		opts = append(opts, h.tlsLoad())
	}

	h.http = http.NewServer(opts...)
	app.Lynx().Helper().Infof("HTTP service successfully initialized")
	return h, nil
}

func (h *ServiceHttp) tlsLoad() http.ServerOption {
	cert, err := tls.X509KeyPair(app.Lynx().Cert().GetCrt(), app.Lynx().Cert().GetKey())
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(app.Lynx().Cert().GetRootCA()) {
		panic(err)
	}

	return http.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
		ServerName:   app.Name(),
		ClientAuth:   tls.ClientAuthType(h.conf.GetTlsAuthType()),
	})
}

func (h *ServiceHttp) Unload() error {
	if h.http == nil {
		return nil
	}
	if err := h.http.Close(); err != nil {
		app.Lynx().Helper().Error(err)
		return err
	}
	app.Lynx().Helper().Info("message", "Closing the HTTP resources")
	return nil
}

func Http(opts ...Option) plugin.Plugin {
	s := &ServiceHttp{
		weight: 600,
		conf:   &conf.Http{},
	}

	for _, option := range opts {
		option(s)
	}
	return s
}
