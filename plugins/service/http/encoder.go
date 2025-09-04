// Package http implements HTTP-related features, including response encoding and middleware.
package http

import (
	nhttp "net/http"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/protobuf/runtime/protoimpl"
)

// Response represents a standardized HTTP response structure.
// It contains the status code, message, and an optional data payload.
type Response struct {
	// state is the status of the protobuf message for internal handling.
	state protoimpl.MessageState
	// sizeCache caches the message size to optimize serialization performance.
	sizeCache protoimpl.SizeCache
	// unknownFields stores unknown fields encountered during parsing.
	unknownFields protoimpl.UnknownFields

	// Code is the response status code.
	Code int `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	// Message is the descriptive message of the response.
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	// Data is the payload carried by the response.
	Data interface{} `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

// ResponseEncoder encodes response data into a standardized JSON format.
// It wraps the data in a Response struct with code=200 and message="success".
// w is the HTTP response writer used to send the response to the client.
// r is the HTTP request object (currently unused).
// data is the response payload to encode.
// Returns an error if encoding fails.
func ResponseEncoder(w http.ResponseWriter, r *http.Request, data interface{}) error {
	// Create a standardized response structure
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
	// Write the JSON data to the HTTP response
	_, err = w.Write(body)
	if err != nil {
		// Writing failed; return the error
		return err
	}
	return nil
}

func EncodeErrorFunc(w http.ResponseWriter, r *http.Request, err error) {
	// Convert the error to a Kratos Error entity
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
	// Set HTTP Status Code
	w.WriteHeader(nhttp.StatusInternalServerError)
	_, wErr := w.Write(body)
	if wErr != nil {
		log.Error("write error", wErr)
	}
}
