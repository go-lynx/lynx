package tracer

import (
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
)

// buildPropagator 根据 `conf.Tracer.Config.propagators` 构建全局 TextMapPropagator（上下文传播器）。
//
// 语义说明：
// - 当存在多个传播器时，采用 Composite 组合，Inject 时会按顺序写入多个 header；Extract 时依次尝试，先成功先返回。
// - 若配置为空或全部不识别，则回退到默认组合：W3C TraceContext + W3C Baggage。
//
// 支持的取值（对应 `conf.Propagator_*`）：
// - W3C_TRACE_CONTEXT：W3C Trace Context（traceparent/tracestate）。
// - W3C_BAGGAGE：W3C Baggage（baggage）。
// - B3：B3 单头（b3）。
// - B3_MULTI：B3 多头（x-b3-traceid / x-b3-spanid / x-b3-sampled ...）。
// - JAEGER：Jaeger（uber-trace-id）。
func buildPropagator(c *conf.Tracer) propagation.TextMapPropagator {
	// 读取插件配置（可能为 nil）
	cfg := c.GetConfig()
	if cfg == nil || len(cfg.GetPropagators()) == 0 {
		// 无配置时的安全默认：W3C tracecontext + baggage
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	var list []propagation.TextMapPropagator
	// 将配置的传播器逐一解析并加入组合
	for _, p := range cfg.GetPropagators() {
		switch p {
		case conf.Propagator_W3C_TRACE_CONTEXT:
			// W3C Trace Context：标准 traceparent/tracestate
			list = append(list, propagation.TraceContext{})
		case conf.Propagator_W3C_BAGGAGE:
			// W3C Baggage：携带自定义键值上下文
			list = append(list, propagation.Baggage{})
		case conf.Propagator_B3:
			// B3 单头：使用单个 "b3" header 进行注入/提取
			list = append(list, b3.New())
		case conf.Propagator_B3_MULTI:
			// B3 多头：使用多个 x-b3-* header；与部分网关/老组件更兼容
			list = append(list, b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)))
		case conf.Propagator_JAEGER:
			// Jaeger：使用 uber-trace-id header
			list = append(list, jaeger.Jaeger{})
		}
	}
	if len(list) == 0 {
		// 全部不识别/过滤后仍为空时的兜底
		return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}
	// 使用 Composite 组合，保持配置顺序；顺序可能影响 Inject 时 header 的覆盖顺序
	return propagation.NewCompositeTextMapPropagator(list...)
}

