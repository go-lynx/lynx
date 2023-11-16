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

type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    int         `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string      `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Data    interface{} `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

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

func ResponsePack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				defer func() {
					// return context traceId
					tr.ReplyHeader().Set("Tid", trace.SpanContextFromContext(ctx).TraceID().String())
					tr.ReplyHeader().Set("Content-Type", "application/json")
				}()
			}
			return handler(ctx, req)
		}
	}
}
