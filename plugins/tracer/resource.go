package tracer

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// buildResource 根据 Tracer 配置与 Lynx 应用元信息构建 OpenTelemetry Resource：
// - service.name 优先使用配置中的 resource.service_name，否则回退到 app.GetName()
// - 默认注入：service.instance.id、service.version、service.namespace
// - 支持附加自定义 attributes（字符串键值对）
// 注意：使用 Schemaless 构建，便于灵活扩展。
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
