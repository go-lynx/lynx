package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/grpc/conf"
	polaris2 "github.com/go-lynx/lynx/plugin/polaris"
	"github.com/go-lynx/lynx/service/limiter"
)

var name = "grpc"

type ServiceGrpc struct {
	grpc      *grpc.Server
	weight    int
	tls       bool
	serverCrt []byte
	serverKey []byte
	rootCA    []byte
}

type Option func(g *ServiceGrpc)

func EnableTls() Option {
	return func(g *ServiceGrpc) {
		g.tls = true
	}
}

func Weight(w int) Option {
	return func(g *ServiceGrpc) {
		g.weight = w
	}
}

func (g *ServiceGrpc) Weight() int {
	return g.weight
}

func (g *ServiceGrpc) Name() string {
	return name
}

func (g *ServiceGrpc) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Grpc)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}
	boot.GetHelper().Infof("Initializing GRPC service")

	var opts = []grpc.ServerOption{
		grpc.Middleware(
			tracing.Server(tracing.WithTracerName(c.Lynx.Application.Name)),
			logging.Server(boot.GetLogger()),
			validate.Validator(),
			// Recovery program after exception
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			limiter.GrpcRateLimit(c.Lynx),
		),
	}

	if c.Network != "" {
		opts = append(opts, grpc.Network(c.Network))
	}
	if c.Addr != "" {
		opts = append(opts, grpc.Address(c.Addr))
	}
	if c.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Timeout.AsDuration()))
	}

	if g.tls {
		err := g.initTls(c)
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
			ServerName:   c.Lynx.Application.Name,
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

func (g *ServiceGrpc) initTls(c *conf.Grpc) error {
	source, err := polaris2.GetPolaris().Config(polaris.WithConfigFile(polaris.File{
		Name:  "tls-service.yaml",
		Group: c.Lynx.Application.Name,
	}))

	if err != nil {
		return err
	}

	sc := config.New(
		config.WithSource(source),
	)

	if err := sc.Load(); err != nil {
		return err
	}
	if err := sc.Scan(&c); err != nil {
		return err
	}

	g.serverKey = []byte(c.Tls.ServerKey)
	g.rootCA = []byte(c.Tls.RootCA)
	g.serverCrt = []byte(c.Tls.ServerCrt)

	return nil
}

func GetGRPC() *grpc.Server {
	return boot.GetPlugin(name).(*ServiceGrpc).grpc
}

func Grpc(opts ...Option) plugin.Plugin {
	s := &ServiceGrpc{
		tls:    false,
		weight: 500,
	}
	for _, option := range opts {
		option(s)
	}
	return s
}
