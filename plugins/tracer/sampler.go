package tracer

import (
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
)

// buildSampler 根据 Tracer 配置构建 OpenTelemetry 采样器（Sampler）。
// 优先读取 modular 配置中的 sampler：
// - ALWAYS_ON:      始终采样（AlwaysSample）
// - ALWAYS_OFF:     从不采样（NeverSample）
// - TRACEID_RATIO:  按比例采样（TraceIDRatioBased）
// - PARENT_BASED_TRACEID_RATIO: 基于父 Span 的按比例采样（ParentBased + TraceIDRatioBased）
// 回退策略：
// - 当未配置 config.sampler 或类型未指定时，使用外层 legacy 字段 ratio，默认 ParentBased(TraceIDRatioBased(ratio))。
func buildSampler(c *conf.Tracer) traceSdk.Sampler {
	// 获取 Tracer 配置
	cfg := c.GetConfig()
	// 如果配置为空或采样器配置为空或类型未指定，则使用默认配置
	if cfg == nil || cfg.Sampler == nil || cfg.Sampler.Type == conf.Sampler_SAMPLER_UNSPECIFIED {
		// 默认：ParentBased + TraceIDRatio 取外层 ratio（向后兼容）
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(c.GetRatio())))
	}

	// 获取采样器配置
	s := cfg.GetSampler()
	// 根据采样器类型构建采样器
	switch s.GetType() {
	case conf.Sampler_ALWAYS_ON:
		// 始终采样
		return traceSdk.AlwaysSample()
	case conf.Sampler_ALWAYS_OFF:
		// 从不采样
		return traceSdk.NeverSample()
	case conf.Sampler_TRACEID_RATIO:
		// 按比例采样
		r := s.GetRatio()
		if r == 0 {
			// 如果比例未配置，则使用外层 ratio
			r = c.GetRatio()
		}
		return traceSdk.TraceIDRatioBased(float64(r))
	case conf.Sampler_PARENT_BASED_TRACEID_RATIO:
		// 基于父 Span 的按比例采样
		r := s.GetRatio()
		if r == 0 {
			// 如果比例未配置，则使用外层 ratio
			r = c.GetRatio()
		}
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(r)))
	default:
		// 默认使用 ParentBased + TraceIDRatio
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(c.GetRatio())))
	}
}
