package tracer

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// buildResource builds OpenTelemetry Resource based on Tracer configuration and Lynx application metadata:
// - service.name prefers resource.service_name from config, otherwise falls back to app.GetName()
// - Default injection: service.instance.id, service.version, service.namespace
// - Supports additional custom attributes (string key-value pairs)
// Note: Uses Schemaless construction for flexible extension.
func buildResource(c *conf.Tracer) *resource.Resource {
	var r *conf.Resource
	if c != nil && c.GetConfig() != nil {
		r = c.GetConfig().GetResource()
	}
	attrs := []attribute.KeyValue{
		semconv.ServiceInstanceIDKey.String(app.GetHost()),
		semconv.ServiceVersionKey.String(app.GetVersion()),
		semconv.ServiceNamespaceKey.String(app.Lynx().GetControlPlane().GetNamespace()),
	}

	// service.name: prefer config override
	serviceName := app.GetName()
	if r != nil && r.GetServiceName() != "" {
		serviceName = r.GetServiceName()
	}
	attrs = append(attrs, semconv.ServiceNameKey.String(serviceName))

	// extra attributes
	if r != nil {
		for k, v := range r.GetAttributes() {
			attrs = append(attrs, attribute.String(k, v))
		}
	}

	return resource.NewSchemaless(attrs...)
}
