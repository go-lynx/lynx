package tracer

import (
	"context"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/tracer/conf"
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

func (t *PlugTracer) Weight() int {
	return t.weight
}

func (t *PlugTracer) DependsOn(config.Value) []string {
	return nil
}

func (t *PlugTracer) Name() string {
	return name
}

func (t *PlugTracer) ConfPrefix() string {
	return confPrefix
}

func (t *PlugTracer) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(t.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing link monitoring component")
	exp, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(t.conf.GetAddr()),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithCompressor("gzip"),
	)
	if err != nil {
		return nil, err
	}

	tp := traceSdk.NewTracerProvider(
		traceSdk.WithSampler(traceSdk.ParentBased(traceSdk.TraceIDRatioBased(float64(t.conf.GetRatio())))),
		traceSdk.WithBatcher(exp),
		traceSdk.WithResource(
			resource.NewSchemaless(
				semconv.ServiceInstanceIDKey.String(app.Host()),
				semconv.ServiceNameKey.String(app.Name()),
				semconv.ServiceVersionKey.String(app.Version()),
				semconv.ServiceNamespaceKey.String(app.Lynx().ControlPlane().Namespace()),
			)),
	)

	otel.SetTracerProvider(tp)
	app.Lynx().Helper().Infof("Link monitoring component successfully initialized")
	return t, nil
}

func (t *PlugTracer) Unload() error {
	return nil
}

func Tracer(opts ...Option) plugin.Plugin {
	t := &PlugTracer{
		weight: 700,
		conf:   &conf.Tracer{},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
