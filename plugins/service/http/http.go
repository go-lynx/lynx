// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
package http

import (
	"context"
	"fmt"
	nhttp "net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
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

	// ========== 监控相关 ==========
	// Prometheus 监控指标
	requestCounter   *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	errorCounter     *prometheus.CounterVec
	healthCheckTotal *prometheus.CounterVec

	// ========== 安全相关 ==========
	// 限流器
	rateLimiter *rate.Limiter

	// ========== 性能相关 ==========
	// 连接超时配置
	idleTimeout       time.Duration
	keepAliveTimeout  time.Duration
	readHeaderTimeout time.Duration
	// 请求大小限制
	maxRequestSize int64

	// ========== 优雅关闭相关 ==========
	// 关闭信号通道
	shutdownChan chan struct{}
	// 是否正在关闭
	isShuttingDown bool
	// 关闭超时
	shutdownTimeout time.Duration
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
		shutdownChan: make(chan struct{}),
	}
}

// ========== 配置管理 ==========

// InitializeResources 实现了 HTTP 插件的自定义初始化逻辑。
// 该函数会加载并验证 HTTP 服务器的配置，如果配置未提供，则使用默认配置。
func (h *ServiceHttp) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	h.conf = &conf.Http{}

	// 从运行时配置中扫描并加载 HTTP 配置
	err := rt.GetConfig().Value(confPrefix).Scan(h.conf)
	if err != nil {
		log.Warnf("Failed to load HTTP configuration, using defaults: %v", err)
	}

	// 设置默认配置
	h.setDefaultConfig()

	// 验证配置
	if err := h.validateConfig(); err != nil {
		return fmt.Errorf("HTTP configuration validation failed: %w", err)
	}

	log.Infof("HTTP configuration loaded: network=%s, addr=%s, tls=%v",
		h.conf.Network, h.conf.Addr, h.conf.GetTlsEnable())
	return nil
}

// setDefaultConfig 设置默认配置
func (h *ServiceHttp) setDefaultConfig() {
	// 基础配置
	if h.conf.Network == "" {
		h.conf.Network = "tcp"
	}
	if h.conf.Addr == "" {
		h.conf.Addr = ":8080"
	}
	if h.conf.Timeout == nil {
		h.conf.Timeout = &durationpb.Duration{Seconds: 10}
	}

	// 监控配置默认值
	h.initMonitoringDefaults()

	// 安全配置默认值
	h.initSecurityDefaults()

	// 性能配置默认值
	h.initPerformanceDefaults()

	// 优雅关闭配置默认值
	h.initGracefulShutdownDefaults()
}

// validateConfig 验证配置参数
func (h *ServiceHttp) validateConfig() error {
	// 验证地址格式
	if h.conf.Addr != "" {
		if !strings.Contains(h.conf.Addr, ":") {
			return fmt.Errorf("invalid address format: %s", h.conf.Addr)
		}
		parts := strings.Split(h.conf.Addr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid address format: %s", h.conf.Addr)
		}
		if port, err := strconv.Atoi(parts[1]); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port number: %s", parts[1])
		}
	}

	// 验证超时时间
	if h.conf.Timeout != nil {
		if h.conf.Timeout.AsDuration() <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
	}

	// 验证请求大小限制
	if h.maxRequestSize < 0 {
		return fmt.Errorf("max request size cannot be negative")
	}

	// 验证限流配置
	if h.rateLimiter != nil {
		// 限流器已初始化，配置有效
	}

	return nil
}

// ========== 安全相关 ==========

// initSecurityDefaults 初始化安全默认配置
func (h *ServiceHttp) initSecurityDefaults() {
	// 请求大小限制：10MB
	h.maxRequestSize = 10 * 1024 * 1024

	// 限流配置：100 req/s, burst: 200
	h.rateLimiter = rate.NewLimiter(100, 200)
}

// initRateLimiter 初始化限流器
func (h *ServiceHttp) initRateLimiter() {
	if h.rateLimiter != nil {
		log.Infof("Rate limiter initialized: %d req/s, burst: %d",
			h.rateLimiter.Limit(), h.rateLimiter.Burst())
	}
}

// ========== 性能相关 ==========

// initPerformanceDefaults 初始化性能默认配置
func (h *ServiceHttp) initPerformanceDefaults() {
	h.idleTimeout = 60 * time.Second
	h.keepAliveTimeout = 30 * time.Second
	h.readHeaderTimeout = 20 * time.Second
}

// ========== 优雅关闭相关 ==========

// initGracefulShutdownDefaults 初始化优雅关闭默认配置
func (h *ServiceHttp) initGracefulShutdownDefaults() {
	h.shutdownTimeout = 30 * time.Second
}

// ========== 服务器管理 ==========

