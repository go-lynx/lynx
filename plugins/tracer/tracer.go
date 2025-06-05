package tracer

import (
	"context"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugins/tracer/v2/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Plugin metadata
// 插件元数据，定义插件的基本信息
const (
	// pluginName 是 HTTP 服务器插件的唯一标识符，用于在插件系统中识别该插件。
	pluginName = "tracer.server"

	// pluginVersion 表示 HTTP 服务器插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述了 HTTP 服务器插件的功能。
	pluginDescription = "tracer server plugin for lynx framework"

	// confPrefix 是加载 HTTP 服务器配置时使用的配置前缀。
	confPrefix = "lynx.tracer"
)

// PlugTracer 实现了 Lynx 框架的 Tracer 插件功能。
// 它嵌入了 plugins.BasePlugin 以继承通用的插件功能，并维护 Tracer 链路追踪的配置和实例。
type PlugTracer struct {
	// 嵌入基础插件，继承插件的通用属性和方法
	*plugins.BasePlugin
	// HTTP 服务器的配置信息
	conf *conf.Tracer
}

// NewPlugTracer 创建一个新的 Tracer 服务器插件实例。
// 该函数初始化插件的基础信息，并返回一个指向 Tracer 结构体的指针。
func NewPlugTracer() *PlugTracer {
	return &PlugTracer{
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
			9999,
		),
		conf: &conf.Tracer{},
	}
}

func (t *PlugTracer) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	t.conf = &conf.Tracer{}

	// 从运行时配置中扫描并加载 Tracer 配置
	err := rt.GetConfig().Value(confPrefix).Scan(t.conf)
	if err != nil {
		return err
	}

	// 设置默认配置
	defaultConf := &conf.Tracer{
		// 默认不启用链路跟踪
		Enable: false,
		// 默认导出地址为 localhost:4317
		Addr: "localhost:4317",
		// 默认采样率为 1.0，即全量采样
		Ratio: 1.0,
	}

	// 对未设置的字段使用默认值
	if t.conf.Addr == "" {
		t.conf.Addr = defaultConf.Addr
	}
	if t.conf.Ratio == 0 {
		t.conf.Ratio = defaultConf.Ratio
	}

	return nil
}

func (t *PlugTracer) StartupTasks() error {
	if !t.conf.Enable {
		return nil
	}

	// 使用 Lynx 应用的 Helper 记录日志，指示正在初始化链路监控组件
	log.Infof("Initializing link monitoring component")

	var tracerProviderOptions []traceSdk.TracerProviderOption

	tracerProviderOptions = append(tracerProviderOptions, // 设置采样器，根据配置中的比率进行采样
		traceSdk.WithSampler(traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(t.conf.GetRatio())))),
		// 设置资源信息，包括服务实例 ID、服务名称、服务版本和服务命名空间
		traceSdk.WithResource(
			resource.NewSchemaless(
				// 服务实例 ID，使用主机名
				semconv.ServiceInstanceIDKey.String(app.GetHost()),
				// 服务名称
				semconv.ServiceNameKey.String(app.GetName()),
				// 服务版本
				semconv.ServiceVersionKey.String(app.GetVersion()),
				// 服务命名空间，使用 Lynx 控制平面的命名空间
				semconv.ServiceNamespaceKey.String(app.Lynx().GetControlPlane().GetNamespace()),
			)))

	// 如果配置中指定了地址，则设置导出器
	// 否则，不设置导出器
	if t.conf.GetAddr() != "None" {
		// 创建一个新的 ot-lp 跟踪导出器，用于将跟踪数据发送到指定的端点
		exp, err := otlptracegrpc.New(
			context.Background(),
			// 设置导出器的端点地址
			otlptracegrpc.WithEndpoint(t.conf.GetAddr()),
			// 禁用 TLS 加密，使用不安全的连接
			otlptracegrpc.WithInsecure(),
			// 使用 gzip 压缩算法来压缩跟踪数据
			otlptracegrpc.WithCompressor("gzip"),
		)
		// 如果创建导出器时发生错误，返回 nil 和错误信息
		if err != nil {
			return err
		}
		// 设置导出器，用于将跟踪数据发送到收集器
		tracerProviderOptions = append(tracerProviderOptions, traceSdk.WithBatcher(exp))
	}

	// 创建一个新的跟踪提供者，用于生成和处理跟踪数据
	tp := traceSdk.NewTracerProvider(tracerProviderOptions...)

	// 设置全局跟踪提供者，用于后续的跟踪数据生成和处理
	otel.SetTracerProvider(tp)

	// 使用 Lynx 应用的 Helper 记录日志，指示链路监控组件初始化成功
	log.Infof("link monitoring component successfully initialized")
	return nil
}
