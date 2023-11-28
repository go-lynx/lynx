package http

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/limiter"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/http/conf"
)

var name = "http"

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
	return name
}

func (h *ServiceHttp) Weight() int {
	return h.weight
}

func (h *ServiceHttp) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Http)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	app.GetHelper().Infof("Initializing HTTP service")

	var opts = []http.ServerOption{
		http.Middleware(
			tracing.Server(
				tracing.WithTracerName(c.Lynx.Application.Name),
			),
			logging.Server(app.GetLogger()),
			validate.Validator(),
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			limiter.HttpRateLimit(c.Lynx),
			ResponsePack(),
		),
		http.ResponseEncoder(ResponseEncoder),
	}

	if c.Network != "" {
		opts = append(opts, http.Network(c.Network))
	}
	if c.Addr != "" {
		opts = append(opts, http.Address(c.Addr))
	}
	if c.Timeout != nil {
		opts = append(opts, http.Timeout(c.Timeout.AsDuration()))
	}

	h.http = http.NewServer(opts...)
	app.GetHelper().Infof("HTTP service successfully initialized")
	return h, nil
}

func (h *ServiceHttp) Unload() error {
	app.GetHelper().Info("message", "Closing the HTTP resources")
	if err := h.http.Close(); err != nil {
		app.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetHTTP() *http.Server {
	return boot.GetPlugin(name).(*ServiceHttp).http
}

func Http(opts ...Option) plugin.Plugin {
	s := &ServiceHttp{
		weight: 600,
	}

	for _, option := range opts {
		option(s)
	}
	return s
}
