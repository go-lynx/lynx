// Package http 实现了 HTTP 相关的功能，包括响应编码和中间件。
package http

import (
	"context"
	"encoding/json"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/runtime/protoimpl"
)

// Response 表示标准化的 HTTP 响应结构。
// 它包含状态码、消息和可选的数据负载。
type Response struct {
	// state 是 protobuf 消息的状态，用于内部处理。
	state protoimpl.MessageState
	// sizeCache 缓存消息的大小，用于优化序列化性能。
	sizeCache protoimpl.SizeCache
	// unknownFields 存储解析过程中遇到的未知字段。
	unknownFields protoimpl.UnknownFields

	// Code 是响应的状态码。
	Code int `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	// Message 是响应的描述消息。
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	// Data 是响应携带的具体数据。
	Data interface{} `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

// ResponseEncoder 将响应数据编码为标准化的 JSON 格式。
// 它将数据封装在一个状态码为 200、消息为 "success" 的 Response 结构体中。
// w 是 HTTP 响应写入器，用于向客户端发送响应。
// r 是 HTTP 请求对象，当前未使用。
// data 是要编码的响应数据。
// 返回编码过程中可能出现的错误。
func ResponseEncoder(w http.ResponseWriter, r *http.Request, data interface{}) error {
	// 创建一个标准化的响应结构体
	res := &Response{
		Code:    200,
		Message: "success",
		Data:    data,
	}
	// 将响应结构体序列化为 JSON 字节切片
	msRes, err := json.Marshal(res)
	if err != nil {
		// 序列化失败，返回错误
		return err
	}
	// 将 JSON 数据写入 HTTP 响应
	_, err = w.Write(msRes)
	if err != nil {
		// 写入失败，返回错误
		return err
	}
	return nil
}

// ResponsePack 返回一个中间件，用于向响应添加跟踪 ID 和内容类型头。
// 它从上下文提取跟踪 ID，并将其作为 "TraceID" 设置到响应头中。
func ResponsePack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// 尝试从上下文获取服务器传输信息
			if tr, ok := transport.FromServerContext(ctx); ok {
				defer func() {
					// 将上下文的跟踪 ID、跨度 ID 添加到响应头
					tr.ReplyHeader().Set("TraceID", trace.SpanContextFromContext(ctx).TraceID().String())
					tr.ReplyHeader().Set("SpanID", trace.SpanContextFromContext(ctx).SpanID().String())
					// 设置响应的内容类型为 JSON
					tr.ReplyHeader().Set("Content-Type", "application/json")
				}()
			}
			// 调用下一个处理函数
			return handler(ctx, req)
		}
	}
}
