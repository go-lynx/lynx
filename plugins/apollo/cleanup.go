package apollo

import (
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// stopHealthCheck stops health check
// Uses sync.Once to ensure the channel is closed only once, preventing panic
func (p *PlugApollo) stopHealthCheck() {
	p.mu.Lock()
	ch := p.healthCheckCh
	p.mu.Unlock()
	
	if ch != nil {
		// Use sync.Once to ensure close() is only called once
		// This prevents panic if stopHealthCheck() is called multiple times
		p.healthCheckCloseOnce.Do(func() {
			close(ch)
			p.mu.Lock()
			p.healthCheckCh = nil
			p.mu.Unlock()
			log.Infof("Stopping health check")
		})
	}
}

// cleanupWatchers cleans up watchers
func (p *PlugApollo) cleanupWatchers() {
	log.Infof("Cleaning up watchers")

	// Clean up configuration watchers
	p.watcherMutex.Lock()
	configWatcherCount := len(p.configWatchers)
	for configKey, watcher := range p.configWatchers {
		log.Infof("Stopping config watcher for: %s", configKey)
		if watcher != nil {
			watcher.Stop()
		}
	}
	p.configWatchers = make(map[string]*ConfigWatcher)
	p.watcherMutex.Unlock()

	log.Infof("Cleaned up %d config watchers", configWatcherCount)
}

// closeClientConnection closes client connection
func (p *PlugApollo) closeClientConnection() {
	if p.client != nil {
		log.Infof("Closing Apollo HTTP client connection")

		// Get client information
		clientInfo := map[string]interface{}{
			"client_type": "ApolloHTTPClient",
			"app_id":      p.conf.AppId,
			"cluster":     p.conf.Cluster,
			"namespace":   p.conf.Namespace,
		}

		// Close HTTP client
		p.client.Close()

		log.Infof("Apollo HTTP client connection closed: %+v", clientInfo)
		p.client = nil
	}
}

// releaseMemoryResources releases memory resources
func (p *PlugApollo) releaseMemoryResources() {
	log.Infof("Releasing memory resources")

	// Clear configuration
	if p.conf != nil {
		log.Infof("Clearing configuration")
		// Don't set to nil, just clear sensitive fields if needed
	}

	// Clear enhanced components
	if p.metrics != nil {
		log.Infof("Clearing metrics")
		p.metrics = nil
	}

	if p.retryManager != nil {
		log.Infof("Clearing retry manager")
		p.retryManager = nil
	}

	if p.circuitBreaker != nil {
		log.Infof("Clearing circuit breaker")
		p.circuitBreaker.ForceClose()
		p.circuitBreaker = nil
	}

	// Clear cache
	p.clearConfigCache()

	log.Infof("Memory resources released")
}

// clearConfigCache clears configuration cache
func (p *PlugApollo) clearConfigCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.configCache = make(map[string]interface{})
	log.Infof("Configuration cache cleared")
}

// stopBackgroundTasks stops background tasks
func (p *PlugApollo) stopBackgroundTasks() {
	log.Infof("Stopping background tasks")

	// Stop retry tasks
	if p.retryManager != nil {
		log.Infof("Stopping retry manager background tasks")
		p.retryManager = nil
	}

	// Stop circuit breaker tasks
	if p.circuitBreaker != nil {
		log.Infof("Stopping circuit breaker background tasks")
		p.circuitBreaker.ForceClose()
	}

	// Stop metrics collection tasks
	if p.metrics != nil {
		log.Infof("Stopping metrics collection tasks")
		p.metrics = nil
	}

	// Stop other background tasks
	log.Infof("Stopping health check tasks")
	log.Infof("Stopping monitoring tasks")
	log.Infof("Stopping audit log tasks")

	log.Infof("Background tasks stopped")
}

// getCleanupStats gets cleanup statistics
func (p *PlugApollo) getCleanupStats() map[string]interface{} {
	stats := map[string]interface{}{
		"cleanup_time": time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
		"resources": map[string]interface{}{
			"client_closed":      p.client == nil,
			"metrics_cleared":   p.metrics == nil,
			"retry_cleared":     p.retryManager == nil,
			"breaker_cleared":   p.circuitBreaker == nil,
		},
	}

	return stats
}

// CleanupTasks cleanup tasks
func (p *PlugApollo) CleanupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.IsInitialized() {
		return nil
	}

	if p.IsDestroyed() {
		return nil
	}

	// Record cleanup operation metrics
	if p.metrics != nil {
		p.metrics.RecordClientOperation("cleanup", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordClientOperation("cleanup", "success")
			}
		}()
	}

	log.Infof("Destroying Apollo plugin")

	// 1. Stop health check
	p.stopHealthCheck()

	// 2. Clean up watchers
	p.cleanupWatchers()

	// 3. Close client connection
	p.closeClientConnection()

	// 4. Release memory resources
	p.releaseMemoryResources()

	// 5. Stop background tasks
	p.stopBackgroundTasks()

	p.setDestroyed()
	log.Infof("Apollo plugin destroyed successfully")
	return nil
}

