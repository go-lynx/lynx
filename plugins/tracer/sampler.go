package tracer

import (
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
)

// buildSampler builds an OpenTelemetry Sampler based on Tracer configuration.
// Priority is given to the modular sampler configuration:
// - ALWAYS_ON:      Always sample (AlwaysSample)
// - ALWAYS_OFF:     Never sample (NeverSample)
// - TRACEID_RATIO:  Sample by ratio (TraceIDRatioBased)
// - PARENT_BASED_TRACEID_RATIO: Parent-based ratio sampling (ParentBased + TraceIDRatioBased)
// Fallback strategy:
// - When config.sampler is not configured or type is not specified, use the outer legacy ratio field, default ParentBased(TraceIDRatioBased(ratio)).
func buildSampler(c *conf.Tracer) traceSdk.Sampler {
	// Get Tracer configuration
	cfg := c.GetConfig()
	// If configuration is empty or sampler configuration is empty or type is not specified, use default configuration
	if cfg == nil || cfg.Sampler == nil || cfg.Sampler.Type == conf.Sampler_SAMPLER_UNSPECIFIED {
		// Default: ParentBased + TraceIDRatio using outer ratio (backward compatibility)
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(c.GetRatio())))
	}

	// Get sampler configuration
	s := cfg.GetSampler()
	// Build sampler based on sampler type
	switch s.GetType() {
	case conf.Sampler_ALWAYS_ON:
		// Always sample
		return traceSdk.AlwaysSample()
	case conf.Sampler_ALWAYS_OFF:
		// Never sample
		return traceSdk.NeverSample()
	case conf.Sampler_TRACEID_RATIO:
		// Sample by ratio
		r := s.GetRatio()
		if r == 0 {
			// If ratio is not configured, use outer ratio
			r = c.GetRatio()
		}
		return traceSdk.TraceIDRatioBased(float64(r))
	case conf.Sampler_PARENT_BASED_TRACEID_RATIO:
		// Parent-based ratio sampling
		r := s.GetRatio()
		if r == 0 {
			// If ratio is not configured, use outer ratio
			r = c.GetRatio()
		}
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(r)))
	default:
		// Default to ParentBased + TraceIDRatio
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(c.GetRatio())))
	}
}
