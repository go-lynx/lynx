package tracer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
)

// buildExporter builds OTLP Trace exporter and batch processing (BatchSpanProcessor) options based on Tracer configuration.
// Features:
// - Protocol: gRPC (default) or HTTP (specified via config.protocol)
// - Connection: addr, insecure/TLS, headers, custom http_path (HTTP)
// - Reliability: timeout, retry (gRPC supports initial/max interval)
// - Compression: gzip (gRPC/HTTP)
// - Batch processing: queue size, batch size, export timeout, scheduling delay
// Returns:
// - exp: Initialized SpanExporter
// - batchOpts: Batch processor options
// - useBatch: Whether batch processing is enabled (based on config.batch.enabled)
// - err: Error when initialization fails
func buildExporter(ctx context.Context, c *conf.Tracer) (exp traceSdk.SpanExporter, batchOpts []traceSdk.BatchSpanProcessorOption, useBatch bool, err error) {
	// Get Tracer configuration
	cfg := c.GetConfig()
	// Note: cfg should be guaranteed non-nil when initialized in outer layer; directly read its fields here

	// Batch processing options
	if cfg != nil && cfg.Batch.GetEnabled() {
		// Enable batch processing
		useBatch = true
		// Maximum queue length: determines the queue capacity for spans to be exported
		if v := cfg.Batch.GetMaxQueueSize(); v > 0 {
			batchOpts = append(batchOpts, traceSdk.WithMaxQueueSize(int(v)))
		}
		// Scheduling delay: time interval for batch processor to trigger export periodically
		if d := cfg.Batch.GetScheduledDelay(); d != nil {
			batchOpts = append(batchOpts, traceSdk.WithBatchTimeout(d.AsDuration()))
		}
		// Export timeout: timeout for single batch export
		if d := cfg.Batch.GetExportTimeout(); d != nil {
			batchOpts = append(batchOpts, traceSdk.WithExportTimeout(d.AsDuration()))
		}
		// Maximum export batch size: maximum number of spans per flush
		if v := cfg.Batch.GetMaxBatchSize(); v > 0 {
			batchOpts = append(batchOpts, traceSdk.WithMaxExportBatchSize(int(v)))
		}
	}

	// Build exporter
	switch cfg.GetProtocol() {
	case conf.Protocol_OTLP_HTTP:
		// OTLP over HTTP
		opts := []otlptracehttp.Option{}
		// Target address: Collector's HTTP endpoint (usually host:4318), no need to include protocol
		if c.Addr != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(c.Addr))
		}
		// Plaintext HTTP: set to Insecure to use http (otherwise defaults to https)
		if cfg.GetInsecure() {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		// Custom URL path: default /v1/traces, can be specified via http_path
		if hp := cfg.GetHttpPath(); hp != "" {
			opts = append(opts, otlptracehttp.WithURLPath(hp))
		}
		// Additional request headers: commonly used for authentication (e.g., Authorization: Bearer <token>)
		if hdrs := cfg.GetHeaders(); len(hdrs) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(hdrs))
		}
		// Timeout
		if to := cfg.GetTimeout(); to != nil {
			opts = append(opts, otlptracehttp.WithTimeout(to.AsDuration()))
		}
		// Compression (gzip)
		if cfg.GetCompression() == conf.Compression_COMPRESSION_GZIP {
			opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
		}
		// Initialize HTTP exporter
		exp, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("create otlp http exporter: %w", err)
		}
		return exp, batchOpts, useBatch, nil

	default:
		// Default to gRPC
		opts := []otlptracegrpc.Option{}
		// Target address: Collector's gRPC endpoint (usually host:4317)
		if c.Addr != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(c.Addr))
		}

		// TLS / Insecure
		// Plaintext gRPC: do not enable TLS
		if cfg.GetInsecure() {
			opts = append(opts, otlptracegrpc.WithInsecure())
			// Otherwise load credentials based on TLS configuration; return error directly if configuration is incorrect
		} else if tlsOpt, tlsErr := buildTLSCredentials(cfg); tlsErr == nil && tlsOpt != nil {
			opts = append(opts, *tlsOpt)
		} else if tlsErr != nil {
			return nil, nil, false, tlsErr
		}

		// Headers
		// Additional gRPC Metadata: e.g., authentication headers, tenant information, etc.
		if hdrs := cfg.GetHeaders(); len(hdrs) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(hdrs))
		}

		// Timeout
		// Request timeout within exporter
		if to := cfg.GetTimeout(); to != nil {
			opts = append(opts, otlptracegrpc.WithTimeout(to.AsDuration()))
		}

		// Retry
		// gRPC retry strategy: supports initial/max backoff intervals; maximum retry time is used to approximate limit on maximum retry attempts
		if r := cfg.GetRetry(); r != nil {
			rc := otlptracegrpc.RetryConfig{Enabled: r.GetEnabled()}
			if ii := r.GetInitialInterval(); ii != nil {
				rc.InitialInterval = ii.AsDuration()
			}
			if mi := r.GetMaxInterval(); mi != nil {
				rc.MaxInterval = mi.AsDuration()
			}
			// Approximately map max_attempts to maximum retry duration (MaxElapsedTime) to limit total retry attempts
			if ma := r.GetMaxAttempts(); ma > 0 {
				// Prefer to use MaxInterval as upper bound for each wait; if not set, fall back to InitialInterval; then fall back to 5s
				chosen := rc.MaxInterval
				if chosen == 0 {
					chosen = rc.InitialInterval
				}
				if chosen == 0 {
					chosen = 5 * time.Second
				}
				rc.MaxElapsedTime = time.Duration(ma) * chosen
			}
			opts = append(opts, otlptracegrpc.WithRetry(rc))
		}

		// Compression (gzip)
		// gRPC compression: can be enabled to reduce bandwidth when Collector supports gzip
		if cfg.GetCompression() == conf.Compression_COMPRESSION_GZIP {
			opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
		}

		// Initialize gRPC exporter
		exp, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("create otlp grpc exporter: %w", err)
		}
		return exp, batchOpts, useBatch, nil
	}
}

