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
		ratio := c.GetRatio()
		// Ensure ratio is within valid range with proper floating-point handling
		ratio = clampRatio(ratio)
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(ratio)))
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
		if !isValidRatio(r) {
			// If ratio is invalid, use outer ratio as fallback
			r = c.GetRatio()
			// Ensure fallback ratio is also valid
			r = clampRatio(r)
		}
		return traceSdk.TraceIDRatioBased(float64(r))

	case conf.Sampler_PARENT_BASED_TRACEID_RATIO:
		// Parent-based ratio sampling
		r := s.GetRatio()
		if !isValidRatio(r) {
			// If ratio is invalid, use outer ratio as fallback
			r = c.GetRatio()
			// Ensure fallback ratio is also valid
			r = clampRatio(r)
		}
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(r)))

	default:
		// Default to ParentBased + TraceIDRatio with validated ratio
		ratio := c.GetRatio()
		// Ensure ratio is within valid range
		ratio = clampRatio(ratio)
		return traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(ratio)))
	}
}

// isValidRatio checks if a ratio value is within the valid range [0.0, 1.0]
// Uses epsilon comparison to handle floating-point precision issues
func isValidRatio(ratio float32) bool {
	const epsilon = 1e-9
	return ratio >= -epsilon && ratio <= 1.0+epsilon
}

// clampRatio ensures a ratio value is within the valid range [0.0, 1.0]
func clampRatio(ratio float32) float32 {
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}
