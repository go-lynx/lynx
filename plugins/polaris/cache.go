package polaris

import (
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// updateServiceInstanceCache 更新服务实例缓存
func (p *PlugPolaris) updateServiceInstanceCache(serviceName string, instances []model.Instance) {
	// 实现本地缓存更新逻辑
	cacheKey := fmt.Sprintf("service:%s:%s", p.conf.Namespace, serviceName)

	// 构建缓存数据
	cacheData := map[string]interface{}{
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"instances":    instances,
		"updated_at":   time.Now().Unix(),
		"count":        len(instances),
	}

	// 使用锁保护缓存操作
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// 1. 检查缓存是否存在
	if p.serviceCache == nil {
		p.serviceCache = make(map[string]interface{})
	}

	// 2. 更新缓存数据
	p.serviceCache[cacheKey] = cacheData

	// 3. 设置缓存过期时间（可选）
	// 这里可以实现 TTL 机制

	// 4. 记录缓存统计信息
	cacheStats := map[string]interface{}{
		"cache_key":      cacheKey,
		"cache_size":     len(p.serviceCache),
		"instance_count": len(instances),
		"update_time":    time.Now().Unix(),
	}

	log.Infof("Updated service instance cache for %s: %d instances", serviceName, len(instances))
	log.Debugf("Cache stats: %+v", cacheStats)
}

// updateConfigCache 更新配置缓存
func (p *PlugPolaris) updateConfigCache(fileName, group string, config model.ConfigFile) {
	// 实现配置缓存更新逻辑
	cacheKey := fmt.Sprintf("config:%s:%s:%s", p.conf.Namespace, group, fileName)

	// 构建缓存数据
	cacheData := map[string]interface{}{
		"config_file":    fileName,
		"group":          group,
		"namespace":      p.conf.Namespace,
		"content":        config.GetContent(),
		"updated_at":     time.Now().Unix(),
		"content_length": len(config.GetContent()),
	}

	// 具体实现：使用内存缓存
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// 1. 检查缓存是否存在
	if p.configCache == nil {
		p.configCache = make(map[string]interface{})
	}

	// 2. 更新缓存数据
	p.configCache[cacheKey] = cacheData

	// 3. 设置缓存过期时间（可选）
	// 这里可以实现 TTL 机制

	// 4. 记录缓存统计信息
	cacheStats := map[string]interface{}{
		"cache_key":      cacheKey,
		"cache_size":     len(p.configCache),
		"content_length": len(config.GetContent()),
		"update_time":    time.Now().Unix(),
	}

	log.Infof("Updated config cache for %s:%s, content length: %d", fileName, group, len(config.GetContent()))
	log.Debugf("Config cache stats: %+v", cacheStats)
}

// getServiceInstanceFromCache 从缓存获取服务实例
func (p *PlugPolaris) getServiceInstanceFromCache(serviceName string) ([]model.Instance, bool) {
	cacheKey := fmt.Sprintf("service:%s:%s", p.conf.Namespace, serviceName)

	// 使用读锁保护缓存读取
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

// getConfigFromCache 从缓存获取配置
func (p *PlugPolaris) getConfigFromCache(fileName, group string) (string, bool) {
	cacheKey := fmt.Sprintf("config:%s:%s:%s", p.conf.Namespace, group, fileName)

	// 使用读锁保护缓存读取
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

// clearServiceCache 清理服务缓存
func (p *PlugPolaris) clearServiceCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	if p.serviceCache != nil {
		clearedCount := len(p.serviceCache)
		p.serviceCache = make(map[string]interface{})
		log.Infof("Cleared %d service cache entries", clearedCount)
	}
}

// clearConfigCache 清理配置缓存
func (p *PlugPolaris) clearConfigCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	if p.configCache != nil {
		clearedCount := len(p.configCache)
		p.configCache = make(map[string]interface{})
		log.Infof("Cleared %d config cache entries", clearedCount)
	}
}

// getCacheStats 获取缓存统计信息
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
