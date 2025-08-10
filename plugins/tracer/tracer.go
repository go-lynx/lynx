package tracer

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

// 插件元数据，定义 Tracer 插件的基本信息
const (
	// pluginName 是 Tracer 插件在 Lynx 插件系统中的唯一标识符。
	pluginName = "tracer.server"

	// pluginVersion 表示 Tracer 插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述 Tracer 插件的用途。
	pluginDescription = "OpenTelemetry tracer plugin for Lynx framework"

	// confPrefix 是加载 Tracer 配置时使用的配置前缀。
	confPrefix = "lynx.tracer"
)

// PlugTracer 实现了 Lynx 框架的 Tracer 插件功能。
// 它嵌入了 plugins.BasePlugin 以继承通用的插件功能，并维护 Tracer 链路追踪的配置和实例。
type PlugTracer struct {
	// 嵌入基础插件，继承插件的通用属性和方法
	*plugins.BasePlugin
	// Tracer 的配置信息（支持模块化配置与向后兼容的旧字段）
	conf *conf.Tracer
}

// NewPlugTracer 创建一个新的 Tracer 插件实例。
// 该函数初始化插件的基础信息（ID、名称、描述、版本、配置前缀、权重）并返回实例。
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

// InitializeResources 从运行时加载并校验 Tracer 配置，同时填充默认值。
// - 先从 runtime 配置树中扫描 "lynx.tracer" 到 t.conf
// - 校验必要参数（采样率范围、启用但未配置地址等）
// - 设置合理默认值（addr、ratio）
func (t *PlugTracer) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	t.conf = &conf.Tracer{}

	// 从运行时配置中扫描并加载 Tracer 配置
	err := rt.GetConfig().Value(confPrefix).Scan(t.conf)
	if err != nil {
		return fmt.Errorf("failed to load tracer configuration: %w", err)
	}

	// 验证配置
	if err := t.validateConfiguration(); err != nil {
		return fmt.Errorf("tracer configuration validation failed: %w", err)
	}

	// 设置默认值
	t.setDefaultValues()

	return nil
}

// validateConfiguration 验证配置合法性：
// - ratio 必须在 [0,1]
// - 当 enable=true 时必须提供有效的 addr
func (t *PlugTracer) validateConfiguration() error {
	// 验证采样率
	if t.conf.Ratio < 0 || t.conf.Ratio > 1 {
		return fmt.Errorf("sampling ratio must be between 0 and 1, got %f", t.conf.Ratio)
	}

	// 验证地址配置
	if t.conf.Enable && t.conf.Addr == "" {
		return fmt.Errorf("tracer address is required when tracing is enabled")
	}

	return nil
}

// setDefaultValues 为未配置项设置默认值：
// - addr 默认为 localhost:4317（OTLP/gRPC 默认端口）
// - ratio 默认为 1.0（全量采样）
func (t *PlugTracer) setDefaultValues() {
	if t.conf.Addr == "" {
		t.conf.Addr = "localhost:4317"
	}
	if t.conf.Ratio == 0 {
		t.conf.Ratio = 1.0
	}
}

// StartupTasks 完成 OpenTelemetry TracerProvider 的初始化：
// - 构建采样器、资源、Span 限额
// - 根据配置创建 OTLP 导出器（gRPC/HTTP），并选择批处理或同步处理器
// - 设置全局 TracerProvider 与 TextMapPropagator
// - 打印初始化日志
func (t *PlugTracer) StartupTasks() error {
	if !t.conf.Enable {
		return nil
	}

	// 使用 Lynx 应用的 Helper 记录日志，指示正在初始化链路监控组件
	log.Infof("Initializing link monitoring component")

	var tracerProviderOptions []trace.TracerProviderOption

	// Sampler
	sampler := buildSampler(t.conf)
	tracerProviderOptions = append(tracerProviderOptions, trace.WithSampler(sampler))

	// Resource
	res := buildResource(t.conf)
	tracerProviderOptions = append(tracerProviderOptions, trace.WithResource(res))

	// Span limits
	if limits := buildSpanLimits(t.conf); limits != nil {
		tracerProviderOptions = append(tracerProviderOptions, trace.WithSpanLimits(*limits))
	}

	// 如果配置中指定了地址，则设置导出器
	// 否则，不设置导出器
	if t.conf.GetAddr() != "None" {
		exp, batchOpts, useBatch, err := buildExporter(context.Background(), t.conf)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		if useBatch {
			tracerProviderOptions = append(tracerProviderOptions, trace.WithBatcher(exp, batchOpts...))
		} else {
			tracerProviderOptions = append(tracerProviderOptions, trace.WithSyncer(exp))
		}
	}

	// 创建一个新的跟踪提供者，用于生成和处理跟踪数据
	tp := trace.NewTracerProvider(tracerProviderOptions...)

	// 设置全局跟踪提供者，用于后续的跟踪数据生成和处理
	otel.SetTracerProvider(tp)

	// Propagators
	var propagator propagation.TextMapPropagator = buildPropagator(t.conf)
	otel.SetTextMapPropagator(propagator)

	// 验证 TracerProvider 是否成功创建
	if tp == nil {
		return fmt.Errorf("failed to create tracer provider")
	}

	// 使用 Lynx 应用的 Helper 记录日志，指示链路监控组件初始化成功
	log.Infof("link monitoring component successfully initialized")
	return nil
}

// ShutdownTasks 优雅关闭 TracerProvider：
// - 在 30s 超时内调用 SDK 的 Shutdown
// - 捕获并记录错误
func (t *PlugTracer) ShutdownTasks() error {
	// 获取全局 TracerProvider
	tp := otel.GetTracerProvider()
	if tp != nil {
		// 检查是否为 SDK TracerProvider
		if sdkTp, ok := tp.(*trace.TracerProvider); ok {
			// 创建带超时的上下文
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// 在关闭前尽力刷新缓冲中的 span，减少数据丢失
			if err := sdkTp.ForceFlush(ctx); err != nil {
				log.Errorf("Failed to force flush tracer provider: %v", err)
			}

			// 优雅关闭 TracerProvider
			if err := sdkTp.Shutdown(ctx); err != nil {
				log.Errorf("Failed to shutdown tracer provider: %v", err)
				return fmt.Errorf("failed to shutdown tracer provider: %w", err)
			}

			log.Infof("Tracer provider shutdown successfully")
		}
	}

	return nil
}
