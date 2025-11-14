// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"google.golang.org/protobuf/proto"
)

// buildMiddlewares builds the middleware chain based on configuration.
func (h *ServiceHttp) buildMiddlewares() []middleware.Middleware {
	var middlewares []middleware.Middleware

	// Create middleware configuration if not present
	if h.conf.Middleware == nil {
		h.conf.Middleware = &conf.MiddlewareConfig{
			EnableTracing:    true,
			EnableLogging:    true,
			EnableMetrics:    true,
			EnableRecovery:   true,
			EnableRateLimit:  true,
			EnableValidation: true,
		}
	}

	// Base middlewares - order matters!
	// Tracing middleware
	if h.conf.Middleware.EnableTracing {
		middlewares = append(middlewares, tracing.Server(tracing.WithTracerName(app.GetName())))
		log.Infof("Tracing middleware enabled")
	}

	// Logging middleware
	if h.conf.Middleware.EnableLogging {
		middlewares = append(middlewares, logging.Server(log.Logger))
		log.Infof("Logging middleware enabled")
	}

	// Metrics middleware
	if h.conf.Middleware.EnableMetrics {
		middlewares = append(middlewares, h.metricsMiddleware())
		log.Infof("Metrics middleware enabled")
	}

	// Enhanced response wrapper middleware (integrated with metrics)
	if h.conf.Middleware.EnableTracing && h.conf.Middleware.EnableLogging && h.conf.Middleware.EnableMetrics {
		middlewares = append(middlewares, TracerLogPackWithMetrics(h))
		log.Infof("TracerLogPackWithMetrics middleware enabled")
	}

	// Request parameter validation middleware
	if h.conf.Middleware.EnableValidation {
		middlewares = append(middlewares, validate.ProtoValidate())
		log.Infof("Validation middleware enabled")
	}

	// Recovery middleware
	if h.conf.Middleware.EnableRecovery {
		middlewares = append(middlewares, h.recoveryMiddleware())
		log.Infof("Recovery middleware enabled")
	}

	// Security-related middlewares
	if h.conf.Middleware.EnableRateLimit {
		middlewares = append(middlewares, h.rateLimitMiddleware())
		log.Infof("Rate limit middleware enabled")
	}

	// Connection limit middleware
	if h.maxConnections > 0 || h.maxConcurrentRequests > 0 {
		middlewares = append(middlewares, h.connectionLimitMiddleware())
		log.Infof("Connection limit middleware enabled")
	}

	// Circuit breaker middleware
	middlewares = append(middlewares, h.circuitBreakerMiddleware())
	log.Infof("Circuit breaker middleware enabled")

	// Configure rate limit middleware using Lynx control plane HTTP rate limit policy
	// If a rate limit middleware exists, append it
	if rl := app.Lynx().GetControlPlane().HTTPRateLimit(); rl != nil && h.conf.Middleware.EnableRateLimit {
		middlewares = append(middlewares, rl)
		log.Infof("Control plane rate limit middleware enabled")
	}

	// Custom middleware and ordering features will be implemented in future versions
	// These features require additional protobuf definitions and implementation logic
	// Tracked in enhancement request #HTTP-1234

	return middlewares
}

