package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// MiddlewareAdapter 
// Responsibility: provide HTTP/gRPC rate limit middleware and router middleware.

// HTTPRateLimit creates HTTP rate limit middleware.
// It fetches HTTP rate limit policies from Polaris and applies them to the HTTP request flow.
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

// GRPCRateLimit creates gRPC rate limit middleware.
// It fetches gRPC rate limit policies from Polaris and applies them to the gRPC request flow.
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

// CheckRateLimit checks rate limiting for a service with optional labels.
func (p *PlugPolaris) CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if err := p.checkInitialized(); err != nil {
		return false, err
	}

	// Record metrics for the rate limit check operation
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("check_rate_limit", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("check_rate_limit", "success")
			}
		}()
	}

	log.Infof("Checking rate limit for service: %s", serviceName)

	// Create Limit API client
	limitAPI := api.NewLimitAPIByContext(p.sdk)
	if limitAPI == nil {
		return false, NewInitError("failed to create limit API")
	}

	// Build quota request
	quotaReq := api.NewQuotaRequest()
	quotaReq.SetService(serviceName)
	quotaReq.SetNamespace(p.conf.Namespace)

	// Set labels
	for key, value := range labels {
		quotaReq.AddArgument(model.BuildQueryArgument(key, value))
	}

	// Execute with circuit breaker and retry mechanism
	var future api.QuotaFuture
	var lastErr error

	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// Call SDK API to check rate limit
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

	// Obtain rate limit result
	result := future.Get()
	if result == nil {
		log.Errorf("Rate limit result is nil for service %s", serviceName)
		return false, NewServiceError(ErrCodeRateLimitFailed, "rate limit result is nil")
	}

	// Check whether the request is allowed
	if result.Code == model.QuotaResultOk {
		log.Infof("Rate limit check passed for service %s", serviceName)
		return true, nil
	} else {
		log.Warnf("Rate limit exceeded for service %s", serviceName)
		return false, nil
	}
}
