package tracer

import (
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
)

// buildPropagator builds global TextMapPropagator (context propagator) based on `conf.Tracer.Config.propagators`.
//
// Semantic description:
// - When multiple propagators exist, use Composite combination. Inject writes multiple headers in order; Extract tries in sequence, first success first return.
// - If configuration is empty or all unrecognized, fall back to default combination: W3C TraceContext + W3C Baggage.
//
// Supported values (corresponding to `conf.Propagator_*`):
// - W3C_TRACE_CONTEXT: W3C Trace Context (traceparent/tracestate).
// - W3C_BAGGAGE: W3C Baggage (baggage).
// - B3: B3 single header (b3).
// - B3_MULTI: B3 multiple headers (x-b3-traceid / x-b3-spanid / x-b3-sampled ...).
// - JAEGER: Jaeger (uber-trace-id).
func buildPropagator(c *conf.Tracer) propagation.TextMapPropagator {
	// Read plugin configuration (may be nil)
	cfg := c.GetConfig()
	if cfg == nil || len(cfg.GetPropagators()) == 0 {
		// Safe default when no configuration: W3C tracecontext + baggage
		return propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	var list []propagation.TextMapPropagator
	// Parse configured propagators one by one and add to combination
	for _, p := range cfg.GetPropagators() {
		switch p {
		case conf.Propagator_W3C_TRACE_CONTEXT:
			// W3C Trace Context: standard traceparent/tracestate
			list = append(list, propagation.TraceContext{})
		case conf.Propagator_W3C_BAGGAGE:
			// W3C Baggage: carries custom key-value context
			list = append(list, propagation.Baggage{})
		case conf.Propagator_B3:
			// B3 single header: uses single "b3" header for injection/extraction
			list = append(list, b3.New())
		case conf.Propagator_B3_MULTI:
			// B3 multiple headers: uses multiple x-b3-* headers; more compatible with some gateways/old components
			list = append(list, b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)))
		case conf.Propagator_JAEGER:
			// Jaeger: uses uber-trace-id header
			list = append(list, jaeger.Jaeger{})
		}
	}
	if len(list) == 0 {
		// Fallback when all unrecognized/filtered out and still empty
		return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}
	// Use Composite combination, maintaining configuration order; order may affect header override order during Inject
	return propagation.NewCompositeTextMapPropagator(list...)
}
