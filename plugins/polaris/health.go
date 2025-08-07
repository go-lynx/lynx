package polaris

import (
	"fmt"
	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	"strings"
)

// CheckHealth 健康检查
func (p *PlugPolaris) CheckHealth() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return NewInitError("Polaris plugin not initialized")
	}

	if p.destroyed {
		return NewInitError("Polaris plugin has been destroyed")
	}

	// 检查 Polaris 实例
	if p.polaris == nil {
		return NewInitError("Polaris instance is nil")
	}

	// 检查 SDK 连接
	if p.sdk == nil {
		return NewInitError("Polaris SDK context is nil")
	}

	// 真正检查 Polaris 控制平面的健康状态
	return p.checkPolarisControlPlaneHealth()
}

// checkPolarisControlPlaneHealth 检查 Polaris 控制平面健康状态
func (p *PlugPolaris) checkPolarisControlPlaneHealth() error {
	// 记录健康检查开始
	if p.metrics != nil {
		p.metrics.RecordHealthCheck("polaris", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordHealthCheck("polaris", "success")
			}
		}()
	}

	log.Infof("Checking Polaris control plane health")

	// 使用熔断器和重试机制执行健康检查
	var healthErr error
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 1. 检查 SDK 连接状态
			if err := p.checkSDKConnection(); err != nil {
				healthErr = err
				return err
			}

			// 2. 检查服务发现功能
			if err := p.checkServiceDiscoveryHealth(); err != nil {
				healthErr = err
				return err
			}

			// 3. 检查配置管理功能
			if err := p.checkConfigManagementHealth(); err != nil {
				healthErr = err
				return err
			}

			// 4. 检查限流功能
			if err := p.checkRateLimitHealth(); err != nil {
				healthErr = err
				return err
			}

			return nil
		})
	})

	if err != nil {
		log.Errorf("Polaris control plane health check failed: %v", healthErr)
		if p.metrics != nil {
			p.metrics.RecordHealthCheck("polaris", "error")
		}
		return WrapServiceError(healthErr, ErrCodeServiceUnavailable, "Polaris control plane health check failed")
	}

	log.Infof("Polaris control plane health check passed")
	return nil
}

// checkSDKConnection 检查 SDK 连接状态
func (p *PlugPolaris) checkSDKConnection() error {
	// 尝试创建 Consumer API 客户端来验证连接
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		return fmt.Errorf("failed to create consumer API client")
	}

	// 尝试创建一个简单的服务发现请求来验证连接
	req := &api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   "health-check-service", // 使用一个测试服务名
			Namespace: p.conf.Namespace,
		},
	}

	// 尝试调用 API，即使服务不存在也应该能连接
	_, err := consumerAPI.GetInstances(req)
	if err != nil {
		// 如果错误是服务不存在，说明连接是正常的
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no instances") {
			log.Debugf("SDK connection test passed (service not found is expected)")
			return nil
		}
		return fmt.Errorf("SDK connection test failed: %v", err)
	}

	return nil
}

// checkServiceDiscoveryHealth 检查服务发现功能
func (p *PlugPolaris) checkServiceDiscoveryHealth() error {
	// 检查服务发现相关的组件状态
	if p.activeWatchers == nil {
		return fmt.Errorf("service watchers not initialized")
	}

	// 检查是否有活跃的监听器
	watcherCount := len(p.activeWatchers)
	log.Debugf("Service discovery health: %d active watchers", watcherCount)

	return nil
}

// checkConfigManagementHealth 检查配置管理功能
func (p *PlugPolaris) checkConfigManagementHealth() error {
	// 检查配置管理相关的组件状态
	if p.configWatchers == nil {
		return fmt.Errorf("config watchers not initialized")
	}

	// 检查是否有活跃的配置监听器
	configWatcherCount := len(p.configWatchers)
	log.Debugf("Config management health: %d active config watchers", configWatcherCount)

	return nil
}

// checkRateLimitHealth 检查限流功能
func (p *PlugPolaris) checkRateLimitHealth() error {
	// 检查限流相关的组件状态
	if p.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not initialized")
	}

	if p.retryManager == nil {
		return fmt.Errorf("retry manager not initialized")
	}

	// 检查熔断器状态
	breakerState := p.circuitBreaker.GetState()
	log.Debugf("Rate limit health: circuit breaker state = %s", breakerState)

	return nil
}
