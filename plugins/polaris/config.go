package polaris

import (
	"fmt"
	"github.com/polarismesh/polaris-go/pkg/model"
	"os"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"gopkg.in/yaml.v3"
)

// ConfigAdapter configuration adapter
// Responsibility: provide Polaris configuration center related functionality

// GetConfig gets configuration from Polaris configuration center
// This method retrieves the corresponding configuration source from Polaris configuration center based on the provided configuration file name and group name
func (p *PlugPolaris) GetConfig(fileName string, group string) (config.Source, error) {
	return GetPolaris().Config(
		polaris.WithConfigFile(
			polaris.File{
				Name:  fileName,
				Group: group,
			}))
}

// GetConfigValue gets configuration value
func (p *PlugPolaris) GetConfigValue(fileName, group string) (string, error) {
	if err := p.checkInitialized(); err != nil {
		return "", err
	}

	// Record configuration operation metrics
	if p.metrics != nil {
		p.metrics.RecordConfigOperation("get", fileName, group, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordConfigOperation("get", fileName, group, "success")
			}
		}()
	}

	log.Infof("Getting configFile: %s, group: %s", fileName, group)

	// Create Config API client
	configAPI := api.NewConfigFileAPIBySDKContext(p.sdk)
	if configAPI == nil {
		return "", NewInitError("failed to create configFile API")
	}

	// Execute with circuit breaker and retry mechanism
	var configFile model.ConfigFile
	var lastErr error

	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// Call SDK API to get configuration
			cfg, err := configAPI.GetConfigFile(p.conf.Namespace, group, fileName)
			if err != nil {
				lastErr = err
				return err
			}
			configFile = cfg
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to get configFile %s:%s after retries: %v", fileName, group, err)
		if p.metrics != nil {
			p.metrics.RecordConfigOperation("get", fileName, group, "error")
		}
		return "", WrapServiceError(lastErr, ErrCodeConfigGetFailed, "failed to get configFile value")
	}

	// Check if configuration exists
	if configFile == nil {
		log.Warnf("Config %s:%s not found", fileName, group)
		return "", NewServiceError(ErrCodeConfigNotFound, "configFile not found")
	}

	// Get configuration content
	content := configFile.GetContent()
	log.Infof("Successfully got configFile %s:%s, content length: %d", fileName, group, len(content))
	return content, nil
}

// loadPolarisConfiguration loads Polaris SDK configuration file and initializes SDK
// This method determines whether to use custom configuration file based on the config_path field in configuration
func (p *PlugPolaris) loadPolarisConfiguration() (api.SDKContext, error) {
	// Create basic configuration
	configuration := api.NewConfiguration()

	if p.conf.ConfigPath != "" {
		// Check if configuration file exists
		if _, err := os.Stat(p.conf.ConfigPath); os.IsNotExist(err) {
			log.Warnf("Polaris configuration file not found: %s, using default configuration", p.conf.ConfigPath)
		} else {
			log.Infof("Loading Polaris SDK configuration from: %s", p.conf.ConfigPath)

			// Read configuration file content
			configData, err := os.ReadFile(p.conf.ConfigPath)
			if err != nil {
				log.Errorf("Failed to read Polaris configuration file: %v", err)
				return nil, fmt.Errorf("failed to read Polaris configuration file: %w", err)
			}

			// Parse YAML configuration file
			var polarisConfig map[string]any
			if err := yaml.Unmarshal(configData, &polarisConfig); err != nil {
				log.Errorf("Failed to parse Polaris configuration file: %v", err)
				return nil, fmt.Errorf("failed to parse Polaris configuration file: %w", err)
			}

			// Set SDK configuration based on configuration file content
			if err := p.applyPolarisConfig(configuration, polarisConfig); err != nil {
				log.Errorf("Failed to apply Polaris configuration: %v", err)
				return nil, fmt.Errorf("failed to apply Polaris configuration: %w", err)
			}

			log.Infof("Successfully loaded Polaris configuration from: %s", p.conf.ConfigPath)
		}
	} else {
		log.Info("Using default Polaris SDK configuration")
	}

	// Initialize SDK context
	sdk, err := api.InitContextByConfig(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Polaris SDK context: %w", err)
	}

	return sdk, nil
}

// applyPolarisConfig applies parsed configuration file content to SDK configuration object
func (p *PlugPolaris) applyPolarisConfig(_ any, polarisConfig map[string]any) error {
	// Validate basic structure of configuration file
	if polarisConfig == nil {
		return fmt.Errorf("polaris config is nil")
	}

	// Safely extract and validate configuration items
	if err := p.validateAndLogConfig(polarisConfig); err != nil {
		return fmt.Errorf("failed to validate polaris config: %w", err)
	}

	// Set environment variable for Polaris SDK to read configuration file
	err := os.Setenv("POLARIS_CONFIG_PATH", p.conf.ConfigPath)
	if err != nil {
		return err
	}
	log.Infof("Set POLARIS_CONFIG_PATH environment variable to: %s", p.conf.ConfigPath)

	return nil
}

