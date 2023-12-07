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
	"github.com/go-lynx/lynx/conf"
	gGrpc "google.golang.org/grpc"
)

type GrpcSubscribe struct {
	name string
	dis  registry.Discovery
	tls  bool
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
			logging.Client(Lynx().GetLogger()),
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
		Lynx().GetHelper().Error(err)
		panic(err)
	}
	return conn
}

func (g *GrpcSubscribe) tlsLoad() *tls.Config {
	if !g.tls {
		return nil
	}

	if Lynx().ControlPlane() == nil {
		return nil
	}
	s, err := Lynx().ControlPlane().Config("tls-root.yaml", g.name)
	c := config.New(
		config.WithSource(s),
	)
	if err := c.Load(); err != nil {
		panic(err)
	}
	var t conf.Tls
	if err := c.Scan(&t); err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM([]byte(t.GetRootCA())) {
		panic(err)
	}
	return &tls.Config{ServerName: g.name, RootCAs: certPool}
}

func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if Lynx().ControlPlane() == nil {
		return nil
	}
	return Lynx().ControlPlane().NewNodeRouter(g.name)
}
