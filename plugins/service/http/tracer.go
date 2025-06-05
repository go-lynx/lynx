package http

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app/log"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"time"
)

// TracerLogPack 返回一个中间件，用于向响应添加跟踪 ID 和内容类型头。
// 它从上下文提取跟踪 ID，并将其作为 "TraceID" 设置到响应头中。
func TracerLogPack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			start := time.Now()
			span := trace.SpanContextFromContext(ctx)
			traceID := span.TraceID().String()
			spanID := span.SpanID().String()

			var method, endpoint, path, clientIP string
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.Operation()
				endpoint = tr.Endpoint()
				path = tr.RequestHeader().Get("Path")
				clientIP = tr.RequestHeader().Get("X-Forwarded-For")
				if clientIP == "" {
					clientIP = tr.RequestHeader().Get("X-Real-IP")
				}

				// 设置响应头
				defer func() {
					tr.ReplyHeader().Set("Trace-Id", traceID)
					tr.ReplyHeader().Set("Span-Id", spanID)
					tr.ReplyHeader().Set("Content-Type", "application/json")
				}()
			}

			// 打印请求体（如果是 proto message）
			var body []byte
			if msg, ok := req.(proto.Message); ok {
				body, _ = protojson.Marshal(msg)
				log.DebugfCtx(ctx, "[HTTP Request] method=%s endpoint=%s path=%s client-ip=%s body=%s",
					method, endpoint, path, clientIP, body)
			} else {
				log.DebugfCtx(ctx, "[HTTP Request] method=%s endpoint=%s path=%s client-ip=%s body=%#v",
					method, endpoint, path, clientIP, req)
			}

			reply, err = handler(ctx, req)

			log.DebugfCtx(ctx, "[HTTP Response] method=%s endpoint=%s path=%s duration=%v error=%v body=%#v",
				method, endpoint, path, time.Since(start), err, reply)

			return reply, err
		}
	}
}
