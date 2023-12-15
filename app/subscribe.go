package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/plugin/cert/conf"
	gGrpc "google.golang.org/grpc"
)

type GrpcSubscribe struct {
	name string
	dis  registry.Discovery
	tls  bool
	rca  string
}

type Option func(o *GrpcSubscribe)

func WithServiceName(name string) Option {
	return func(o *GrpcSubscribe) {
		o.name = name
	}
}

func WithDiscovery(dis registry.Discovery) Option {
	return func(o *GrpcSubscribe) {
		o.dis = dis
	}
}

func EnableTls() Option {
	return func(o *GrpcSubscribe) {
		o.tls = true
	}
}

func WithRootCAFileName(rca string) Option {
	return func(o *GrpcSubscribe) {
		o.rca = rca
	}
}

func NewGrpcSubscribe(opts ...Option) *GrpcSubscribe {
	gs := &GrpcSubscribe{
		tls: false,
	}
	for _, o := range opts {
		o(gs)
	}
	return gs
}

// Subscribe subscribe GRPC service
func (g *GrpcSubscribe) Subscribe() *gGrpc.ClientConn {
	if g.name == "" {
		return nil
	}
	opts := []grpc.ClientOption{
		grpc.WithEndpoint("discovery:///" + g.name),
		grpc.WithDiscovery(g.dis),
		grpc.WithMiddleware(
			logging.Client(Lynx().Logger()),
			tracing.Client(),
		),
		grpc.WithTLSConfig(g.tlsLoad()),
		grpc.WithNodeFilter(g.nodeFilter()),
	}
	var conn *gGrpc.ClientConn
	var err error
	if g.tls {
		conn, err = grpc.Dial(context.Background(), opts...)
	} else {
		conn, err = grpc.DialInsecure(context.Background(), opts...)
	}
	if err != nil {
		Lynx().Helper().Error(err)
		panic(err)
	}
	return conn
}

func (g *GrpcSubscribe) tlsLoad() *tls.Config {
	if !g.tls {
		return nil
	}

	certPool := x509.NewCertPool()
	var rootCA []byte

	if g.rca != "" {
		// Obtain the root certificate of the remote file
		if Lynx().ControlPlane() == nil {
			return nil
		}
		s, err := Lynx().ControlPlane().Config(g.rca, g.name)
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
		rootCA = Lynx().cert.RootCA()
	}
	// Use the root certificate of the current application directly
	if !certPool.AppendCertsFromPEM(rootCA) {
		panic("Failed to load root certificate")
	}
	return &tls.Config{ServerName: g.name, RootCAs: certPool}
}

func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if Lynx().ControlPlane() == nil {
		return nil
	}
	return Lynx().ControlPlane().NewNodeRouter(g.name)
}
