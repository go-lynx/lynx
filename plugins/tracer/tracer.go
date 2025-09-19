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
// - additional validation for config fields when present
func (t *PlugTracer) validateConfiguration() error {
	// Validate sampling ratio (even when disabled, ratio should be valid for future use)
	if t.conf.Ratio < 0 || t.conf.Ratio > 1 {
		return fmt.Errorf("sampling ratio must be between 0 and 1, got %f", t.conf.Ratio)
	}

	// Validate address configuration when tracing is enabled
	if t.conf.Enable && t.conf.Addr == "" {
		return fmt.Errorf("tracer address is required when tracing is enabled")
	}

	// Additional validation for config fields when present
	if t.conf.Config != nil {
		if err := t.validateConfigFields(); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}
	}

	return nil
}

// validateConfigFields validates the nested config fields
func (t *PlugTracer) validateConfigFields() error {
	cfg := t.conf.Config

	// Validate batch configuration
	if cfg.Batch != nil && cfg.Batch.GetEnabled() {
		if cfg.Batch.GetMaxQueueSize() < 0 {
			return fmt.Errorf("batch max_queue_size must be non-negative")
		}
		if cfg.Batch.GetMaxBatchSize() < 0 {
			return fmt.Errorf("batch max_batch_size must be non-negative")
		}
		// Validate batch size vs queue size relationship
		if cfg.Batch.GetMaxBatchSize() > 0 && cfg.Batch.GetMaxQueueSize() > 0 {
			if cfg.Batch.GetMaxBatchSize() > cfg.Batch.GetMaxQueueSize() {
				return fmt.Errorf("batch max_batch_size cannot exceed max_queue_size")
			}
		}
		// Validate that at least one of the batch limits is set
		if cfg.Batch.GetMaxQueueSize() == 0 && cfg.Batch.GetMaxBatchSize() == 0 {
			return fmt.Errorf("batch processing enabled but no limits configured (max_queue_size or max_batch_size must be set)")
		}
	}

	// Validate retry configuration
	if cfg.Retry != nil && cfg.Retry.GetEnabled() {
		if cfg.Retry.GetMaxAttempts() < 1 {
			return fmt.Errorf("retry max_attempts must be at least 1")
		}
		if cfg.Retry.GetInitialInterval() != nil && cfg.Retry.GetInitialInterval().AsDuration() < 0 {
			return fmt.Errorf("retry initial_interval must be non-negative")
		}
		if cfg.Retry.GetMaxInterval() != nil && cfg.Retry.GetMaxInterval().AsDuration() < 0 {
			return fmt.Errorf("retry max_interval must be non-negative")
		}
		// Validate interval relationship
		if cfg.Retry.GetInitialInterval() != nil && cfg.Retry.GetMaxInterval() != nil {
			if cfg.Retry.GetInitialInterval().AsDuration() > cfg.Retry.GetMaxInterval().AsDuration() {
				return fmt.Errorf("retry initial_interval cannot exceed max_interval")
			}
		}
	}

	// Validate sampler configuration
	if cfg.Sampler != nil {
		if cfg.Sampler.GetRatio() < 0 || cfg.Sampler.GetRatio() > 1 {
			return fmt.Errorf("sampler ratio must be between 0 and 1, got %f", cfg.Sampler.GetRatio())
		}
	}

	// Validate connection management configuration
	if cfg.Connection != nil {
		if err := t.validateConnectionConfig(cfg.Connection); err != nil {
			return fmt.Errorf("connection configuration validation failed: %w", err)
		}
	}

	// Validate load balancing configuration
	if cfg.LoadBalancing != nil {
		if err := t.validateLoadBalancingConfig(cfg.LoadBalancing); err != nil {
			return fmt.Errorf("load balancing configuration validation failed: %w", err)
		}
	}

	return nil
}

// validateConnectionConfig validates connection management configuration
func (t *PlugTracer) validateConnectionConfig(conn *conf.Connection) error {
	// Validate connection timeout
	if conn.GetConnectTimeout() != nil && conn.GetConnectTimeout().AsDuration() < 0 {
		return fmt.Errorf("connection connect_timeout must be non-negative")
	}

	// Validate reconnection period
	if conn.GetReconnectionPeriod() != nil && conn.GetReconnectionPeriod().AsDuration() < 0 {
		return fmt.Errorf("connection reconnection_period must be non-negative")
	}

	// Validate connection age settings
	if conn.GetMaxConnAge() != nil && conn.GetMaxConnAge().AsDuration() < 0 {
		return fmt.Errorf("connection max_conn_age must be non-negative")
	}

	if conn.GetMaxConnIdleTime() != nil && conn.GetMaxConnIdleTime().AsDuration() < 0 {
		return fmt.Errorf("connection max_conn_idle_time must be non-negative")
	}

	if conn.GetMaxConnAgeGrace() != nil && conn.GetMaxConnAgeGrace().AsDuration() < 0 {
		return fmt.Errorf("connection max_conn_age_grace must be non-negative")
	}

	// Validate relationship between connection age and idle time
	if conn.GetMaxConnAge() != nil && conn.GetMaxConnIdleTime() != nil {
		if conn.GetMaxConnAge().AsDuration() < conn.GetMaxConnIdleTime().AsDuration() {
			return fmt.Errorf("connection max_conn_age cannot be less than max_conn_idle_time")
		}
	}

	return nil
}

// validateLoadBalancingConfig validates load balancing configuration
func (t *PlugTracer) validateLoadBalancingConfig(lb *conf.LoadBalancing) error {
	// Validate load balancing policy
	validPolicies := map[string]bool{
		"pick_first":  true,
		"round_robin": true,
		"least_conn":  true,
	}

	if lb.GetPolicy() != "" && !validPolicies[lb.GetPolicy()] {
		return fmt.Errorf("load balancing policy must be one of: pick_first, round_robin, least_conn, got: %s", lb.GetPolicy())
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

	// Use Lynx application Helper to log, indicating that tracing component is being initialized
	log.Infof("Initializing tracing component")

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

	// Use Lynx application Helper to log, indicating that tracing component initialization was successful
	log.Infof("Tracing component successfully initialized")
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
			// Create context with timeout for graceful shutdown
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
