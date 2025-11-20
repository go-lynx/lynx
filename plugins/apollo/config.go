package apollo

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/apollo/conf"
)

// ConfigAdapter configuration adapter
// Responsibility: provide Apollo configuration center related functionality

// GetConfig gets configuration from Apollo configuration center
// This method retrieves the corresponding configuration source from Apollo configuration center based on the provided namespace
func (p *PlugApollo) GetConfig(fileName string, group string) (config.Source, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	// For Apollo, namespace is used instead of fileName/group
	// fileName is treated as namespace, group is ignored (Apollo uses app_id + cluster + namespace)
	namespace := fileName
	if namespace == "" {
		namespace = p.conf.Namespace
	}

	log.Infof("Getting config from Apollo - Namespace: [%s]", namespace)

	// Create Apollo config source
	source := NewApolloConfigSource(p.client, p.conf.AppId, p.conf.Cluster, namespace)

	return source, nil
}

// GetConfigSources gets all configuration sources for multi-config loading
// This method implements the MultiConfigControlPlane interface
func (p *PlugApollo) GetConfigSources() ([]config.Source, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	var sources []config.Source

	// Get main configuration source
	mainSource, err := p.getMainConfigSource()
	if err != nil {
		return nil, fmt.Errorf("failed to get main config source: %w", err)
	}
	if mainSource != nil {
		sources = append(sources, mainSource)
	}

	// Get additional configuration sources
	additionalSources, err := p.getAdditionalConfigSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get additional config sources: %w", err)
	}
	sources = append(sources, additionalSources...)

	return sources, nil
}

// getMainConfigSource gets the main configuration source based on service_config
func (p *PlugApollo) getMainConfigSource() (config.Source, error) {
	if p.conf.ServiceConfig == nil {
		// Fallback to default behavior if service_config is not configured
		namespace := p.conf.Namespace
		if namespace == "" {
			namespace = conf.DefaultNamespace
		}

		log.Infof("Loading main configuration - Namespace: [%s]", namespace)

		return p.GetConfig(namespace, "")
	}

	// Use service_config configuration
	serviceConfig := p.conf.ServiceConfig

	// Determine namespace
	namespace := serviceConfig.Namespace
	if namespace == "" {
		namespace = p.conf.Namespace
	}

	log.Infof("Loading main configuration - Namespace: [%s]", namespace)

	return p.GetConfig(namespace, "")
}

// getAdditionalConfigSources gets additional configuration sources
func (p *PlugApollo) getAdditionalConfigSources() ([]config.Source, error) {
	if p.conf.ServiceConfig == nil || len(p.conf.ServiceConfig.AdditionalNamespaces) == 0 {
		return nil, nil
	}

	serviceConfig := p.conf.ServiceConfig

	var sources []config.Source
	for _, namespace := range serviceConfig.AdditionalNamespaces {
		// Use service_config namespace as default if not specified
		if namespace == "" {
			namespace = serviceConfig.Namespace
		}
		if namespace == "" {
			namespace = p.conf.Namespace
		}

		log.Infof("Loading additional configuration - Namespace: [%s] Priority: [%d] Strategy: [%s]",
			namespace, serviceConfig.Priority, serviceConfig.MergeStrategy)

		source, err := p.GetConfig(namespace, "")
		if err != nil {
			log.Errorf("Failed to load additional configuration - Namespace: [%s] Error: %v",
				namespace, err)
			return nil, fmt.Errorf("failed to load additional config %s: %w", namespace, err)
		}

		sources = append(sources, source)
	}

	// Sort by priority if needed (for now, just append in order)
	// Higher priority configs should be loaded later to override earlier ones
	// Note: Priority sorting is handled by the framework's config merge logic
	sort.Slice(sources, func(i, j int) bool {
		// Return false to maintain insertion order
		// The framework will handle priority-based merging
		return false
	})

	return sources, nil
}

