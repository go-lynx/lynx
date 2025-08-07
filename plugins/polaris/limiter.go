package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// MiddlewareAdapter 中间件适配器
// 职责：提供 HTTP/gRPC 限流中间件和路由中间件

// HTTPRateLimit 创建 HTTP 限流中间件
// 从 Polaris 获取 HTTP 限流策略，并应用到 HTTP 请求处理流程中
func (p *PlugPolaris) HTTPRateLimit() middleware.Middleware {
	if err := p.checkInitialized(); err != nil {
		log.Warnf("Polaris plugin not initialized, returning nil HTTP rate limit middleware: %v", err)
		return nil
	}

	log.Infof("Synchronizing [HTTP] rate limit policy")

	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// GRPCRateLimit 创建 gRPC 限流中间件
// 从 Polaris 获取 gRPC 限流策略，并应用到 gRPC 请求处理流程中
func (p *PlugPolaris) GRPCRateLimit() middleware.Middleware {
	if err := p.checkInitialized(); err != nil {
		log.Warnf("Polaris plugin not initialized, returning nil gRPC rate limit middleware: %v", err)
		return nil
	}

	log.Infof("Synchronizing [GRPC] rate limit policy")

	return polaris.Ratelimit(GetPolaris().Limiter(
		polaris.WithLimiterService(app.GetName()),
		polaris.WithLimiterNamespace(GetPlugin().conf.Namespace),
	))
}

// CheckRateLimit 检查限流
func (p *PlugPolaris) CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if err := p.checkInitialized(); err != nil {
		return false, err
	}

	// 记录限流检查操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("check_rate_limit", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("check_rate_limit", "success")
			}
		}()
	}

	log.Infof("Checking rate limit for service: %s", serviceName)

	// 创建 Limit API 客户端
	limitAPI := api.NewLimitAPIByContext(p.sdk)
	if limitAPI == nil {
		return false, NewInitError("failed to create limit API")
	}

	// 构建限流请求
	quotaReq := api.NewQuotaRequest()
	quotaReq.SetService(serviceName)
	quotaReq.SetNamespace(p.conf.Namespace)

	// 设置标签
	for key, value := range labels {
		quotaReq.AddArgument(model.BuildQueryArgument(key, value))
	}

	// 使用熔断器和重试机制执行操作
	var future api.QuotaFuture
	var lastErr error

	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 调用 SDK API 检查限流
			fut, err := limitAPI.GetQuota(quotaReq)
			if err != nil {
				lastErr = err
				return err
			}
			future = fut
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to check rate limit for service %s after retries: %v", serviceName, err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("check_rate_limit", "error")
		}
		return false, WrapServiceError(lastErr, ErrCodeRateLimitFailed, "failed to check rate limit")
	}

	// 获取限流结果
	result := future.Get()
	if result == nil {
		log.Errorf("Rate limit result is nil for service %s", serviceName)
		return false, NewServiceError(ErrCodeRateLimitFailed, "rate limit result is nil")
	}

	// 检查是否被限流
	if result.Code == model.QuotaResultOk {
		log.Infof("Rate limit check passed for service %s", serviceName)
		return true, nil
	} else {
		log.Warnf("Rate limit exceeded for service %s", serviceName)
		return false, nil
	}
}
