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

// Plugin metadata, defines basic information of Tracer plugin
const (
	// pluginName is the unique identifier of Tracer plugin in Lynx plugin system.
	pluginName = "tracer.server"

	// pluginVersion represents the current version of Tracer plugin.
	pluginVersion = "v2.0.0"

	// pluginDescription briefly describes the purpose of Tracer plugin.
	pluginDescription = "OpenTelemetry tracer plugin for Lynx framework"

	// confPrefix is the configuration prefix used when loading Tracer configuration.
	confPrefix = "lynx.tracer"
)

// PlugTracer implements the Tracer plugin functionality for Lynx framework.
// It embeds plugins.BasePlugin to inherit common plugin functionality and maintains Tracer tracing configuration and instances.
type PlugTracer struct {
	// Embed base plugin, inheriting common plugin properties and methods
	*plugins.BasePlugin
	// Tracer configuration information (supports modular configuration and backward-compatible old fields)
	conf *conf.Tracer
}

// NewPlugTracer creates a new Tracer plugin instance.
// This function initializes the plugin's basic information (ID, name, description, version, configuration prefix, weight) and returns the instance.
func NewPlugTracer() *PlugTracer {
	return &PlugTracer{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			9999,
		),
		conf: &conf.Tracer{},
	}
}

// InitializeResources loads and validates Tracer configuration from runtime, while filling default values.
// - First scan "lynx.tracer" from runtime configuration tree to t.conf
// - Validate necessary parameters (sampling ratio range, enabled but unconfigured address, etc.)
// - Set reasonable default values (addr, ratio)
func (t *PlugTracer) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	t.conf = &conf.Tracer{}

	// Scan and load Tracer configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(t.conf)
	if err != nil {
		return fmt.Errorf("failed to load tracer configuration: %w", err)
	}

	// Validate configuration
	if err := t.validateConfiguration(); err != nil {
		return fmt.Errorf("tracer configuration validation failed: %w", err)
	}

	// Set default values
	t.setDefaultValues()

	return nil
}

// validateConfiguration validates configuration legality:
// - ratio must be in [0,1]
// - when enable=true, valid addr must be provided
func (t *PlugTracer) validateConfiguration() error {
	// Validate sampling ratio
	if t.conf.Ratio < 0 || t.conf.Ratio > 1 {
		return fmt.Errorf("sampling ratio must be between 0 and 1, got %f", t.conf.Ratio)
	}

	// Validate address configuration
	if t.conf.Enable && t.conf.Addr == "" {
		return fmt.Errorf("tracer address is required when tracing is enabled")
	}

	return nil
}

// setDefaultValues sets default values for unconfigured items:
// - addr defaults to localhost:4317 (OTLP/gRPC default port)
// - ratio defaults to 1.0 (full sampling)
func (t *PlugTracer) setDefaultValues() {
	if t.conf.Addr == "" {
		t.conf.Addr = "localhost:4317"
	}
	if t.conf.Ratio == 0 {
		t.conf.Ratio = 1.0
	}
}

// StartupTasks completes OpenTelemetry TracerProvider initialization:
// - Build sampler, resource, Span limits
// - Create OTLP exporter (gRPC/HTTP) based on configuration, and choose batch or sync processor
// - Set global TracerProvider and TextMapPropagator
// - Print initialization logs
func (t *PlugTracer) StartupTasks() error {
	if !t.conf.Enable {
		return nil
	}

	// Use Lynx application Helper to log, indicating that link monitoring component is being initialized
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

	// If address is specified in configuration, set exporter
	// Otherwise, don't set exporter
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

	// Create a new trace provider for generating and processing trace data
	tp := trace.NewTracerProvider(tracerProviderOptions...)

	// Set global trace provider for subsequent trace data generation and processing
	otel.SetTracerProvider(tp)

	// Propagators
	var propagator propagation.TextMapPropagator = buildPropagator(t.conf)
	otel.SetTextMapPropagator(propagator)

	// Verify that TracerProvider was successfully created
	if tp == nil {
		return fmt.Errorf("failed to create tracer provider")
	}

	// Use Lynx application Helper to log, indicating that link monitoring component initialization was successful
	log.Infof("link monitoring component successfully initialized")
	return nil
}

// ShutdownTasks gracefully shuts down TracerProvider:
// - Call SDK's Shutdown within 30s timeout
// - Catch and log errors
func (t *PlugTracer) ShutdownTasks() error {
	// Get global TracerProvider
	tp := otel.GetTracerProvider()
	if tp != nil {
		// Check if it's SDK TracerProvider
		if sdkTp, ok := tp.(*trace.TracerProvider); ok {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Try to flush buffered spans before shutdown to reduce data loss
			if err := sdkTp.ForceFlush(ctx); err != nil {
				log.Errorf("Failed to force flush tracer provider: %v", err)
			}

			// Gracefully shutdown TracerProvider
			if err := sdkTp.Shutdown(ctx); err != nil {
				log.Errorf("Failed to shutdown tracer provider: %v", err)
				return fmt.Errorf("failed to shutdown tracer provider: %w", err)
			}

			log.Infof("Tracer provider shutdown successfully")
		}
	}

	return nil
}