// connectionLimitMiddleware returns a connection limit middleware.
// The semaphores are initialized once and reused across all requests.
func (h *ServiceHttp) connectionLimitMiddleware() middleware.Middleware {
	// Initialize semaphores once (thread-safe)
	h.semInitOnce.Do(func() {
		if h.maxConnections > 0 {
			h.connectionSem = make(chan struct{}, h.maxConnections)
			log.Infof("Initialized connection semaphore with capacity: %d", h.maxConnections)
		}
		if h.maxConcurrentRequests > 0 {
			h.requestSem = make(chan struct{}, h.maxConcurrentRequests)
			log.Infof("Initialized request semaphore with capacity: %d", h.maxConcurrentRequests)
		}
	})

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Apply connection limit
			if h.connectionSem != nil {
				select {
				case h.connectionSem <- struct{}{}:
					if h.maxConnections > 0 {
						newCount := atomic.AddInt32(&h.activeConnectionsCount, 1)
						h.UpdateConnectionPoolUsage(newCount, int32(h.maxConnections))
					}
					defer func() {
						<-h.connectionSem
						if h.maxConnections > 0 {
							newCount := atomic.AddInt32(&h.activeConnectionsCount, -1)
							if newCount < 0 {
								atomic.StoreInt32(&h.activeConnectionsCount, 0)
								newCount = 0
							}
							h.UpdateConnectionPoolUsage(newCount, int32(h.maxConnections))
						}
					}()
				default:
					// Connection limit exceeded
					if h.errorCounter != nil {
						method := "unknown"
						path := "unknown"
						if tr, ok := transport.FromServerContext(ctx); ok {
							method = tr.RequestHeader().Get("X-HTTP-Method")
							if method == "" {
								method = "POST"
							}
							path = tr.Operation()
						}
						h.errorCounter.WithLabelValues(method, path, "connection_limit_exceeded").Inc()
					}
					return nil, fmt.Errorf("connection limit exceeded: max %d connections", h.maxConnections)
				}
			}

			// Apply concurrent request limit
			if h.requestSem != nil {
				select {
				case h.requestSem <- struct{}{}:
					defer func() { <-h.requestSem }()
				default:
					// Request limit exceeded
					if h.errorCounter != nil {
						method := "unknown"
						path := "unknown"
						if tr, ok := transport.FromServerContext(ctx); ok {
							method = tr.RequestHeader().Get("X-HTTP-Method")
							if method == "" {
								method = "POST"
							}
							path = tr.Operation()
						}
						h.errorCounter.WithLabelValues(method, path, "request_limit_exceeded").Inc()
					}
					return nil, fmt.Errorf("concurrent request limit exceeded: max %d requests", h.maxConcurrentRequests)
				}
			}

			return handler(ctx, req)
		}
	}
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
			// Get request information
			method := "unknown"
			path := "unknown"
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.RequestHeader().Get("X-HTTP-Method")
				if method == "" {
					method = "POST" // Kratos uses POST by default
				}
				path = tr.Operation()
			}

			// Increment request queue length
			if h.requestQueueLength != nil {
				h.requestQueueLength.WithLabelValues(path).Inc()
				defer h.requestQueueLength.WithLabelValues(path).Dec()
			}

			if h.rateLimiter != nil && !h.rateLimiter.Allow() {
				// Record rate limit metrics
				if h.errorCounter != nil {
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
			route := "unknown"

			// Use Kratos transport to get request information
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.RequestHeader().Get("X-HTTP-Method")
				if method == "" {
					method = "POST" // Kratos uses POST by default
				}
				path = tr.Operation() // Operation path
				route = path          // Use operation path as route for now
			}

			// Increment active connections
			if h.activeConnections != nil {
				h.activeConnections.WithLabelValues(h.conf.Addr).Inc()
				defer h.activeConnections.WithLabelValues(h.conf.Addr).Dec()
			}

			// Increment inflight requests
			if h.inflightRequests != nil {
				h.inflightRequests.WithLabelValues(path).Inc()
				defer h.inflightRequests.WithLabelValues(path).Dec()
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
					if data, marshalErr := proto.Marshal(msg); marshalErr == nil {
						h.responseSize.WithLabelValues(method, path).Observe(float64(len(data)))
					}
				}
			}

			// Record route metrics
			if h.routeRequestDuration != nil {
				h.routeRequestDuration.WithLabelValues(route, method).Observe(duration)
			}

			if h.routeRequestCounter != nil {
				status := "success"
				if err != nil {
					status = "error"
				}
				h.routeRequestCounter.WithLabelValues(route, method, status).Inc()
			}

			return reply, err
		}
	}
}
