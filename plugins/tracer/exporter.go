package tracer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// validateAddress validates the address format for OTLP endpoints
func validateAddress(addr string) error {
	if addr == "" {
		return nil // Empty address is valid (will use defaults)
	}

	// Check if it's a valid host:port format
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format '%s': must be host:port", addr)
	}

	if host == "" {
		return fmt.Errorf("invalid address format '%s': host cannot be empty", addr)
	}

	if port == "" {
		return fmt.Errorf("invalid address format '%s': port cannot be empty", addr)
	}

	// Validate port is numeric and in valid range
	if portNum, err := net.LookupPort("tcp", port); err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid port '%s' in address '%s': must be 1-65535", port, addr)
	}

	return nil
}

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

	// Handle case when config is nil
	if cfg == nil {
		// Use default configuration when config is nil
		cfg = &conf.Config{}
	}

	// Validate address format
	if addrErr := validateAddress(c.Addr); addrErr != nil {
		return nil, nil, false, fmt.Errorf("address validation failed: %w", addrErr)
	}

	// Batch processing options
	if cfg.Batch != nil && cfg.Batch.GetEnabled() {
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
			return nil, nil, false, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
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
			return nil, nil, false, fmt.Errorf("failed to build TLS credentials: %w", tlsErr)
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

		// Connection management configuration
		if conn := cfg.GetConnection(); conn != nil {
			// Set reconnection period
			if rp := conn.GetReconnectionPeriod(); rp != nil {
				opts = append(opts, otlptracegrpc.WithReconnectionPeriod(rp.AsDuration()))
			}

			// Set connection timeout and other connection options via dial options
			var dialOpts []grpc.DialOption

			// Connection timeout is now handled via context in exporter creation

			// Connection pool settings
			if conn.GetMaxConnIdleTime() != nil || conn.GetMaxConnAge() != nil || conn.GetMaxConnAgeGrace() != nil {
				// Build service config for connection pool management
				serviceConfig := buildConnectionPoolServiceConfig(conn)
				if serviceConfig != "" {
					dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(serviceConfig))
				}
			}

			// Apply dial options if any
			if len(dialOpts) > 0 {
				opts = append(opts, otlptracegrpc.WithDialOption(dialOpts...))
			}
		} else {
			// Set default reconnection period if not configured
			opts = append(opts, otlptracegrpc.WithReconnectionPeriod(5*time.Second))
		}

		// Load balancing configuration
		if lb := cfg.GetLoadBalancing(); lb != nil {
			var dialOpts []grpc.DialOption

			// Build service config for load balancing
			serviceConfig := buildLoadBalancingServiceConfig(lb)
			if serviceConfig != "" {
				dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(serviceConfig))
			}

			// Apply load balancing dial options
			if len(dialOpts) > 0 {
				opts = append(opts, otlptracegrpc.WithDialOption(dialOpts...))
			}
		} else {
			// Add default load balancing support
			opts = append(opts, otlptracegrpc.WithDialOption(
				grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
			))
		}

		// Compression (gzip)
		// gRPC compression: can be enabled to reduce bandwidth when Collector supports gzip
		if cfg.GetCompression() == conf.Compression_COMPRESSION_GZIP {
			opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
		}

		// Initialize gRPC exporter with connection timeout if configured
		exporterCtx := ctx
		var cancel context.CancelFunc
		if conn := cfg.GetConnection(); conn != nil {
			if ct := conn.GetConnectTimeout(); ct != nil {
				exporterCtx, cancel = context.WithTimeout(ctx, ct.AsDuration())
				defer cancel()
			}
		}

		exp, err = otlptracegrpc.New(exporterCtx, opts...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
		}
		return exp, batchOpts, useBatch, nil
	}
}

