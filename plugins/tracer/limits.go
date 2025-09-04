package tracer

import (
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
)

// buildSpanLimits builds OpenTelemetry SpanLimits based on configuration.
// Only sets fields supported by the current SDK version:
// - AttributeCountLimit
// - AttributeValueLengthLimit
// - EventCountLimit
// - LinkCountLimit
// Unconfigured or value <= 0 fields will be ignored, returning nil indicates not to override default limits.
func buildSpanLimits(c *conf.Tracer) *traceSdk.SpanLimits {
	// Read modular configuration; if limits not provided, return nil to indicate using SDK default limits
	cfg := c.GetConfig()
	if cfg == nil || cfg.Limits == nil {
		return nil
	}
	// Extract limit configuration
	l := cfg.GetLimits()
	// Initialize empty SpanLimits; only assign values >0
	limits := &traceSdk.SpanLimits{}
	// Maximum number of attributes allowed per Span
	if v := l.GetAttributeCountLimit(); v > 0 {
		limits.AttributeCountLimit = int(v)
	}
	// Maximum value length (characters) for a single attribute
	if v := l.GetAttributeValueLengthLimit(); v > 0 {
		limits.AttributeValueLengthLimit = int(v)
	}
	// Maximum number of events allowed per Span
	if v := l.GetEventCountLimit(); v > 0 {
		limits.EventCountLimit = int(v)
	}
	// Maximum number of links allowed per Span
	if v := l.GetLinkCountLimit(); v > 0 {
		limits.LinkCountLimit = int(v)
	}
	// Note: If none of the above fields are set (remain 0), SDK will use its default values; returning empty structure is sufficient
	return limits
}
