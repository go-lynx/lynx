package grpcx

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Incoming returns the incoming gRPC metadata from the context, if present.
func Incoming(ctx context.Context) (metadata.MD, bool) {
	return metadata.FromIncomingContext(ctx)
}

// Outgoing returns the outgoing gRPC metadata from the context, if present.
func Outgoing(ctx context.Context) (metadata.MD, bool) {
	return metadata.FromOutgoingContext(ctx)
}

// GetAll returns all values for key from outgoing and incoming metadata (outgoing first).
func GetAll(ctx context.Context, key string) []string {
	k := strings.ToLower(key)
	var out []string
	if md, ok := metadata.FromOutgoingContext(ctx); ok && md != nil {
		out = append(out, md.Get(k)...)
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok && md != nil {
		out = append(out, md.Get(k)...)
	}
	return out
}

// Get returns the first value for key from outgoing/incoming metadata.
func Get(ctx context.Context, key string) (string, bool) {
	vals := GetAll(ctx, key)
	if len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// AppendOutgoing appends values to the given key in the outgoing metadata.
func AppendOutgoing(ctx context.Context, key string, values ...string) context.Context {
	if len(values) == 0 {
		return ctx
	}
	k := strings.ToLower(key)
	kv := make([]string, 0, len(values)*2)
	for _, v := range values {
		kv = append(kv, k, v)
	}
	return metadata.AppendToOutgoingContext(ctx, kv...)
}

// SetOutgoing sets the values for key in the outgoing metadata, replacing any existing values.
func SetOutgoing(ctx context.Context, key string, values ...string) context.Context {
	k := strings.ToLower(key)
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	md[k] = append([]string(nil), values...)
	return metadata.NewOutgoingContext(ctx, md)
}

// WithOutgoing sets multiple key-value pairs in the outgoing metadata, replacing existing values.
func WithOutgoing(ctx context.Context, kv map[string]string) context.Context {
	if len(kv) == 0 {
		return ctx
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	for k, v := range kv {
		md[strings.ToLower(k)] = []string{v}
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// MergeOutgoing merges the provided metadata into the outgoing metadata, appending values for existing keys.
func MergeOutgoing(ctx context.Context, mdNew metadata.MD) context.Context {
	if mdNew == nil {
		return ctx
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	if md == nil {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	for k, vals := range mdNew {
		lk := strings.ToLower(k)
		md[lk] = append(md[lk], vals...)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// CopyIncomingToOutgoing copies selected or all incoming metadata keys into the outgoing metadata.
func CopyIncomingToOutgoing(ctx context.Context, keys ...string) context.Context {
	inc, ok := metadata.FromIncomingContext(ctx)
	if !ok || inc == nil || len(inc) == 0 {
		return ctx
	}
	if len(keys) == 0 {
		return MergeOutgoing(ctx, inc)
	}
	mdSel := metadata.MD{}
	for _, k := range keys {
		lk := strings.ToLower(k)
		if vals := inc.Get(lk); len(vals) > 0 {
			mdSel[lk] = append([]string(nil), vals...)
		}
	}
	return MergeOutgoing(ctx, mdSel)
}

// CopyHTTPRequestHeadersToOutgoing copies HTTP request headers into the outgoing gRPC metadata.
// If keys are provided, only those headers are copied; otherwise, all headers are copied.
// Header names are normalized to lowercase for gRPC metadata.
func CopyHTTPRequestHeadersToOutgoing(ctx context.Context, r *http.Request, keys ...string) context.Context {
	if r == nil {
		return ctx
	}
	return CopyHTTPHeadersToOutgoing(ctx, r.Header, keys...)
}

// CopyHTTPHeadersToOutgoing copies headers from an http.Header into outgoing gRPC metadata.
// If keys are provided, only those keys are copied; otherwise, all headers are copied.
// Header names are normalized to lowercase for gRPC metadata.
func CopyHTTPHeadersToOutgoing(ctx context.Context, h http.Header, keys ...string) context.Context {
	if len(h) == 0 {
		return ctx
	}
	if len(keys) == 0 {
		md := metadata.MD{}
		for k, vs := range h {
			lk := strings.ToLower(k)
			if len(vs) > 0 {
				md[lk] = append(md[lk], vs...)
			}
		}
		return MergeOutgoing(ctx, md)
	}
	mdSel := metadata.MD{}
	for _, k := range keys {
		vs := h.Values(k)
		if len(vs) > 0 {
			lk := strings.ToLower(k)
			mdSel[lk] = append(mdSel[lk], vs...)
		}
	}
	return MergeOutgoing(ctx, mdSel)
}

// SetHeader sets server header metadata on the current gRPC response.
func SetHeader(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	md := metadata.MD{}
	for k, v := range kv {
		md[strings.ToLower(k)] = []string{v}
	}
	return grpc.SetHeader(ctx, md)
}

// SetTrailer sets server trailer metadata on the current gRPC response.
func SetTrailer(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	md := metadata.MD{}
	for k, v := range kv {
		md[strings.ToLower(k)] = []string{v}
	}
	return grpc.SetTrailer(ctx, md)
}
