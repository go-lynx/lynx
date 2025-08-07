package polaris

import (
	"github.com/polarismesh/polaris-go/api"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// GetServiceInstances 获取服务实例
func (p *PlugPolaris) GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if !p.initialized {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// 记录服务发现操作指标
	if p.metrics != nil {
		p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "success")
			}
		}()
	}

	log.Infof("Getting service instances for: %s", serviceName)

	// 使用熔断器和重试机制执行操作
	var instances []model.Instance
	var lastErr error

	// 使用熔断器包装重试操作
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 创建 Consumer API 客户端
			consumerAPI := api.NewConsumerAPIByContext(p.sdk)
			if consumerAPI == nil {
				return NewInitError("failed to create consumer API")
			}

			// 构建服务发现请求
			req := &api.GetInstancesRequest{
				GetInstancesRequest: model.GetInstancesRequest{
					Service:   serviceName,
					Namespace: p.conf.Namespace,
				},
			}

			// 调用 SDK API 获取服务实例
			resp, err := consumerAPI.GetInstances(req)
			if err != nil {
				lastErr = err
				return err
			}

			instances = resp.Instances
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to get instances for service %s after retries: %v", serviceName, err)
		if p.metrics != nil {
			p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "error")
		}

		return nil, WrapServiceError(lastErr, ErrCodeServiceUnavailable, "failed to get service instances")
	}

	log.Infof("Successfully got %d instances for service %s", len(instances), serviceName)
	return instances, nil
}

// WatchService 监听服务变更
func (p *PlugPolaris) WatchService(serviceName string) (*ServiceWatcher, error) {
	if !p.initialized {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// 记录服务监听操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("watch_service", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("watch_service", "success")
			}
		}()
	}

	log.Infof("Watching service: %s", serviceName)

	// 检查是否已经在监听该服务
	p.watcherMutex.Lock()
	if existingWatcher, exists := p.activeWatchers[serviceName]; exists {
		p.watcherMutex.Unlock()
		log.Infof("Service %s is already being watched", serviceName)
		return existingWatcher, nil
	}
	p.watcherMutex.Unlock()

	// 创建 Consumer API 客户端
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		return nil, NewInitError("failed to create consumer API")
	}

	// 创建服务监听器并连接到 SDK
	watcher := NewServiceWatcher(consumerAPI, serviceName, p.conf.Namespace)
	watcher.metrics = p.metrics // 传递 metrics 引用

	// 设置事件处理回调
	watcher.SetOnInstancesChanged(func(instances []model.Instance) {
		p.handleServiceInstancesChanged(serviceName, instances)
	})

	watcher.SetOnError(func(err error) {
		p.handleServiceWatchError(serviceName, err)
	})

	// 注册监听器
	p.watcherMutex.Lock()
	p.activeWatchers[serviceName] = watcher
	p.watcherMutex.Unlock()

	// 启动监听
	watcher.Start()

	return watcher, nil
}

// checkServiceHealth 检查服务健康状态
func (p *PlugPolaris) checkServiceHealth(serviceName string, instances []model.Instance) {
	healthyCount := 0
	unhealthyCount := 0
	isolatedCount := 0

	for _, instance := range instances {
		if instance.IsIsolated() {
			isolatedCount++
		} else if instance.IsHealthy() {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	// 记录健康状态指标
	if p.metrics != nil {
		// 记录健康实例数量
		log.Infof("Service health metrics: %s - Healthy: %d, Unhealthy: %d, Isolated: %d",
			serviceName, healthyCount, unhealthyCount, isolatedCount)
	}

	// 如果健康实例太少，发出警告
	if healthyCount == 0 && len(instances) > 0 {
		log.Warnf("Service %s has no healthy instances! Total: %d, Unhealthy: %d, Isolated: %d",
			serviceName, len(instances), unhealthyCount, isolatedCount)
	} else if healthyCount < len(instances)/2 {
		log.Warnf("Service %s has low healthy instance ratio: %d/%d",
			serviceName, healthyCount, len(instances))
	}
}

// retryServiceWatch 重试服务监听
func (p *PlugPolaris) retryServiceWatch(serviceName string) {
	// 实现重试逻辑
	log.Infof("Retrying service watch for %s", serviceName)

	// 等待一段时间后重试
	time.Sleep(5 * time.Second)

	// 重新创建监听器
	if _, err := p.WatchService(serviceName); err == nil {
		log.Infof("Successfully recreated service watcher for %s", serviceName)
	} else {
		log.Errorf("Failed to recreate service watcher for %s: %v", serviceName, err)
	}
}

// useCachedServiceInstances 使用缓存的服务实例
func (p *PlugPolaris) useCachedServiceInstances(serviceName string) {
	log.Infof("Using cached service instances for %s", serviceName)
	// 这里可以实现从缓存获取服务实例的逻辑
}

// switchToBackupDiscovery 切换到备用服务发现
func (p *PlugPolaris) switchToBackupDiscovery(serviceName string) {
	log.Infof("Switching to backup discovery for %s", serviceName)
	// 这里可以实现切换到备用服务发现的逻辑
}

// notifyDegradationMode 通知降级模式
func (p *PlugPolaris) notifyDegradationMode(serviceName string, info map[string]interface{}) {
	log.Infof("Notifying degradation mode for %s: %+v", serviceName, info)
	// 这里可以实现通知降级模式的逻辑
}

// getServiceDiscoveryStats 获取服务发现统计信息
func (p *PlugPolaris) getServiceDiscoveryStats() map[string]interface{} {
	stats := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"watchers": map[string]interface{}{
			"active_count": len(p.activeWatchers),
			"config_count": len(p.configWatchers),
		},
		"cache": p.getCacheStats(),
	}

	return stats
}

// validateServiceInstance 验证服务实例
func (p *PlugPolaris) validateServiceInstance(instance model.Instance) bool {
	// 验证服务实例的有效性
	if instance == nil {
		return false
	}

	// 检查必要字段
	if instance.GetId() == "" {
		return false
	}

	if instance.GetHost() == "" {
		return false
	}

	if instance.GetPort() <= 0 {
		return false
	}

	return true
}

// filterHealthyInstances 过滤健康实例
func (p *PlugPolaris) filterHealthyInstances(instances []model.Instance) []model.Instance {
	healthyInstances := make([]model.Instance, 0, len(instances))

	for _, instance := range instances {
		if p.validateServiceInstance(instance) && instance.IsHealthy() && !instance.IsIsolated() {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	return healthyInstances
}

// getServiceInstanceCount 获取服务实例数量统计
func (p *PlugPolaris) getServiceInstanceCount(instances []model.Instance) map[string]int {
	counts := map[string]int{
		"total":     len(instances),
		"healthy":   0,
		"unhealthy": 0,
		"isolated":  0,
		"valid":     0,
		"invalid":   0,
	}

	for _, instance := range instances {
		if p.validateServiceInstance(instance) {
			counts["valid"]++
		} else {
			counts["invalid"]++
			continue
		}

		if instance.IsIsolated() {
			counts["isolated"]++
		} else if instance.IsHealthy() {
			counts["healthy"]++
		} else {
			counts["unhealthy"]++
		}
	}

	return counts
}
