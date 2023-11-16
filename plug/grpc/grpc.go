package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
)

var plugName = "grpc"

type ServiceGrpc struct {
	grpc      *grpc.Server
	tls       bool
	serverCrt []byte
	serverKey []byte
	rootCA    []byte
}

type Option func(o *ServiceGrpc)

func EnableTls() Option {
	return func(o *ServiceGrpc) {
		o.tls = true
	}
}

func (g *ServiceGrpc) Weight() int {
	return 500
}

func (g *ServiceGrpc) Name() string {
	return plugName
}

func (g *ServiceGrpc) Load(b *conf.Bootstrap) (plug.Plug, error) {
	boot.GetHelper().Infof("Initializing GRPC service")
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			tracing.Server(tracing.WithTracerName(b.Server.Name)),
			logging.Server(boot.GetLogger()),
			validate.Validator(),
			// Recovery program after exception
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			boot.GrpcRateLimit(b.Server),
		),
	}
	if b.Server.Grpc.Network != "" {
		opts = append(opts, grpc.Network(b.Server.Grpc.Network))
	}
	if b.Server.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(b.Server.Grpc.Addr))
	}
	if b.Server.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(b.Server.Grpc.Timeout.AsDuration()))
	}

	if g.tls {
		err := g.initTls(b)
		if err != nil {
			return nil, err
		}

		cert, err := tls.X509KeyPair(g.serverCrt, g.serverKey)
		if err != nil {
			return nil, err
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(g.rootCA) {
			return nil, err
		}

		opts = append(opts, grpc.TLSConfig(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    certPool,
			ServerName:   b.Server.Name,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}))
	}

	g.grpc = grpc.NewServer(opts...)
	boot.GetHelper().Infof("GRPC service successfully initialized")
	return g, nil
}

func (g *ServiceGrpc) Unload() error {
	boot.GetHelper().Info("message", "Closing the GRPC resources")
	if err := g.grpc.Stop(nil); err != nil {
		boot.GetHelper().Error(err)
	}
	return nil
}

func (g *ServiceGrpc) initTls(b *conf.Bootstrap) error {
	source, err := boot.Polaris().Config(polaris.WithConfigFile(polaris.File{
		Name:  "tls-service.yaml",
		Group: b.Server.Name,
	}))

	if err != nil {
		return err
	}

	c := config.New(
		config.WithSource(source),
	)

	if err := c.Load(); err != nil {
		return err
	}
	var t conf.TlsService
	if err := c.Scan(&t); err != nil {
		return err
	}

	g.serverKey = []byte(t.ServerKey)
	g.rootCA = []byte(t.RootCA)
	g.serverCrt = []byte(t.ServerCrt)

	return nil
}

func GetGRPC() *grpc.Server {
	return boot.GetPlug(plugName).(*ServiceGrpc).grpc
}

func Grpc(opts ...Option) plug.Plug {
	s := ServiceGrpc{
		tls: false,
	}
	for _, option := range opts {
		option(&s)
	}
	return &s
}
