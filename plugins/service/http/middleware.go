// Package http 实现了 Lynx 框架的 HTTP 服务器插件功能。
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

// buildMiddlewares 构建中间件链
func (h *ServiceHttp) buildMiddlewares() []middleware.Middleware {
	var middlewares []middleware.Middleware

	// 基础中间件
	middlewares = append(middlewares,
		// 配置链路追踪中间件，设置追踪器名称为应用名称
		tracing.Server(tracing.WithTracerName(app.GetName())),
		// 配置日志中间件，使用 Lynx 框架的日志记录器
		logging.Server(log.Logger),
		// 配置增强的响应包装中间件（集成监控指标）
		TracerLogPackWithMetrics(h),
		// 配置参数验证中间件
		validate.ProtoValidate(),
		// 配置恢复中间件，处理请求处理过程中的 panic
		h.recoveryMiddleware(),
	)

	// 安全中间件
	middlewares = append(middlewares, h.rateLimitMiddleware())

	// 配置限流中间件，使用 Lynx 框架控制平面的 HTTP 限流策略
	// 如果有限流中间件，则追加进去
	if rl := app.Lynx().GetControlPlane().HTTPRateLimit(); rl != nil {
		middlewares = append(middlewares, rl)
	}

	return middlewares
}

// recoveryMiddleware 恢复中间件
func (h *ServiceHttp) recoveryMiddleware() middleware.Middleware {
	return recovery.Recovery(
		recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
			log.ErrorCtx(ctx, "Panic recovered", "error", err)

			// 记录错误指标
			if h.errorCounter != nil {
				method := "POST"
				path := "unknown"
				if tr, ok := transport.FromServerContext(ctx); ok {
					// Kratos HTTP 传输常用 POST
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

// rateLimitMiddleware 限流中间件
func (h *ServiceHttp) rateLimitMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if h.rateLimiter != nil && !h.rateLimiter.Allow() {
				// 记录限流指标
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

// metricsMiddleware 监控中间件
func (h *ServiceHttp) metricsMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			start := time.Now()

			// 获取请求信息
			method := "unknown"
			path := "unknown"

			// 使用 Kratos transport 包获取请求信息
			if tr, ok := transport.FromServerContext(ctx); ok {
				method = tr.RequestHeader().Get("X-HTTP-Method")
				if method == "" {
					method = "POST" // Kratos 默认使用 POST
				}
				path = tr.Operation() // 获取操作路径
			}

			// 处理请求
			reply, err = handler(ctx, req)

			// 记录指标
			duration := time.Since(start).Seconds()
			if h.requestDuration != nil {
				h.requestDuration.WithLabelValues(method, path).Observe(duration)
			}

			// 记录请求计数
			if h.requestCounter != nil {
				status := "success"
				if err != nil {
					status = "error"
				}
				h.requestCounter.WithLabelValues(method, path, status).Inc()
			}

			// 记录响应大小
			if h.responseSize != nil && reply != nil {
				// 尝试获取响应大小
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
