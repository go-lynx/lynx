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

// Response represents a standardized HTTP response structure.
// It includes a status code, message, and optional data payload.
type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    int         `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string      `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Data    interface{} `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

// ResponseEncoder encodes the response data into a standardized JSON format.
// It wraps the data in a Response struct with a 200 status code and "success" message.
func ResponseEncoder(w http.ResponseWriter, r *http.Request, data interface{}) error {
	res := &Response{
		Code:    200,
		Message: "success",
		Data:    data,
	}
	msRes, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = w.Write(msRes)
	if err != nil {
		return err
	}
	return nil
}

// ResponsePack returns a middleware that adds trace ID and content type headers to the response.
// It extracts the trace ID from the context and sets it in the response header as "T-Id".
func ResponsePack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				defer func() {
					// return context traceId
					tr.ReplyHeader().Set("T-Id", trace.SpanContextFromContext(ctx).TraceID().String())
					tr.ReplyHeader().Set("Content-Type", "application/json")
				}()
			}
			return handler(ctx, req)
		}
	}
}
