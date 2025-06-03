package http

import (
	"context"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"
)

// ResponsePack 返回一个中间件，用于向响应添加跟踪 ID 和内容类型头。
// 它从上下文提取跟踪 ID，并将其作为 "TraceID" 设置到响应头中。
func ResponsePack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// 尝试从上下文获取服务器传输信息
			if tr, ok := transport.FromServerContext(ctx); ok {
				defer func() {
					// 将上下文的跟踪 ID、跨度 ID 添加到响应头
					tr.ReplyHeader().Set("Trace-Id", trace.SpanContextFromContext(ctx).TraceID().String())
					tr.ReplyHeader().Set("Span-Id", trace.SpanContextFromContext(ctx).SpanID().String())
					// 设置响应的内容类型为 JSON
					tr.ReplyHeader().Set("Content-Type", "application/json")
				}()
			}
			// 调用下一个处理函数
			return handler(ctx, req)
		}
	}
}
