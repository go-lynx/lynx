package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/grpc/conf"
)

var (
	name       = "grpc"
	confPrefix = "lynx.grpc"
)

type ServiceGrpc struct {
	grpc   *grpc.Server
	conf   *conf.Grpc
	weight int
}

type Option func(g *ServiceGrpc)

func Weight(w int) Option {
	return func(g *ServiceGrpc) {
		g.weight = w
	}
}

func Config(c *conf.Grpc) Option {
	return func(g *ServiceGrpc) {
		g.conf = c
	}
}

func (g *ServiceGrpc) Weight() int {
	return g.weight
}

func (g *ServiceGrpc) Name() string {
	return name
}

func (g *ServiceGrpc) ConfPrefix() string {
	return confPrefix
}

func (g *ServiceGrpc) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(g.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing GRPC service")

	opts := []grpc.ServerOption{
		grpc.Middleware(
			tracing.Server(tracing.WithTracerName(app.Name())),
			logging.Server(app.Lynx().Logger()),
			app.Lynx().ControlPlane().GrpcRateLimit(),
			validate.Validator(),
			// Recovery program after exception
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
		),
	}

	if g.conf.Network != "" {
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	if g.conf.Addr != "" {
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	if g.conf.Timeout != nil {
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	if g.conf.GetTls() {
		opts = append(opts, g.tlsLoad())
	}

	g.grpc = grpc.NewServer(opts...)
	app.Lynx().Helper().Infof("GRPC service successfully initialized")
	return g, nil
}

func (g *ServiceGrpc) tlsLoad() grpc.ServerOption {
	cert, err := tls.X509KeyPair(app.Lynx().Cert().Crt(), app.Lynx().Cert().Key())
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(app.Lynx().Cert().RootCA()) {
		panic(err)
	}

	return grpc.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
		ServerName:   app.Name(),
		ClientAuth:   tls.ClientAuthType(g.conf.GetTlsAuthType()),
	})
}

func (g *ServiceGrpc) Unload() error {
	if g.grpc == nil {
		return nil
	}
	if err := g.grpc.Stop(nil); err != nil {
		app.Lynx().Helper().Error(err)
	}
	app.Lynx().Helper().Info("message", "Closing the GRPC resources")
	return nil
}

func Grpc(opts ...Option) plugin.Plugin {
	s := &ServiceGrpc{
		weight: 500,
		conf:   &conf.Grpc{},
	}
	for _, option := range opts {
		option(s)
	}
	return s
}
