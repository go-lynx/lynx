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

// getClientIP returns the client IP address.
func getClientIP(header transport.Header) string {
	for _, key := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if ip := header.Get(key); ip != "" {
			return ip
		}
	}
	return "unknown"
}

// safeProtoToJSON safely marshals a proto message to JSON.
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

// TracerLogPack returns middleware that adds trace IDs and Content-Type headers to the response.
// It extracts trace information from context and sets "Trace-Id" and "Span-Id" in response headers.
func TracerLogPack() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Check if the context has been canceled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			start := time.Now()
			span := trace.SpanContextFromContext(ctx)
			traceID := span.TraceID().String()
			spanID := span.SpanID().String()

			var tr transport.Transporter
			var ok bool

			// Extract request information
			if tr, ok = transport.FromServerContext(ctx); !ok {
				// Transport not available; still log basic information
				log.WarnfCtx(ctx, "Failed to get transport from context, proceeding without tracing")
				return handler(ctx, req)
			}

			endpoint := tr.Endpoint()
			clientIP := getClientIP(tr.RequestHeader())
			api := tr.Operation()

			// Set response headers
			defer func() {
				header := tr.ReplyHeader()
				header.Set("Trace-Id", traceID)
				header.Set("Span-Id", spanID)

				// Safely check response type and set Content-Type
				if _, ok := reply.(proto.Message); ok {
					header.Set(contentTypeKey, jsonContentType)
				}
			}()

			// Log the request
			var reqBody string
			if msg, ok := req.(proto.Message); ok {
				if body, jsonErr := safeProtoToJSON(msg); jsonErr == nil {
					reqBody = body
				} else {
					reqBody = fmt.Sprintf("<failed to marshal request: %v>", jsonErr)
				}
			} else {
				reqBody = fmt.Sprintf("%#v", req)
			}

			// Collect all request headers
			headers := make(map[string]string)
			for _, key := range tr.RequestHeader().Keys() {
				headers[key] = tr.RequestHeader().Get(key)
			}
			headersStr := fmt.Sprintf("%#v", headers)

			// Log with Info level for production monitoring
			log.InfofCtx(ctx, httpRequestLogFormat, api, endpoint, clientIP, headersStr, reqBody)

			// Handle the request
			reply, err = handler(ctx, req)

			// Log the response
			var respBody string
			if msg, ok := reply.(proto.Message); ok {
				if body, jsonErr := safeProtoToJSON(msg); jsonErr == nil {
					respBody = body
				} else {
					respBody = fmt.Sprintf("<failed to marshal response: %v>", jsonErr)
				}
			} else {
				respBody = fmt.Sprintf("%#v", reply)
			}

			// Collect all response headers
			respHeaders := make(map[string]string)
			for _, key := range tr.ReplyHeader().Keys() {
				respHeaders[key] = tr.ReplyHeader().Get(key)
			}
			respHeadersStr := fmt.Sprintf("%#v", respHeaders)

			// Choose log level based on presence of error
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

// TracerLogPackWithMetrics returns an enhanced middleware that integrates tracing, logging, and monitoring metrics.
func TracerLogPackWithMetrics(service *ServiceHttp) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Check if the context has been canceled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			start := time.Now()
			span := trace.SpanContextFromContext(ctx)
			traceID := span.TraceID().String()
			spanID := span.SpanID().String()

			var tr transport.Transporter
			var ok bool

			// Extract request information
			if tr, ok = transport.FromServerContext(ctx); !ok {
				// Transport not available; still log basic information
				log.WarnfCtx(ctx, "Failed to get transport from context, proceeding without tracing")
				return handler(ctx, req)
			}

			endpoint := tr.Endpoint()
			clientIP := getClientIP(tr.RequestHeader())
			api := tr.Operation()

			// Set response headers
			defer func() {
				header := tr.ReplyHeader()
				header.Set("Trace-Id", traceID)
				header.Set("Span-Id", spanID)

				// Safely check response type and set Content-Type
				if _, ok := reply.(proto.Message); ok {
					header.Set(contentTypeKey, jsonContentType)
				}
			}()

			// Log the request
			var reqBody string
			if msg, ok := req.(proto.Message); ok {
				if body, jsonErr := safeProtoToJSON(msg); jsonErr == nil {
					reqBody = body
				} else {
					reqBody = fmt.Sprintf("<failed to marshal request: %v>", jsonErr)
				}
			} else {
				reqBody = fmt.Sprintf("%#v", req)
			}

			// Collect all request headers
			headers := make(map[string]string)
			for _, key := range tr.RequestHeader().Keys() {
				headers[key] = tr.RequestHeader().Get(key)
			}
			headersStr := fmt.Sprintf("%#v", headers)

			// Log with Info level for production monitoring
			log.InfofCtx(ctx, httpRequestLogFormat, api, endpoint, clientIP, headersStr, reqBody)

			// Inflight counter and request size metrics
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

			// Handle the request
			reply, err = handler(ctx, req)

			// Log the response
			var respBody string
			if msg, ok := reply.(proto.Message); ok {
				if body, jsonErr := safeProtoToJSON(msg); jsonErr == nil {
					respBody = body
				} else {
					respBody = fmt.Sprintf("<failed to marshal response: %v>", jsonErr)
				}
			} else {
				respBody = fmt.Sprintf("%#v", reply)
			}

			// Collect all response headers
			respHeaders := make(map[string]string)
			for _, key := range tr.ReplyHeader().Keys() {
				respHeaders[key] = tr.ReplyHeader().Get(key)
			}
			respHeadersStr := fmt.Sprintf("%#v", respHeaders)

			// Choose log level based on presence of error
			duration := time.Since(start)
			if err != nil {
				log.ErrorfCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			} else {
				log.InfofCtx(ctx, httpResponseLogFormat,
					api, endpoint, duration, err, respHeadersStr, respBody)
			}

			// Record monitoring metrics (if the service instance is available)
			if service != nil {
				// Record request duration
				if service.requestDuration != nil {
					service.requestDuration.WithLabelValues("POST", api).Observe(duration.Seconds())
				}

				// Record request count
				if service.requestCounter != nil {
					status := "success"
					if err != nil {
						status = "error"
					}
					service.requestCounter.WithLabelValues("POST", api, status).Inc()
				}

				// Record response size
				if service.responseSize != nil && reply != nil {
					if msg, ok := reply.(proto.Message); ok {
						if data, marshalErr := proto.Marshal(msg); marshalErr == nil {
							service.responseSize.WithLabelValues("POST", api).Observe(float64(len(data)))
						}
					}
				}

				// Record errors
				if err != nil && service.errorCounter != nil {
					service.errorCounter.WithLabelValues("POST", api, "tracer_error").Inc()
				}
			}

			return reply, err
		}
	}
}
