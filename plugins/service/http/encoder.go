// Package http 实现了 HTTP 相关的功能，包括响应编码和中间件。
package http

import (
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/protobuf/runtime/protoimpl"
	nhttp "net/http"
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
	codec, _ := http.CodecForRequest(r, "Accept")
	body, err := codec.Marshal(res)
	if err != nil {
		w.WriteHeader(nhttp.StatusInternalServerError)
		return err
	}
	// 将 JSON 数据写入 HTTP 响应
	_, err = w.Write(body)
	if err != nil {
		// 写入失败，返回错误
		return err
	}
	return nil
}

func EncodeErrorFunc(w http.ResponseWriter, r *http.Request, err error) {
	// 拿到error并转换成kratos Error实体
	se := errors.FromError(err)
	res := &Response{
		Code:    int(se.Code),
		Message: se.Message,
	}
	codec, _ := http.CodecForRequest(r, "Accept")
	body, err := codec.Marshal(res)
	if err != nil {
		w.WriteHeader(nhttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// 设置HTTP Status Code
	w.WriteHeader(nhttp.StatusInternalServerError)
	_, wErr := w.Write(body)
	if wErr != nil {
		log.Error("write error", wErr)
	}
}
