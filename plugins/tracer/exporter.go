package tracer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"
	"os"

	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
)

// buildExporter 根据 Tracer 配置构建 OTLP Trace 导出器以及批处理（BatchSpanProcessor）选项。
// 特性：
// - 协议：gRPC（默认）或 HTTP（通过 config.protocol 指定）
// - 连接：addr、insecure/TLS、headers、自定义 http_path（HTTP）
// - 可靠性：timeout、retry（gRPC 支持 initial/max interval）
// - 压缩：gzip（gRPC/HTTP）
// - 批处理：queue size、batch size、导出超时、调度延迟
// 返回：
// - exp: 已初始化的 SpanExporter
// - batchOpts: 批处理器可选项
// - useBatch: 是否启用批处理（依据 config.batch.enabled）
// - err: 初始化失败时的错误
func buildExporter(ctx context.Context, c *conf.Tracer) (exp traceSdk.SpanExporter, batchOpts []traceSdk.BatchSpanProcessorOption, useBatch bool, err error) {
	// 获取 Tracer 配置
	cfg := c.GetConfig()
	// 说明：cfg 在外层初始化时应保证非空；此处直接读取其字段

	// 批处理选项
	if cfg != nil && cfg.Batch.GetEnabled() {
		// 启用批处理
		useBatch = true
		// 最大队列长度：决定待导出 span 的队列容量
		if v := cfg.Batch.GetMaxQueueSize(); v > 0 {
			batchOpts = append(batchOpts, traceSdk.WithMaxQueueSize(int(v)))
		}
		// 调度延迟：批处理器定期触发导出的时间间隔
		if d := cfg.Batch.GetScheduledDelay(); d != nil {
			batchOpts = append(batchOpts, traceSdk.WithBatchTimeout(d.AsDuration()))
		}
		// 导出超时：单次批量导出的超时时间
		if d := cfg.Batch.GetExportTimeout(); d != nil {
			batchOpts = append(batchOpts, traceSdk.WithExportTimeout(d.AsDuration()))
		}
		// 单批最大导出数量：每次 flush 的最大 span 数
		if v := cfg.Batch.GetMaxBatchSize(); v > 0 {
			batchOpts = append(batchOpts, traceSdk.WithMaxExportBatchSize(int(v)))
		}
	}

	// 构建导出器
	switch cfg.GetProtocol() {
	case conf.Protocol_OTLP_HTTP:
		// OTLP over HTTP
		opts := []otlptracehttp.Option{}
		// 目标地址：Collector 的 HTTP 端点（通常是 host:4318），无需包含协议
		if c.Addr != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(c.Addr))
		}
		// 明文 HTTP：设置为 Insecure 表示使用 http（否则默认为 https）
		if cfg.GetInsecure() {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		// 自定义 URL 路径：默认 /v1/traces，可通过 http_path 指定
		if hp := cfg.GetHttpPath(); hp != "" {
			opts = append(opts, otlptracehttp.WithURLPath(hp))
		}
		// 附加请求头：常用于鉴权（例如 Authorization: Bearer <token>）
		if hdrs := cfg.GetHeaders(); len(hdrs) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(hdrs))
		}
		// 超时
		if to := cfg.GetTimeout(); to != nil {
			opts = append(opts, otlptracehttp.WithTimeout(to.AsDuration()))
		}
		// 压缩（gzip）
		if cfg.GetCompression() == conf.Compression_COMPRESSION_GZIP {
			opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
		}
		// 初始化 HTTP 导出器
		exp, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("create otlp http exporter: %w", err)
		}
		return exp, batchOpts, useBatch, nil

	default:
		// 默认走 gRPC
		opts := []otlptracegrpc.Option{}
		// 目标地址：Collector 的 gRPC 端点（通常是 host:4317）
		if c.Addr != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(c.Addr))
		}

		// TLS / Insecure
		// 明文 gRPC：不启用 TLS
		if cfg.GetInsecure() {
			opts = append(opts, otlptracegrpc.WithInsecure())
		// 否则根据 TLS 配置加载凭证；若配置错误则直接返回错误
		} else if tlsOpt, tlsErr := buildTLSCredentials(cfg); tlsErr == nil && tlsOpt != nil {
			opts = append(opts, *tlsOpt)
		} else if tlsErr != nil {
			return nil, nil, false, tlsErr
		}

		// Headers
		// 附加 gRPC Metadata：例如认证头、租户信息等
		if hdrs := cfg.GetHeaders(); len(hdrs) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(hdrs))
		}

		// Timeout
		// 导出器内部的请求超时时间
		if to := cfg.GetTimeout(); to != nil {
			opts = append(opts, otlptracegrpc.WithTimeout(to.AsDuration()))
		}

		// Retry
		// gRPC 重试策略：支持初始/最大退避间隔；最大重试时间用于近似限制最大重试次数
		if r := cfg.GetRetry(); r != nil {
			rc := otlptracegrpc.RetryConfig{Enabled: r.GetEnabled()}
			if ii := r.GetInitialInterval(); ii != nil {
				rc.InitialInterval = ii.AsDuration()
			}
			if mi := r.GetMaxInterval(); mi != nil {
				rc.MaxInterval = mi.AsDuration()
			}
			// 近似将 max_attempts 映射为最大重试时长（MaxElapsedTime），以限制总体重试次数
			if ma := r.GetMaxAttempts(); ma > 0 {
				// 优先使用 MaxInterval 作为每次等待的上界；如未设置则回退到 InitialInterval；再回退到 5s
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
		// gRPC 压缩：当 Collector 支持 gzip 时可启用以减少带宽
		if cfg.GetCompression() == conf.Compression_COMPRESSION_GZIP {
			opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
		}

		// 初始化 gRPC 导出器
		exp, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("create otlp grpc exporter: %w", err)
		}
		return exp, batchOpts, useBatch, nil
	}
}

// buildTLSCredentials 构建 gRPC TLS 凭证（当 insecure=false 时）。
// 支持：
// - CA 文件（根证书）：ca_file
// - 客户端双向认证：cert_file + key_file
// - InsecureSkipVerify：是否跳过服务端证书校验
// 返回 otlptracegrpc.WithTLSCredentials 对应的选项；若未配置 TLS 则返回 (nil, nil)。
func buildTLSCredentials(cfg *conf.Config) (*otlptracegrpc.Option, error) {
	// 获取 TLS 配置
	tlsCfg := cfg.GetTls()
	if tlsCfg == nil {
		return nil, nil
	}

	// CA
	// 若提供 ca_file，则将根证书加载到 RootCAs；用于验证服务端证书
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
	// 若同时提供 cert_file 与 key_file，则启用 mTLS 客户端证书
	var certs []tls.Certificate
	if tlsCfg.GetCertFile() != "" && tlsCfg.GetKeyFile() != "" {
		cert, err := tls.LoadX509KeyPair(tlsCfg.GetCertFile(), tlsCfg.GetKeyFile())
		if err != nil {
			return nil, fmt.Errorf("load client key pair: %w", err)
		}
		certs = []tls.Certificate{cert}
	}

	// 组装 tls.Config；当 InsecureSkipVerify=true 时，将跳过对服务端证书的校验（仅应在受控环境使用）
	tcfg := &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       certs,
		InsecureSkipVerify: tlsCfg.GetInsecureSkipVerify(),
	}

	creds := credentials.NewTLS(tcfg)
	opt := otlptracegrpc.WithTLSCredentials(creds)
	return &opt, nil
}