// GetConfigValue gets configuration value from Apollo
func (p *PlugApollo) GetConfigValue(namespace, key string) (string, error) {
	if err := p.checkInitialized(); err != nil {
		return "", err
	}

	// Record configuration operation metrics
	if p.metrics != nil {
		p.metrics.RecordConfigOperation(namespace, "get", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordConfigOperation(namespace, "get", "success")
			}
		}()
	}

	log.Infof("Getting config value - Namespace: [%s], Key: [%s]", namespace, key)

	// Execute with circuit breaker and retry mechanism
	var value string
	var lastErr error

	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// Call Apollo client to get configuration
			// This is a placeholder - actual implementation depends on Apollo SDK
			val, err := p.getConfigValueFromApollo(namespace, key)
			if err != nil {
				lastErr = err
				return err
			}
			value = val
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to get config value %s:%s after retries: %v", namespace, key, err)
		if p.metrics != nil {
			p.metrics.RecordConfigOperation(namespace, "get", "error")
		}
		return "", WrapClientError(lastErr, ErrCodeConfigGetFailed, "failed to get config value")
	}

	log.Infof("Successfully got config value - Namespace: [%s], Key: [%s]", namespace, key)
	return value, nil
}

// getConfigValueFromApollo gets configuration value from Apollo client
func (p *PlugApollo) getConfigValueFromApollo(namespace, key string) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("Apollo client not initialized")
	}

	// Check cache first if enabled
	if p.conf.EnableCache {
		cacheKey := fmt.Sprintf("%s:%s", namespace, key)
		p.cacheMutex.RLock()
		if cached, exists := p.configCache[cacheKey]; exists {
			if value, ok := cached.(string); ok {
				p.cacheMutex.RUnlock()
				if p.metrics != nil {
					p.metrics.RecordCacheHit()
				}
				return value, nil
			}
		}
		p.cacheMutex.RUnlock()
		if p.metrics != nil {
			p.metrics.RecordCacheMiss()
		}
	}

	// Get from Apollo
	ctx := context.Background()
	value, err := p.client.GetConfigValue(ctx, namespace, key)
	if err != nil {
		return "", err
	}

	// Cache the value if enabled
	if p.conf.EnableCache {
		cacheKey := fmt.Sprintf("%s:%s", namespace, key)
		p.cacheMutex.Lock()
		p.configCache[cacheKey] = value
		p.cacheMutex.Unlock()
	}

	return value, nil
}

// ApolloConfigSource implements config.Source for Apollo
type ApolloConfigSource struct {
	client    *ApolloHTTPClient // Apollo HTTP client
	appId     string
	cluster   string
	namespace string
}

// NewApolloConfigSource creates a new Apollo config source
func NewApolloConfigSource(client *ApolloHTTPClient, appId, cluster, namespace string) *ApolloConfigSource {
	return &ApolloConfigSource{
		client:    client,
		appId:     appId,
		cluster:   cluster,
		namespace: namespace,
	}
}

// Load implements config.Source interface
func (s *ApolloConfigSource) Load() ([]*config.KeyValue, error) {
	client, ok := s.client.(*ApolloHTTPClient)
	if !ok {
		return nil, fmt.Errorf("invalid Apollo client type")
	}

	ctx := context.Background()
	configResp, err := client.GetConfig(ctx, s.namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from Apollo: %w", err)
	}

	var kvs []*config.KeyValue
	for key, value := range configResp.Configurations {
		kvs = append(kvs, &config.KeyValue{
			Key:   key,
			Value: []byte(value),
		})
	}

	return kvs, nil
}

// Watch implements config.Source interface
func (s *ApolloConfigSource) Watch() (config.Watcher, error) {
	client, ok := s.client.(*ApolloHTTPClient)
	if !ok {
		return nil, fmt.Errorf("invalid Apollo client type")
	}

	// Create a config watcher adapter
	watcher := NewApolloConfigWatcher(client, s.namespace)
	return watcher, nil
}