// buildTLSCredentials builds gRPC TLS credentials (when insecure=false).
// Supports:
// - CA file (root certificate): ca_file
// - Client mutual authentication: cert_file + key_file
// - InsecureSkipVerify: whether to skip server certificate verification
// Returns the option corresponding to otlptracegrpc.WithTLSCredentials; returns (nil, nil) if TLS is not configured.
func buildTLSCredentials(cfg *conf.Config) (*otlptracegrpc.Option, error) {
	// Get TLS configuration
	tlsCfg := cfg.GetTls()
	if tlsCfg == nil {
		return nil, nil
	}

	// CA
	// If ca_file is provided, load root certificate to RootCAs; used to verify server certificate
	var rootCAs *x509.CertPool
	if tlsCfg.GetCaFile() != "" {
		pem, err := os.ReadFile(tlsCfg.GetCaFile())
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}
		rootCAs = x509.NewCertPool()
		rootCAs.AppendCertsFromPEM(pem)
	}

	// Client cert
	// If both cert_file and key_file are provided, enable mTLS client certificate
	var certs []tls.Certificate
	if tlsCfg.GetCertFile() != "" && tlsCfg.GetKeyFile() != "" {
		cert, err := tls.LoadX509KeyPair(tlsCfg.GetCertFile(), tlsCfg.GetKeyFile())
		if err != nil {
			return nil, fmt.Errorf("load client key pair: %w", err)
		}
		certs = []tls.Certificate{cert}
	}

	// Assemble tls.Config; when InsecureSkipVerify=true, skip verification of server certificate (should only be used in controlled environments)
	tcfg := &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       certs,
		InsecureSkipVerify: tlsCfg.GetInsecureSkipVerify(),
	}

	creds := credentials.NewTLS(tcfg)
	opt := otlptracegrpc.WithTLSCredentials(creds)
	return &opt, nil
}
