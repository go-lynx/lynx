package http

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
	boot "github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
)

var plugName = "http"

type ServiceHttp struct {
	http   *http.Server
	weight int
}

type Option func(h *ServiceHttp)

func Weight(w int) Option {
	return func(h *ServiceHttp) {
		h.weight = w
	}
}

func (h *ServiceHttp) Name() string {
	return plugName
}

func (h *ServiceHttp) Weight() int {
	return h.weight
}

func (h *ServiceHttp) Load(b *conf.Bootstrap) (plug.Plug, error) {
	boot.GetHelper().Infof("Initializing HTTP service")
	var opts = []http.ServerOption{
		http.Middleware(
			tracing.Server(
				tracing.WithTracerName(b.Server.Name),
			),
			logging.Server(boot.GetLogger()),
			validate.Validator(),
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			boot.HttpRateLimit(b.Server),
			ResponsePack(),
		),
		http.ResponseEncoder(ResponseEncoder),
	}
	if b.Server.Http.Network != "" {
		opts = append(opts, http.Network(b.Server.Http.Network))
	}
	if b.Server.Http.Addr != "" {
		opts = append(opts, http.Address(b.Server.Http.Addr))
	}
	if b.Server.Http.Timeout != nil {
		opts = append(opts, http.Timeout(b.Server.Http.Timeout.AsDuration()))
	}
	h.http = http.NewServer(opts...)
	boot.GetHelper().Infof("HTTP service successfully initialized")
	return h, nil
}

func (h *ServiceHttp) Unload() error {
	boot.GetHelper().Info("message", "Closing the HTTP resources")
	if err := h.http.Close(); err != nil {
		boot.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetHTTP() *http.Server {
	return boot.GetPlug(plugName).(*ServiceHttp).http
}

func Http(opts ...Option) plug.Plug {
	s := &ServiceHttp{
		weight: 600,
	}

	for _, option := range opts {
		option(s)
	}
	return s
}
