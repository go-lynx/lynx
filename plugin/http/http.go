package http

import (
	"context"
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

func (h *ServiceHttp) Load(b config.Value) (plugin.Plugin, error) {
	// 从配置值 b 中扫描并解析 HTTP 服务的配置到 h.conf 中。
	err := b.Scan(h.conf)
	// 如果发生错误，返回 nil 和错误信息。
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录 HTTP 服务初始化的信息。
	app.Lynx().Helper().Infof("Initializing HTTP service")

	// 定义一个 HTTP 服务器选项切片，用于配置 HTTP 服务器。
	var opts = []http.ServerOption{
		// 使用中间件进行追踪，设置追踪器名称为应用名称。
		http.Middleware(
			tracing.Server(tracing.WithTracerName(app.Name())),
			// 使用日志中间件，记录 HTTP 请求和响应的日志。
			logging.Server(app.Lynx().Logger()),
			// 使用 HTTP 限流中间件，限制 HTTP 请求的速率。
			app.Lynx().ControlPlane().HttpRateLimit(),
			// 使用验证中间件，对请求数据进行验证。
			validate.Validator(),
			// 使用恢复中间件，捕获并处理 panic。
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			// 使用自定义的响应包装中间件。
			ResponsePack(),
		),
		// 设置自定义的响应编码器。
		http.ResponseEncoder(ResponseEncoder),
	}

	// 如果配置中指定了网络类型，则将其添加到 HTTP 服务器选项中。
	if h.conf.Network != "" {
		opts = append(opts, http.Network(h.conf.Network))
	}
	// 如果配置中指定了地址，则将其添加到 HTTP 服务器选项中。
	if h.conf.Addr != "" {
		opts = append(opts, http.Address(h.conf.Addr))
	}
	// 如果配置中指定了超时时间，则将其添加到 HTTP 服务器选项中。
	if h.conf.Timeout != nil {
		opts = append(opts, http.Timeout(h.conf.Timeout.AsDuration()))
	}
	// 如果配置中启用了 TLS，则加载 TLS 配置并将其添加到 HTTP 服务器选项中。
	if h.conf.GetTls() {
		opts = append(opts, h.tlsLoad())
	}

	// 创建一个新的 HTTP 服务器实例，使用之前定义的选项进行配置。
	h.http = http.NewServer(opts...)
	// 使用 Lynx 应用的 Helper 记录 HTTP 服务初始化成功的信息。
	app.Lynx().Helper().Infof("HTTP service successfully initialized")
	// 返回 HTTP 服务实例和 nil 错误，表示加载成功。
	return h, nil
}

// Unload 方法用于停止并关闭 HTTP 服务器。
func (h *ServiceHttp) Unload() error {
	// 检查 HTTP 服务器实例是否存在，如果不存在则直接返回 nil。
	if h.http == nil {
		return nil
	}
	// 调用 HTTP 服务器的 Close 方法来停止服务器，并传入一个 nil 参数。
	// 如果 Close 方法返回错误，则记录错误信息。
	if err := h.http.Close(); err != nil {
		// 使用 app.Lynx().Helper() 记录错误信息。
		app.Lynx().Helper().Error(err)
		return err
	}
	// 记录一条信息，指示 HTTP 资源正在被关闭。
	app.Lynx().Helper().Info("message", "Closing the HTTP resources")
	// 返回 nil，表示卸载过程成功，没有发生错误。
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
