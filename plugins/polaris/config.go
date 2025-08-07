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

// ConfigAdapter 配置适配器
// 职责：提供 Polaris 配置中心的相关功能

// GetConfig 从 Polaris 配置中心获取配置
// 该方法会根据传入的配置文件名和配置文件组名，从 Polaris 配置中心获取对应的配置源
func (p *PlugPolaris) GetConfig(fileName string, group string) (config.Source, error) {
	return GetPolaris().Config(
		polaris.WithConfigFile(
			polaris.File{
				Name:  fileName,
				Group: group,
			}))
}

// GetConfigValue 获取配置值
func (p *PlugPolaris) GetConfigValue(fileName, group string) (string, error) {
	if err := p.checkInitialized(); err != nil {
		return "", err
	}

	// 记录配置操作指标
	if p.metrics != nil {
		p.metrics.RecordConfigOperation("get", fileName, group, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordConfigOperation("get", fileName, group, "success")
			}
		}()
	}

	log.Infof("Getting configFile: %s, group: %s", fileName, group)

	// 创建 Config API 客户端
	configAPI := api.NewConfigFileAPIBySDKContext(p.sdk)
	if configAPI == nil {
		return "", NewInitError("failed to create configFile API")
	}

	// 使用熔断器和重试机制执行操作
	var configFile model.ConfigFile
	var lastErr error

	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 调用 SDK API 获取配置
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

	// 检查配置是否存在
	if configFile == nil {
		log.Warnf("Config %s:%s not found", fileName, group)
		return "", NewServiceError(ErrCodeConfigNotFound, "configFile not found")
	}

	// 获取配置内容
	content := configFile.GetContent()
	log.Infof("Successfully got configFile %s:%s, content length: %d", fileName, group, len(content))
	return content, nil
}

// loadPolarisConfiguration 加载 Polaris SDK 配置文件并初始化 SDK
// 该方法会根据配置中的 config_path 字段来决定是否使用自定义配置文件
func (p *PlugPolaris) loadPolarisConfiguration() (api.SDKContext, error) {
	// 创建基础配置
	configuration := api.NewConfiguration()

	if p.conf.ConfigPath != "" {
		// 检查配置文件是否存在
		if _, err := os.Stat(p.conf.ConfigPath); os.IsNotExist(err) {
			log.Warnf("Polaris configuration file not found: %s, using default configuration", p.conf.ConfigPath)
		} else {
			log.Infof("Loading Polaris SDK configuration from: %s", p.conf.ConfigPath)

			// 读取配置文件内容
			configData, err := os.ReadFile(p.conf.ConfigPath)
			if err != nil {
				log.Errorf("Failed to read Polaris configuration file: %v", err)
				return nil, fmt.Errorf("failed to read Polaris configuration file: %w", err)
			}

			// 解析 YAML 配置文件
			var polarisConfig map[string]any
			if err := yaml.Unmarshal(configData, &polarisConfig); err != nil {
				log.Errorf("Failed to parse Polaris configuration file: %v", err)
				return nil, fmt.Errorf("failed to parse Polaris configuration file: %w", err)
			}

			// 根据配置文件内容设置 SDK 配置
			if err := p.applyPolarisConfig(configuration, polarisConfig); err != nil {
				log.Errorf("Failed to apply Polaris configuration: %v", err)
				return nil, fmt.Errorf("failed to apply Polaris configuration: %w", err)
			}

			log.Infof("Successfully loaded Polaris configuration from: %s", p.conf.ConfigPath)
		}
	} else {
		log.Info("Using default Polaris SDK configuration")
	}

	// 初始化 SDK 上下文
	sdk, err := api.InitContextByConfig(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Polaris SDK context: %w", err)
	}

	return sdk, nil
}

// applyPolarisConfig 将解析的配置文件内容应用到 SDK 配置对象
func (p *PlugPolaris) applyPolarisConfig(_ any, polarisConfig map[string]any) error {
	// 验证配置文件的基本结构
	if polarisConfig == nil {
		return fmt.Errorf("polaris config is nil")
	}

	// 安全地提取和验证配置项
	if err := p.validateAndLogConfig(polarisConfig); err != nil {
		return fmt.Errorf("failed to validate polaris config: %w", err)
	}

	// 设置环境变量让 Polaris SDK 读取配置文件
	err := os.Setenv("POLARIS_CONFIG_PATH", p.conf.ConfigPath)
	if err != nil {
		return err
	}
	log.Infof("Set POLARIS_CONFIG_PATH environment variable to: %s", p.conf.ConfigPath)

	return nil
}

// validateAndLogConfig 安全地验证和记录配置信息
func (p *PlugPolaris) validateAndLogConfig(polarisConfig map[string]any) error {
	// 验证全局配置
	if global, exists := polarisConfig["global"]; exists {
		if globalMap, ok := global.(map[string]any); ok {
			if err := p.validateGlobalConfig(globalMap); err != nil {
				return fmt.Errorf("invalid global config: %w", err)
			}
		} else {
			log.Warnf("Global config is not a map, skipping validation")
		}
	}

	// 验证配置中心配置
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

// validateGlobalConfig 验证全局配置
func (p *PlugPolaris) validateGlobalConfig(global map[string]any) error {
	// 验证服务器连接器配置
	if serverConnector, exists := global["serverConnector"]; exists {
		if connectorMap, ok := serverConnector.(map[string]any); ok {
			if err := p.validateServerConnector(connectorMap); err != nil {
				return fmt.Errorf("invalid server connector config: %w", err)
			}
		} else {
			log.Warnf("Server connector config is not a map")
		}
	}

	// 验证统计报告器配置
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

// validateServerConnector 验证服务器连接器配置
func (p *PlugPolaris) validateServerConnector(connector map[string]any) error {
	// 验证协议
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

// validateStatReporter 验证统计报告器配置
func (p *PlugPolaris) validateStatReporter(reporter map[string]any) error {
	// 验证是否启用
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

	// 验证链配置
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

// validateConfigSection 验证配置中心配置
func (p *PlugPolaris) validateConfigSection(config map[string]any) error {
	// 验证配置连接器
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

// validateConfigConnector 验证配置连接器
func (p *PlugPolaris) validateConfigConnector(connector map[string]any) error {
	// 验证地址列表
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
