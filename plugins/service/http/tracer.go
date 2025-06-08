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
				return handler(ctx, req)
			}

			endpoint := tr.Endpoint()
			clientIP := getClientIP(tr.RequestHeader())

			// 设置响应头
			defer func() {
				header := tr.ReplyHeader()
				header.Set("Trace-Id", traceID)
				header.Set("Span-Id", spanID)
				// 只在确实是 JSON 响应时设置 Content-Type
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
			api := tr.Operation()

			log.DebugfCtx(ctx, httpRequestLogFormat, api, endpoint, clientIP, headersStr, reqBody)

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

			// 使用相同的 API 名称
			log.DebugfCtx(ctx, httpResponseLogFormat,
				api, endpoint, time.Since(start), err, respHeadersStr, respBody)

			return reply, err
		}
	}
}
