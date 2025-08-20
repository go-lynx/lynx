package http

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app/log"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	maxBodySize     = 1024 * 1024 // 1MB
	contentTypeKey  = "Content-Type"
	jsonContentType = "application/json"

	httpRequestLogFormat  = "[HTTP Request] api=%s endpoint=%s client-ip=%s headers=%s body=%s"
	httpResponseLogFormat = "[HTTP Response] api=%s endpoint=%s duration=%v error=%v headers=%s body=%s"
)

// getClientIP 获取客户端 IP 地址
func getClientIP(header transport.Header) string {
	for _, key := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if ip := header.Get(key); ip != "" {
			return ip
		}
	}
	return "unknown"
}

// safeProtoToJSON 安全地将 proto 消息转换为 JSON
func safeProtoToJSON(msg proto.Message) (string, error) {
	body, err := protojson.Marshal(msg)
	if err != nil {
		return "", err
	}
	if len(body) > maxBodySize {
		return fmt.Sprintf("<body too large, size: %d bytes>", len(body)), nil
	}
	return string(body), nil
}

// TracerLogPack 返回一个中间件，用于向响应添加跟踪 ID 和内容类型头。
// 它从上下文提取跟踪 ID，并将其作为 "TraceID" 设置到响应头中。
func TracerLogPack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// 检查上下文是否已取消
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			start := time.Now()
			span := trace.SpanContextFromContext(ctx)
			traceID := span.TraceID().String()
			spanID := span.SpanID().String()

			var tr transport.Transporter
			var ok bool

			// 提取请求信息
			if tr, ok = transport.FromServerContext(ctx); !ok {
				// 无法获取 transport，但仍然记录基本信息
				log.WarnfCtx(ctx, "Failed to get transport from context, proceeding without tracing")
				return handler(ctx, req)
			}

			endpoint := tr.Endpoint()
			clientIP := getClientIP(tr.RequestHeader())
			api := tr.Operation()

			// 设置响应头
			defer func() {
				header := tr.ReplyHeader()
				header.Set("Trace-Id", traceID)
				header.Set("Span-Id", spanID)

				// 安全地检查响应类型并设置 Content-Type
				if _, ok := reply.(proto.Message); ok {
					header.Set(contentTypeKey, jsonContentType)
				}
			}()

			// 记录请求日志
			var reqBody string
			if msg, ok := req.(proto.Message); ok {
				if body, err := safeProtoToJSON(msg); err == nil {
					reqBody = body
				} else {
					reqBody = fmt.Sprintf("<failed to marshal request: %v>", err)
				}
			} else {
				reqBody = fmt.Sprintf("%#v", req)
			}

			// 获取所有请求头
			headers := make(map[string]string)
			for _, key := range tr.RequestHeader().Keys() {
				headers[key] = tr.RequestHeader().Get(key)
			}
			headersStr := fmt.Sprintf("%#v", headers)

			// 使用 Info 级别记录请求日志，便于生产环境监控
			log.InfofCtx(ctx, httpRequestLogFormat, api, endpoint, clientIP, headersStr, reqBody)

			// 处理请求
			reply, err = handler(ctx, req)

			// 记录响应日志
			var respBody string
			if msg, ok := reply.(proto.Message); ok {
				if body, err := safeProtoToJSON(msg); err == nil {
					respBody = body
				} else {
					respBody = fmt.Sprintf("<failed to marshal response: %v>", err)
				}
			} else {
				respBody = fmt.Sprintf("%#v", reply)
			}

			// 获取所有响应头
			respHeaders := make(map[string]string)
			for _, key := range tr.ReplyHeader().Keys() {
				respHeaders[key] = tr.ReplyHeader().Get(key)
			}
			respHeadersStr := fmt.Sprintf("%#v", respHeaders)

			// 根据是否有错误选择日志级别
			duration := time.Since(start)
			if err != nil {
				log.ErrorfCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			} else {
				log.InfofCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			}

			return reply, err
		}
	}
}

