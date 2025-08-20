// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/protobuf/proto"
)

// buildMiddlewares builds the middleware chain.
func (h *ServiceHttp) buildMiddlewares() []middleware.Middleware {
	var middlewares []middleware.Middleware

	// Base middlewares
	middlewares = append(middlewares,
		// Tracing middleware with tracer name set to the application name
		tracing.Server(tracing.WithTracerName(app.GetName())),
		// Logging middleware using the Lynx framework logger
		logging.Server(log.Logger),
		// Enhanced response wrapper middleware (integrated with metrics)
		TracerLogPackWithMetrics(h),
		// Request parameter validation middleware
		validate.ProtoValidate(),
		// Recovery middleware to handle panic during request processing
		h.recoveryMiddleware(),
	)

	// Security-related middlewares
	middlewares = append(middlewares, h.rateLimitMiddleware())

	// Configure rate limit middleware using Lynx control plane HTTP rate limit policy
	// If a rate limit middleware exists, append it
	if rl := app.Lynx().GetControlPlane().HTTPRateLimit(); rl != nil {
		middlewares = append(middlewares, rl)
	}

	return middlewares
}

// recoveryMiddleware returns a recovery middleware.
func (h *ServiceHttp) recoveryMiddleware() middleware.Middleware {
	return recovery.Recovery(
		recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
			log.ErrorCtx(ctx, "Panic recovered", "error", err)

			// Record error metrics
			if h.errorCounter != nil {
				method := "POST"
				path := "unknown"
				if tr, ok := transport.FromServerContext(ctx); ok {
					// Kratos HTTP transport commonly uses POST
					if m := tr.RequestHeader().Get("X-HTTP-Method"); m != "" {
						method = m
					}
					path = tr.Operation()
				}
				h.errorCounter.WithLabelValues(method, path, "panic").Inc()
			}

			return nil
		}),
	)
}

// rateLimitMiddleware returns a rate limit middleware.
func (h *ServiceHttp) rateLimitMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if h.rateLimiter != nil && !h.rateLimiter.Allow() {
				// Record rate limit metrics
				if h.errorCounter != nil {
					method := "POST"
					path := "unknown"
					if tr, ok := transport.FromServerContext(ctx); ok {
						if m := tr.RequestHeader().Get("X-HTTP-Method"); m != "" {
							method = m
						}
						path = tr.Operation()
					}
					h.errorCounter.WithLabelValues(method, path, "rate_limit_exceeded").Inc()
				}
				return nil, fmt.Errorf("rate limit exceeded")
			}
			return handler(ctx, req)
		}
	}
}

// metricsMiddleware returns a metrics middleware.
func (h *ServiceHttp) metricsMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			start := time.Now()

			// Get request information
			method := "unknown"
			path := "unknown"

			// Use Kratos transport to get request information
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.RequestHeader().Get("X-HTTP-Method")
				if method == "" {
					method = "POST" // Kratos uses POST by default
				}
				path = tr.Operation() // Operation path
			}

			// Handle the request
			reply, err = handler(ctx, req)

			// Record metrics
			duration := time.Since(start).Seconds()
			if h.requestDuration != nil {
				h.requestDuration.WithLabelValues(method, path).Observe(duration)
			}

			// Record request count
			if h.requestCounter != nil {
				status := "success"
				if err != nil {
					status = "error"
				}
				h.requestCounter.WithLabelValues(method, path, status).Inc()
			}

			// Record response size
			if h.responseSize != nil && reply != nil {
				// Try to get response size
				if msg, ok := reply.(proto.Message); ok {
					if data, err := proto.Marshal(msg); err == nil {
						h.responseSize.WithLabelValues(method, path).Observe(float64(len(data)))
					}
				}
			}

			return reply, err
		}
	}
}
