package tracer

import (
	"context"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/internal/tracer/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var (
	name       = "tracer"
	confPrefix = "lynx.tracer"
)

type PlugTracer struct {
	conf   *conf.Tracer
	weight int
}

type Option func(t *PlugTracer)

func Weight(w int) Option {
	return func(t *PlugTracer) {
		t.weight = w
	}
}

func Config(c *conf.Tracer) Option {
	return func(t *PlugTracer) {
		t.conf = c
	}
}

func (t *PlugTracer) Load(b config.Value) (plugins.Plugin, error) {
	// 从配置值中扫描并填充 PlugTracer 结构体的 conf 字段
	err := b.Scan(t.conf)
	// 如果扫描过程中发生错误，返回 nil 和错误信息
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录日志，指示正在初始化链路监控组件
	app.Lynx().Helper().Infof("Initializing link monitoring component")

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
		return nil, err
	}

	// 创建一个新的跟踪提供者，用于生成和处理跟踪数据
	tp := traceSdk.NewTracerProvider(
		// 设置采样器，根据配置中的比率进行采样
		traceSdk.WithSampler(traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(t.conf.GetRatio())))),
		// 设置导出器，用于将跟踪数据发送到收集器
		traceSdk.WithBatcher(exp),
		// 设置资源信息，包括服务实例 ID、服务名称、服务版本和服务命名空间
		traceSdk.WithResource(
			resource.NewSchemaless(
				// 服务实例 ID，使用主机名
				semconv.ServiceInstanceIDKey.String(app.Host()),
				// 服务名称
				semconv.ServiceNameKey.String(app.Name()),
				// 服务版本
				semconv.ServiceVersionKey.String(app.Version()),
				// 服务命名空间，使用 Lynx 控制平面的命名空间
				semconv.ServiceNamespaceKey.String(app.Lynx().ControlPlane().Namespace()),
			)),
	)

	// 设置全局跟踪提供者，用于后续的跟踪数据生成和处理
	otel.SetTracerProvider(tp)

	// 使用 Lynx 应用的 Helper 记录日志，指示链路监控组件初始化成功
	app.Lynx().Helper().Infof("Link monitoring component successfully initialized")

	// 返回加载的插件实例和 nil 错误，表示加载成功
	return t, nil
}

func (t *PlugTracer) Unload() error {
	return nil
}

func Tracer(opts ...Option) plugins.Plugin {
	t := &PlugTracer{
		weight: 700,
		conf:   &conf.Tracer{},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
