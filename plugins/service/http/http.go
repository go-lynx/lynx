// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
package http

import (
	"context"
	nhttp "net/http"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
// 插件元数据，定义插件的基本信息
const (
	// pluginName 是 HTTP 服务器插件的唯一标识符，用于在插件系统中识别该插件。
	pluginName = "http.server"

	// pluginVersion 表示 HTTP 服务器插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述了 HTTP 服务器插件的功能。
	pluginDescription = "http server plugin for lynx framework"

	// confPrefix 是加载 HTTP 服务器配置时使用的配置前缀。
	confPrefix = "lynx.http"
)

// ServiceHttp 实现了 Lynx 框架的 HTTP 服务器插件功能。
// 它嵌入了 plugins.BasePlugin 以继承通用的插件功能，并维护 HTTP 服务器的配置和实例。
type ServiceHttp struct {
	// 嵌入基础插件，继承插件的通用属性和方法
	*plugins.BasePlugin
	// HTTP 服务器的配置信息
	conf *conf.Http
	// HTTP 服务器实例
	server *http.Server
}

// NewServiceHttp 创建一个新的 HTTP 服务器插件实例。
// 该函数初始化插件的基础信息，并返回一个指向 ServiceHttp 结构体的指针。
func NewServiceHttp() *ServiceHttp {
	return &ServiceHttp{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件的唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
			// 权重
			10,
		),
	}
}

// InitializeResources 实现了 HTTP 插件的自定义初始化逻辑。
// 该函数会加载并验证 HTTP 服务器的配置，如果配置未提供，则使用默认配置。
func (h *ServiceHttp) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	h.conf = &conf.Http{}

	// 从运行时配置中扫描并加载 HTTP 配置
	err := rt.GetConfig().Value(confPrefix).Scan(h.conf)
	if err != nil {
		return err
	}

	// 设置默认配置
	defaultConf := &conf.Http{
		// 默认网络协议为 TCP
		Network: "tcp",
		// 默认监听地址为 :8080
		Addr: ":8080",
		// 默认不启用 TLS
		TlsEnable: false,
		// 默认超时时间为 10 秒
		Timeout: &durationpb.Duration{Seconds: 10},
	}

	// 对未设置的字段使用默认值
	if h.conf.Network == "" {
		h.conf.Network = defaultConf.Network
	}
	if h.conf.Addr == "" {
		h.conf.Addr = defaultConf.Addr
	}
	if h.conf.Timeout == nil {
		h.conf.Timeout = defaultConf.Timeout
	}

	return nil
}

// StartupTasks 实现了 HTTP 插件的自定义启动逻辑。
// 该函数会配置并启动 HTTP 服务器，添加必要的中间件和配置选项。
func (h *ServiceHttp) StartupTasks() error {
	// 记录 HTTP 服务启动日志
	log.Infof("starting http service")

	var middlewares []middleware.Middleware

	// 添加基础中间件
	middlewares = append(middlewares,
		// 配置链路追踪中间件，设置追踪器名称为应用名称
		tracing.Server(tracing.WithTracerName(app.GetName())),
		// 配置日志中间件，使用 Lynx 框架的日志记录器
		logging.Server(log.Logger),
		// 配置响应包装中间件
		TracerLogPack(),
		// 配置参数验证中间件
		validate.ProtoValidate(),
		// 配置恢复中间件，处理请求处理过程中的 panic
		recovery.Recovery(
			recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
				log.ErrorCtx(ctx, err)
				return nil
			}),
		),
	)

	// 配置限流中间件，使用 Lynx 框架控制平面的 HTTP 限流策略
	// 如果有限流中间件，则追加进去
	if rl := app.Lynx().GetControlPlane().HTTPRateLimit(); rl != nil {
		middlewares = append(middlewares, rl)
	}
	hMiddlewares := http.Middleware(middlewares...)

	// 定义 HTTP 服务器的选项列表
	opts := []http.ServerOption{
		hMiddlewares,
		// 404 方法不存在格式化
		http.NotFoundHandler(nhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(nhttp.StatusNotFound)
			_, _ = w.Write([]byte(`{"code": 404, "message": "404 not found"}`))
			log.Warnf("404 not found path %s", r.URL.Path)
		})),
		// 405 方法不允许处理
		http.MethodNotAllowedHandler(nhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(nhttp.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"code": 405, "message": "method not allowed"}`))
			log.Warnf("405 method not allowed: %s %s", r.Method, r.URL.Path)
		})),
		// 配置响应编码器
		http.ResponseEncoder(ResponseEncoder),
		http.ErrorEncoder(EncodeErrorFunc),
	}

	// 根据配置信息添加额外的服务器选项
	if h.conf.Network != "" {
		// 设置网络协议
		opts = append(opts, http.Network(h.conf.Network))
	}
	if h.conf.Addr != "" {
		// 设置监听地址
		opts = append(opts, http.Address(h.conf.Addr))
	}
	if h.conf.Timeout != nil {
		// 设置超时时间
		opts = append(opts, http.Timeout(h.conf.Timeout.AsDuration()))
	}
	if h.conf.GetTlsEnable() {
		// 如果启用 TLS，添加 TLS 配置选项
		opts = append(opts, h.tlsLoad())
	}

	// 创建 HTTP 服务器实例
	h.server = http.NewServer(opts...)
	// 记录 HTTP 服务启动成功日志
	log.Infof("http service successfully started")
	return nil
}

// CleanupTasks 实现了 HTTP 插件的自定义清理逻辑。
// 该函数会优雅地停止 HTTP 服务器，并处理可能出现的错误。
func (h *ServiceHttp) CleanupTasks() error {
	// 如果服务器实例为空，直接返回 nil
	if h.server == nil {
		return nil
	}
	// 优雅地停止 HTTP 服务器
	if err := h.server.Stop(context.Background()); err != nil {
		// 若停止失败，返回包含错误信息的插件错误
		return plugins.NewPluginError(h.ID(), "Stop", "Failed to stop HTTP server", err)
	}
	return nil
}

// Configure 更新 HTTP 服务器的配置。
// 该函数接收一个任意类型的参数，尝试将其转换为 *conf.Http 类型，如果转换成功则更新配置。
func (h *ServiceHttp) Configure(c any) error {
	// 尝试将传入的配置转换为 *conf.Http 类型
	if httpConf, ok := c.(*conf.Http); ok {
		// 转换成功，更新配置
		h.conf = httpConf
		return nil
	}
	// 转换失败，返回配置无效错误
	return plugins.ErrInvalidConfiguration
}

// CheckHealth 对 HTTP 服务器进行健康检查。
// 该函数目前直接返回 nil，表示服务器健康，可根据实际需求添加检查逻辑。
func (h *ServiceHttp) CheckHealth() error {
	return nil
}
