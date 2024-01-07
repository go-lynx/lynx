package subscribe

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	gGrpc "google.golang.org/grpc"
)

type GrpcSubscribe struct {
	name  string
	dis   registry.Discovery
	tls   bool
	rca   string
	group string
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

func WithRootCAFileGroup(group string) Option {
	return func(o *GrpcSubscribe) {
		o.group = group
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
			logging.Client(app.Lynx().Logger()),
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
		app.Lynx().Helper().Error(err)
		panic(err)
	}
	return conn
}