// validateAndLogConfig safely validates and logs configuration information
func (p *PlugPolaris) validateAndLogConfig(polarisConfig map[string]any) error {
	// Validate global configuration
	if global, exists := polarisConfig["global"]; exists {
		if globalMap, ok := global.(map[string]any); ok {
			if err := p.validateGlobalConfig(globalMap); err != nil {
				return fmt.Errorf("invalid global config: %w", err)
			}
		} else {
			log.Warnf("Global config is not a map, skipping validation")
		}
	}

	// Validate configuration center configuration
	if configSection, exists := polarisConfig["config"]; exists {
		if configMap, ok := configSection.(map[string]any); ok {
			if err := p.validateConfigSection(configMap); err != nil {
				return fmt.Errorf("invalid config section: %w", err)
			}
		} else {
			log.Warnf("Config section is not a map, skipping validation")
		}
	}

	return nil
}

// validateGlobalConfig validates global configuration
func (p *PlugPolaris) validateGlobalConfig(global map[string]any) error {
	// Validate server connector configuration
	if serverConnector, exists := global["serverConnector"]; exists {
		if connectorMap, ok := serverConnector.(map[string]any); ok {
			if err := p.validateServerConnector(connectorMap); err != nil {
				return fmt.Errorf("invalid server connector config: %w", err)
			}
		} else {
			log.Warnf("Server connector config is not a map")
		}
	}

	// Validate statistics reporter configuration
	if statReporter, exists := global["statReporter"]; exists {
		if reporterMap, ok := statReporter.(map[string]any); ok {
			if err := p.validateStatReporter(reporterMap); err != nil {
				return fmt.Errorf("invalid stat reporter config: %w", err)
			}
		} else {
			log.Warnf("Stat reporter config is not a map")
		}
	}

	return nil
}

// validateServerConnector validates server connector configuration
func (p *PlugPolaris) validateServerConnector(connector map[string]any) error {
	// Validate protocol
	if protocol, exists := connector["protocol"]; exists {
		if protocolStr, ok := protocol.(string); ok {
			if protocolStr != "grpc" && protocolStr != "http" {
				log.Warnf("Unsupported protocol: %s, expected grpc or http", protocolStr)
			} else {
				log.Infof("Using protocol: %s", protocolStr)
			}
		} else {
			log.Warnf("Protocol is not a string")
		}
	}

	// 验证地址列表
	if addresses, exists := connector["addresses"]; exists {
		if addressList, ok := addresses.([]any); ok {
			for i, addr := range addressList {
				if addrStr, ok := addr.(string); ok {
					log.Infof("Server address %d: %s", i+1, addrStr)
				} else {
					log.Warnf("Address %d is not a string", i+1)
				}
			}
		} else {
			log.Warnf("Addresses is not a list")
		}
	} else {
		log.Warnf("No server addresses configured")
	}

	return nil
}

// validateStatReporter validates statistics reporter configuration
func (p *PlugPolaris) validateStatReporter(reporter map[string]any) error {
	// Validate if enabled
	if enable, exists := reporter["enable"]; exists {
		if enableBool, ok := enable.(bool); ok {
			if enableBool {
				log.Info("Stat reporter is enabled")
			} else {
				log.Info("Stat reporter is disabled")
			}
		} else {
			log.Warnf("Enable flag is not a boolean")
		}
	}

	// Validate chain configuration
	if chain, exists := reporter["chain"]; exists {
		if chainList, ok := chain.([]any); ok {
			for i, item := range chainList {
				if itemStr, ok := item.(string); ok {
					log.Infof("Stat reporter chain %d: %s", i+1, itemStr)
				} else {
					log.Warnf("Chain item %d is not a string", i+1)
				}
			}
		} else {
			log.Warnf("Chain is not a list")
		}
	}

	return nil
}

// validateConfigSection validates configuration center configuration
func (p *PlugPolaris) validateConfigSection(config map[string]any) error {
	// Validate configuration connector
	if configConnector, exists := config["configConnector"]; exists {
		if connectorMap, ok := configConnector.(map[string]any); ok {
			if err := p.validateConfigConnector(connectorMap); err != nil {
				return fmt.Errorf("invalid config connector: %w", err)
			}
		} else {
			log.Warnf("Config connector is not a map")
		}
	}

	return nil
}

// validateConfigConnector validates configuration connector
func (p *PlugPolaris) validateConfigConnector(connector map[string]any) error {
	// Validate address list
	if addresses, exists := connector["addresses"]; exists {
		if addressList, ok := addresses.([]any); ok {
			for i, addr := range addressList {
				if addrStr, ok := addr.(string); ok {
					log.Infof("Config center address %d: %s", i+1, addrStr)
				} else {
					log.Warnf("Config center address %d is not a string", i+1)
				}
			}
		} else {
			log.Warnf("Config center addresses is not a list")
		}
	} else {
		log.Warnf("No config center addresses configured")
	}

	return nil
}
