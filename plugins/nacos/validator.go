package nacos

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/plugins/nacos/conf"
)

// validateConfig validates Nacos configuration
func (p *PlugNacos) validateConfig() error {
	if p.conf == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate server addresses
	if p.conf.ServerAddresses == "" && p.conf.Endpoint == "" {
		return fmt.Errorf("server_addresses or endpoint must be configured")
	}

	// Validate namespace
	if p.conf.NamespaceId == "" && p.conf.Namespace == "" {
		// Use default namespace
		p.conf.Namespace = conf.DefaultNamespace
	}

	// Validate service config if enable_register is true
	if p.conf.EnableRegister && p.conf.ServiceConfig != nil {
		if err := p.validateServiceConfig(p.conf.ServiceConfig); err != nil {
			return fmt.Errorf("service_config validation failed: %w", err)
		}
	}

	// Validate additional configs
	for i, cfg := range p.conf.AdditionalConfigs {
		if cfg.DataId == "" {
			return fmt.Errorf("additional_configs[%d].data_id is required", i)
		}
		if cfg.Group == "" {
			cfg.Group = conf.DefaultGroup
		}
	}

	return nil
}

// validateServiceConfig validates service configuration
func (p *PlugNacos) validateServiceConfig(sc *conf.ServiceConfig) error {
	if sc == nil {
		return nil
	}

	// Validate health check configuration
	if sc.HealthCheck {
		if sc.HealthCheckType == "" {
			sc.HealthCheckType = conf.DefaultHealthCheckType
		}
		if sc.HealthCheckInterval == 0 {
			sc.HealthCheckInterval = conf.DefaultHealthCheckInterval
		}
		if sc.HealthCheckTimeout == 0 {
			sc.HealthCheckTimeout = conf.DefaultHealthCheckTimeout
		}
		if sc.HealthCheckType == "http" && sc.HealthCheckUrl == "" {
			return fmt.Errorf("health_check_url is required when health_check_type is http")
		}
	}

	// Validate group and cluster
	if sc.Group == "" {
		sc.Group = conf.DefaultGroup
	}
	if sc.Cluster == "" {
		sc.Cluster = conf.DefaultCluster
	}

	return nil
}

// setDefaultConfig sets default configuration values
func (p *PlugNacos) setDefaultConfig() {
	if p.conf == nil {
		return
	}

	// Set default namespace
	if p.conf.NamespaceId == "" && p.conf.Namespace == "" {
		p.conf.Namespace = conf.DefaultNamespace
	}

	// Set default weight
	if p.conf.Weight == 0 {
		p.conf.Weight = conf.DefaultWeight
	}

	// Set default timeout
	if p.conf.Timeout == 0 {
		p.conf.Timeout = conf.DefaultTimeout
	}

	// Set default notify timeout
	if p.conf.NotifyTimeout == 0 {
		p.conf.NotifyTimeout = conf.DefaultNotifyTimeout
	}

	// Set default log level
	if p.conf.LogLevel == "" {
		p.conf.LogLevel = conf.DefaultLogLevel
	}

	// Set default log directory
	if p.conf.LogDir == "" {
		p.conf.LogDir = conf.DefaultLogDir
	}

	// Set default cache directory
	if p.conf.CacheDir == "" {
		p.conf.CacheDir = conf.DefaultCacheDir
	}

	// Set default context path
	if p.conf.ContextPath == "" {
		p.conf.ContextPath = conf.DefaultContextPath
	}

	// Set default service config
	if p.conf.ServiceConfig != nil {
		if p.conf.ServiceConfig.Group == "" {
			p.conf.ServiceConfig.Group = conf.DefaultGroup
		}
		if p.conf.ServiceConfig.Cluster == "" {
			p.conf.ServiceConfig.Cluster = conf.DefaultCluster
		}
	}
}

// normalizeServerAddresses normalizes server addresses
func normalizeServerAddresses(addresses string) []string {
	if addresses == "" {
		return []string{"127.0.0.1:8848"}
	}

	parts := strings.Split(addresses, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			// Remove protocol prefix if present
			part = strings.TrimPrefix(part, "http://")
			part = strings.TrimPrefix(part, "https://")
			result = append(result, part)
		}
	}

	if len(result) == 0 {
		return []string{"127.0.0.1:8848"}
	}

	return result
}