// TracerLogPackWithMetrics 返回一个增强的中间件，集成了追踪、日志和监控指标
func TracerLogPackWithMetrics(service *ServiceHttp) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// 检查上下文是否已取消
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			start := time.Now()
			span := trace.SpanContextFromContext(ctx)
			traceID := span.TraceID().String()
			spanID := span.SpanID().String()

			var tr transport.Transporter
			var ok bool

			// 提取请求信息
			if tr, ok = transport.FromServerContext(ctx); !ok {
				// 无法获取 transport，但仍然记录基本信息
				log.WarnfCtx(ctx, "Failed to get transport from context, proceeding without tracing")
				return handler(ctx, req)
			}

			endpoint := tr.Endpoint()
			clientIP := getClientIP(tr.RequestHeader())
			api := tr.Operation()

			// 设置响应头
			defer func() {
				header := tr.ReplyHeader()
				header.Set("Trace-Id", traceID)
				header.Set("Span-Id", spanID)

				// 安全地检查响应类型并设置 Content-Type
				if _, ok := reply.(proto.Message); ok {
					header.Set(contentTypeKey, jsonContentType)
				}
			}()

			// 记录请求日志
			var reqBody string
			if msg, ok := req.(proto.Message); ok {
				if body, err := safeProtoToJSON(msg); err == nil {
					reqBody = body
				} else {
					reqBody = fmt.Sprintf("<failed to marshal request: %v>", err)
				}
			} else {
				reqBody = fmt.Sprintf("%#v", req)
			}

			// 获取所有请求头
			headers := make(map[string]string)
			for _, key := range tr.RequestHeader().Keys() {
				headers[key] = tr.RequestHeader().Get(key)
			}
			headersStr := fmt.Sprintf("%#v", headers)

			// 使用 Info 级别记录请求日志，便于生产环境监控
			log.InfofCtx(ctx, httpRequestLogFormat, api, endpoint, clientIP, headersStr, reqBody)

			// Inflight + 请求大小埋点
			if service != nil && service.inflightRequests != nil {
				service.inflightRequests.WithLabelValues(api).Inc()
				defer service.inflightRequests.WithLabelValues(api).Dec()
			}

			if service != nil && service.requestSize != nil {
				if msg, ok := req.(proto.Message); ok {
					if data, e := proto.Marshal(msg); e == nil {
						service.requestSize.WithLabelValues("POST", api).Observe(float64(len(data)))
					}
				}
			}

			// 处理请求
			reply, err = handler(ctx, req)

			// 记录响应日志
			var respBody string
			if msg, ok := reply.(proto.Message); ok {
				if body, err := safeProtoToJSON(msg); err == nil {
					respBody = body
				} else {
					respBody = fmt.Sprintf("<failed to marshal response: %v>", err)
				}
			} else {
				respBody = fmt.Sprintf("%#v", reply)
			}

			// 获取所有响应头
			respHeaders := make(map[string]string)
			for _, key := range tr.ReplyHeader().Keys() {
				respHeaders[key] = tr.ReplyHeader().Get(key)
			}
			respHeadersStr := fmt.Sprintf("%#v", respHeaders)

			// 根据是否有错误选择日志级别
			duration := time.Since(start)
			if err != nil {
				log.ErrorfCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			} else {
				log.InfofCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			}

			// 记录监控指标（如果服务实例可用）
			if service != nil {
				// 记录请求持续时间
				if service.requestDuration != nil {
					service.requestDuration.WithLabelValues("POST", api).Observe(duration.Seconds())
				}

				// 记录请求计数
				if service.requestCounter != nil {
					status := "success"
					if err != nil {
						status = "error"
					}
					service.requestCounter.WithLabelValues("POST", api, status).Inc()
				}

				// 记录响应大小
				if service.responseSize != nil && reply != nil {
					if msg, ok := reply.(proto.Message); ok {
						if data, err := proto.Marshal(msg); err == nil {
							service.responseSize.WithLabelValues("POST", api).Observe(float64(len(data)))
						}
					}
				}

				// 记录错误
				if err != nil && service.errorCounter != nil {
					service.errorCounter.WithLabelValues("POST", api, "tracer_error").Inc()
				}
			}

			return reply, err
		}
	}
}