// validateFilePath validates file path security to prevent path traversal attacks
func validateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(filePath, "..") || strings.Contains(filePath, "//") {
		return fmt.Errorf("file path contains invalid characters: %s", filePath)
	}

	// Resolve the absolute path to check if it's within allowed directories
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for '%s': %w", filePath, err)
	}

	// Check if the file exists and is readable
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file does not exist or is not accessible: %s", absPath)
	}

	return nil
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
		// Validate and sanitize file path
		caFilePath := tlsCfg.GetCaFile()
		if err := validateFilePath(caFilePath); err != nil {
			return nil, fmt.Errorf("invalid CA file path: %w", err)
		}

		pem, err := os.ReadFile(caFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file '%s': %w", caFilePath, err)
		}
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("failed to parse CA file '%s': no valid certificates found", caFilePath)
		}
	}

	// Client cert
	// If both cert_file and key_file are provided, enable mTLS client certificate
	var certs []tls.Certificate
	if tlsCfg.GetCertFile() != "" && tlsCfg.GetKeyFile() != "" {
		// Validate and sanitize file paths
		certFilePath := tlsCfg.GetCertFile()
		keyFilePath := tlsCfg.GetKeyFile()

		if err := validateFilePath(certFilePath); err != nil {
			return nil, fmt.Errorf("invalid cert file path: %w", err)
		}
		if err := validateFilePath(keyFilePath); err != nil {
			return nil, fmt.Errorf("invalid key file path: %w", err)
		}

		cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client key pair (cert: '%s', key: '%s'): %w",
				certFilePath, keyFilePath, err)
		}
		certs = []tls.Certificate{cert}
	} else if tlsCfg.GetCertFile() != "" || tlsCfg.GetKeyFile() != "" {
		// Only one of cert_file or key_file is provided
		return nil, fmt.Errorf("both cert_file and key_file must be provided for client authentication, got cert: '%s', key: '%s'",
			tlsCfg.GetCertFile(), tlsCfg.GetKeyFile())
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

// buildConnectionPoolServiceConfig builds a gRPC service config string for connection pool management.
func buildConnectionPoolServiceConfig(conn *conf.Connection) string {
	var serviceConfig strings.Builder
	serviceConfig.WriteString(`{"loadBalancingConfig": [{"roundRobin": {}}]`)

	// Add connection pool settings
	if conn.GetMaxConnIdleTime() != nil {
		serviceConfig.WriteString(`,"maxConnIdleTime":`)
		serviceConfig.WriteString(fmt.Sprintf("%d", int(conn.GetMaxConnIdleTime().AsDuration().Seconds())))
	}
	if conn.GetMaxConnAge() != nil {
		serviceConfig.WriteString(`,"maxConnAge":`)
		serviceConfig.WriteString(fmt.Sprintf("%d", int(conn.GetMaxConnAge().AsDuration().Seconds())))
	}
	if conn.GetMaxConnAgeGrace() != nil {
		serviceConfig.WriteString(`,"maxConnAgeGrace":`)
		serviceConfig.WriteString(fmt.Sprintf("%d", int(conn.GetMaxConnAgeGrace().AsDuration().Seconds())))
	}

	serviceConfig.WriteString("}")
	return serviceConfig.String()
}

// buildLoadBalancingServiceConfig builds a gRPC service config string for load balancing.
func buildLoadBalancingServiceConfig(lb *conf.LoadBalancing) string {
	var serviceConfig strings.Builder

	// Build load balancing policy
	switch lb.GetPolicy() {
	case "round_robin":
		serviceConfig.WriteString(`{"loadBalancingConfig": [{"roundRobin": {}}]`)
	case "pick_first":
		serviceConfig.WriteString(`{"loadBalancingConfig": [{"pickFirst": {}}]`)
	case "least_conn":
		serviceConfig.WriteString(`{"loadBalancingConfig": [{"leastConn": {}}]`)
	default:
		// Default to round_robin
		serviceConfig.WriteString(`{"loadBalancingConfig": [{"roundRobin": {}}]`)
	}

	// Add health checking if enabled
	if lb.GetHealthCheck() {
		serviceConfig.WriteString(`,"healthCheckConfig": {"serviceName": "grpc.health.v1.Health"}`)
	}

	serviceConfig.WriteString("}")
	return serviceConfig.String()
}
