package polaris

import (
	"time"

	"github.com/polarismesh/polaris-go/api"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// GetServiceInstances gets service instances
func (p *PlugPolaris) GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	// Record service discovery operation metrics
	if p.metrics != nil {
		p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "success")
			}
		}()
	}

	log.Infof("Getting service instances for: %s", serviceName)

	// Execute operation with circuit breaker and retry mechanism
	var instances []model.Instance
	var lastErr error

	// Wrap retry operation with circuit breaker
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// Create Consumer API client
			consumerAPI := api.NewConsumerAPIByContext(p.sdk)
			if consumerAPI == nil {
				return NewInitError("failed to create consumer API")
			}

			// Build service discovery request
			req := &api.GetInstancesRequest{
				GetInstancesRequest: model.GetInstancesRequest{
					Service:   serviceName,
					Namespace: p.conf.Namespace,
				},
			}

			// Call SDK API to get service instances
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

// WatchService watches service changes - uses double-checked locking pattern to improve concurrency safety
func (p *PlugPolaris) WatchService(serviceName string) (*ServiceWatcher, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	// Record service watch operation metrics
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("watch_service", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("watch_service", "success")
			}
		}()
	}

	log.Infof("Watching service: %s", serviceName)

	// First check (read lock)
	p.watcherMutex.RLock()
	if existingWatcher, exists := p.activeWatchers[serviceName]; exists {
		p.watcherMutex.RUnlock()
		log.Infof("Service %s is already being watched", serviceName)
		return existingWatcher, nil
	}
	p.watcherMutex.RUnlock()

	// Create Consumer API client
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		return nil, NewInitError("failed to create consumer API")
	}

	// Create service watcher and connect to SDK
	watcher := NewServiceWatcher(consumerAPI, serviceName, p.conf.Namespace)

	// Second check (write lock) - double-checked locking pattern
	p.watcherMutex.Lock()
	defer p.watcherMutex.Unlock()

	// Check again if another goroutine has already created the watcher
	if existingWatcher, exists := p.activeWatchers[serviceName]; exists {
		log.Infof("Service %s watcher was created by another goroutine", serviceName)
		return existingWatcher, nil
	}

	// Register watcher
	p.activeWatchers[serviceName] = watcher

	// Set callback functions
	watcher.SetOnInstancesChanged(func(instances []model.Instance) {
		p.handleServiceInstancesChanged(serviceName, instances)
	})

	watcher.SetOnError(func(err error) {
		p.handleServiceWatchError(serviceName, err)
	})

	// Start watching
	watcher.Start()

	log.Infof("Started watching service: %s", serviceName)
	return watcher, nil
}

// checkServiceHealth checks service health status
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

	// Record health status metrics
	if p.metrics != nil {
		// Record healthy instance count
		log.Infof("Service health metrics: %s - Healthy: %d, Unhealthy: %d, Isolated: %d",
			serviceName, healthyCount, unhealthyCount, isolatedCount)
	}

	// Issue warning if too few healthy instances
	if healthyCount == 0 && len(instances) > 0 {
		log.Warnf("Service %s has no healthy instances! Total: %d, Unhealthy: %d, Isolated: %d",
			serviceName, len(instances), unhealthyCount, isolatedCount)
	} else if healthyCount < len(instances)/2 {
		log.Warnf("Service %s has low healthy instance ratio: %d/%d",
			serviceName, healthyCount, len(instances))
	}
}

// retryServiceWatch retries service watch
func (p *PlugPolaris) retryServiceWatch(serviceName string) {
    // Implement retry logic
    log.Infof("Retrying service watch for %s", serviceName)

    // Wait for a while before retrying, but allow cancellation on plugin stop
    if p.healthCheckCh != nil {
        select {
        case <-p.healthCheckCh:
            log.Infof("Service watch retry canceled due to plugin shutdown: %s", serviceName)
            return
        case <-time.After(5 * time.Second):
        }
    } else {
        if p.IsDestroyed() {
            return
        }
        time.Sleep(5 * time.Second)
    }

    if p.IsDestroyed() {
        return
    }

	// Recreate watcher
	    if _, err := p.WatchService(serviceName); err == nil {
        log.Infof("Successfully recreated service watcher for %s", serviceName)
    } else {
        log.Errorf("Failed to recreate service watcher for %s: %v", serviceName, err)
    }
}

// useCachedServiceInstances uses cached service instances
func (p *PlugPolaris) useCachedServiceInstances(serviceName string) {
	log.Infof("Using cached service instances for %s", serviceName)
	// Here you can implement logic to get service instances from cache
}

// switchToBackupDiscovery switches to backup service discovery
func (p *PlugPolaris) switchToBackupDiscovery(serviceName string) {
	log.Infof("Switching to backup discovery for %s", serviceName)
	// Here you can implement logic to switch to backup service discovery
}

// notifyDegradationMode notifies degradation mode
func (p *PlugPolaris) notifyDegradationMode(serviceName string, info map[string]interface{}) {
	log.Infof("Notifying degradation mode for %s: %+v", serviceName, info)
	// Here you can implement logic to notify degradation mode
}

// getServiceDiscoveryStats gets service discovery statistics
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

// validateServiceInstance validates service instance
func (p *PlugPolaris) validateServiceInstance(instance model.Instance) bool {
	// Validate service instance validity
	if instance == nil {
		return false
	}

	// Check required fields
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

// filterHealthyInstances filters healthy instances
func (p *PlugPolaris) filterHealthyInstances(instances []model.Instance) []model.Instance {
	healthyInstances := make([]model.Instance, 0, len(instances))

	for _, instance := range instances {
		if p.validateServiceInstance(instance) && instance.IsHealthy() && !instance.IsIsolated() {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	return healthyInstances
}

// getServiceInstanceCount gets service instance count statistics
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