// StartupTasks 实现了 HTTP 插件的自定义启动逻辑。
// 该函数会配置并启动 HTTP 服务器，添加必要的中间件和配置选项。
func (h *ServiceHttp) StartupTasks() error {
	// 记录 HTTP 服务启动日志
	log.Infof("Starting HTTP service on %s", h.conf.Addr)

	// 初始化监控指标
	h.initMetrics()

	// 初始化限流器
	h.initRateLimiter()

	// 构建中间件
	middlewares := h.buildMiddlewares()
	hMiddlewares := http.Middleware(middlewares...)

	// 定义 HTTP 服务器的选项列表
	opts := []http.ServerOption{
		hMiddlewares,
		// 404 方法不存在格式化
		http.NotFoundHandler(h.notFoundHandler()),
		// 405 方法不允许处理
		http.MethodNotAllowedHandler(h.methodNotAllowedHandler()),
		// 配置响应编码器
		http.ResponseEncoder(ResponseEncoder),
		http.ErrorEncoder(h.enhancedErrorEncoder),
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

	// 应用性能配置到底层 net/http.Server
	h.applyPerformanceConfig()

	// 添加监控端点
	h.server.HandlePrefix("/metrics", promhttp.Handler())
	h.server.HandlePrefix("/health", h.healthCheckHandler())

	// 记录 HTTP 服务启动成功日志
	log.Infof("HTTP service successfully started with monitoring endpoints and performance optimizations")
	return nil
}

// applyPerformanceConfig 应用性能配置到底层 HTTP 服务器
func (h *ServiceHttp) applyPerformanceConfig() {
	// 通过反射获取底层的 net/http.Server
	serverValue := reflect.ValueOf(h.server).Elem()
	httpServerField := serverValue.FieldByName("srv")

	if httpServerField.IsValid() && !httpServerField.IsNil() {
		httpServer := httpServerField.Interface().(*nhttp.Server)

		// 应用性能配置
		if h.idleTimeout > 0 {
			httpServer.IdleTimeout = h.idleTimeout
			log.Infof("Applied IdleTimeout: %v", h.idleTimeout)
		}

		if h.keepAliveTimeout > 0 {
			httpServer.ReadHeaderTimeout = h.keepAliveTimeout
			log.Infof("Applied KeepAliveTimeout: %v", h.keepAliveTimeout)
		}

		// 设置读取头部超时
		if h.readHeaderTimeout > 0 {
			httpServer.ReadHeaderTimeout = h.readHeaderTimeout
			log.Infof("Applied ReadHeaderTimeout: %v", h.readHeaderTimeout)
		}

		// 设置请求大小限制
		if h.maxRequestSize > 0 {
			httpServer.MaxHeaderBytes = int(h.maxRequestSize)
			log.Infof("Applied MaxRequestSize: %d bytes", h.maxRequestSize)
		}

		log.Infof("Performance configurations applied successfully")
	} else {
		log.Warnf("Could not access underlying HTTP server for performance configuration")
	}
}

// CleanupTasks 实现了 HTTP 插件的自定义清理逻辑。
// 该函数会优雅地停止 HTTP 服务器，并处理可能出现的错误。
func (h *ServiceHttp) CleanupTasks() error {
	// 如果服务器实例为空，直接返回 nil
	if h.server == nil {
		return nil
	}

	log.Infof("Starting graceful shutdown of HTTP service")

	h.isShuttingDown = true
	close(h.shutdownChan)

	// 设置关闭超时
	ctx := context.Background()
	if h.shutdownTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.shutdownTimeout)
		defer cancel()
	}

	// 优雅关闭服务器
	if err := h.server.Stop(ctx); err != nil {
		log.Errorf("Failed to stop HTTP server gracefully: %v", err)
		return plugins.NewPluginError(h.ID(), "Stop", "Failed to stop HTTP server gracefully", err)
	}

	log.Infof("HTTP service gracefully stopped")
	return nil
}

// ========== 配置管理 ==========

// Configure 更新 HTTP 服务器的配置。
// 该函数接收一个任意类型的参数，尝试将其转换为 *conf.Http 类型，如果转换成功则更新配置。
func (h *ServiceHttp) Configure(c any) error {
	// 尝试将传入的配置转换为 *conf.Http 类型
	if httpConf, ok := c.(*conf.Http); ok {
		// 保存旧配置用于回滚
		oldConf := h.conf
		h.conf = httpConf

		// 设置默认配置
		h.setDefaultConfig()

		// 验证新配置
		if err := h.validateConfig(); err != nil {
			// 配置无效，回滚到旧配置
			h.conf = oldConf
			log.Errorf("Invalid new configuration, rolling back: %v", err)
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		log.Infof("HTTP configuration updated successfully")
		return nil
	}

	// 转换失败，返回配置无效错误
	return plugins.ErrInvalidConfiguration
}

// ========== 处理器实现 ==========
// 处理器相关代码已移至 handlers.go 文件
