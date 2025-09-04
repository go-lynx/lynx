package polaris

import (
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// updateServiceInstanceCache updates service instance cache
func (p *PlugPolaris) updateServiceInstanceCache(serviceName string, instances []model.Instance) {
	// Implement local cache update logic
	cacheKey := fmt.Sprintf("service:%s:%s", p.conf.Namespace, serviceName)

	// Build cache data
	cacheData := map[string]interface{}{
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"instances":    instances,
		"updated_at":   time.Now().Unix(),
		"count":        len(instances),
	}

	// Use lock to protect cache operations
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// 1. Check if cache exists
	if p.serviceCache == nil {
		p.serviceCache = make(map[string]interface{})
	}

	// 2. Update cache data
	p.serviceCache[cacheKey] = cacheData

	// 3. Set cache expiration time (optional)
	// TTL mechanism can be implemented here

	// 4. Record cache statistics
	cacheStats := map[string]interface{}{
		"cache_key":      cacheKey,
		"cache_size":     len(p.serviceCache),
		"instance_count": len(instances),
		"update_time":    time.Now().Unix(),
	}

	log.Infof("Updated service instance cache for %s: %d instances", serviceName, len(instances))
	log.Debugf("Cache stats: %+v", cacheStats)
}

// updateConfigCache updates configuration cache
func (p *PlugPolaris) updateConfigCache(fileName, group string, config model.ConfigFile) {
	// Implement configuration cache update logic
	cacheKey := fmt.Sprintf("config:%s:%s:%s", p.conf.Namespace, group, fileName)

	// Build cache data
	cacheData := map[string]interface{}{
		"config_file":    fileName,
		"group":          group,
		"namespace":      p.conf.Namespace,
		"content":        config.GetContent(),
		"updated_at":     time.Now().Unix(),
		"content_length": len(config.GetContent()),
	}

	// Specific implementation: use in-memory cache
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// 1. Check if cache exists
	if p.configCache == nil {
		p.configCache = make(map[string]interface{})
	}

	// 2. Update cache data
	p.configCache[cacheKey] = cacheData

	// 3. Set cache expiration time (optional)
	// TTL mechanism can be implemented here

	// 4. Record cache statistics
	cacheStats := map[string]interface{}{
		"cache_key":      cacheKey,
		"cache_size":     len(p.configCache),
		"content_length": len(config.GetContent()),
		"update_time":    time.Now().Unix(),
	}

	log.Infof("Updated config cache for %s:%s, content length: %d", fileName, group, len(config.GetContent()))
	log.Debugf("Config cache stats: %+v", cacheStats)
}

// getServiceInstanceFromCache retrieves service instances from cache
func (p *PlugPolaris) getServiceInstanceFromCache(serviceName string) ([]model.Instance, bool) {
	cacheKey := fmt.Sprintf("service:%s:%s", p.conf.Namespace, serviceName)

	// Use read lock to protect cache reading
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	if p.serviceCache == nil {
		return nil, false
	}

	if cacheData, exists := p.serviceCache[cacheKey]; exists {
		if data, ok := cacheData.(map[string]interface{}); ok {
			if instances, ok := data["instances"].([]model.Instance); ok {
				log.Infof("Found %d cached instances for service %s", len(instances), serviceName)
				return instances, true
			}
		}
	}

	return nil, false
}

// getConfigFromCache retrieves configuration from cache
func (p *PlugPolaris) getConfigFromCache(fileName, group string) (string, bool) {
	cacheKey := fmt.Sprintf("config:%s:%s:%s", p.conf.Namespace, group, fileName)

	// Use read lock to protect cache reading
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	if p.configCache == nil {
		return "", false
	}

	if cacheData, exists := p.configCache[cacheKey]; exists {
		if data, ok := cacheData.(map[string]interface{}); ok {
			if content, ok := data["content"].(string); ok {
				log.Infof("Found cached config for %s:%s", fileName, group)
				return content, true
			}
		}
	}

	return "", false
}

// clearServiceCache clears service cache
func (p *PlugPolaris) clearServiceCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	if p.serviceCache != nil {
		clearedCount := len(p.serviceCache)
		p.serviceCache = make(map[string]interface{})
		log.Infof("Cleared %d service cache entries", clearedCount)
	}
}

// clearConfigCache clears configuration cache
func (p *PlugPolaris) clearConfigCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	if p.configCache != nil {
		clearedCount := len(p.configCache)
		p.configCache = make(map[string]interface{})
		log.Infof("Cleared %d config cache entries", clearedCount)
	}
}

// getCacheStats retrieves cache statistics
func (p *PlugPolaris) getCacheStats() map[string]interface{} {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	stats := map[string]interface{}{
		"service_cache_size": 0,
		"config_cache_size":  0,
		"timestamp":          time.Now().Unix(),
	}

	if p.serviceCache != nil {
		stats["service_cache_size"] = len(p.serviceCache)
	}

	if p.configCache != nil {
		stats["config_cache_size"] = len(p.configCache)
	}

	return stats
}
