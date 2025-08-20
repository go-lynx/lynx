package boot

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

// LoadBootstrapConfig 从本地文件或目录加载引导配置。
// 它从配置管理器指定的路径读取配置，并初始化应用程序的配置状态。
//
// 返回值:
//   - error: 配置加载过程中发生的任何错误
func (app *Application) LoadBootstrapConfig() error {
	// 检查 Application 实例是否为 nil
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot load bootstrap configuration")
	}

	// 获取配置路径
	configMgr := GetConfigManager()
	configPath := configMgr.GetConfigPath()

	// 检查配置路径是否为空
	if configPath == "" {
		return fmt.Errorf("configuration path is empty: please specify config path via -conf flag or LYNX_CONFIG_PATH environment variable")
	}

	// 记录尝试加载配置的日志
	log.Infof("loading local bootstrap configuration from: %s", configPath)

	// 从本地文件创建配置源
	source := file.NewSource(configPath)
	// 检查配置源是否创建成功
	if source == nil {
		return fmt.Errorf("failed to create configuration source from: %s", configPath)
	}

	// 创建新的配置实例
	cfg := config.New(
		// 指定配置源
		config.WithSource(source),
	)
	// 检查配置实例是否创建成功
	if cfg == nil {
		return fmt.Errorf("failed to create configuration instance")
	}

	// 加载配置
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load configuration from %s: %w", configPath, err)
	}

	// 验证配置
	if err := app.validateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// 在设置清理操作之前存储配置
	// 这样可以确保如果清理设置失败，不会丢失配置引用
	app.conf = cfg

	// 设置配置清理操作 - 如果失败则返回错误
	if err := app.setupConfigCleanup(cfg); err != nil {
		return fmt.Errorf("failed to setup configuration cleanup: %w", err)
	}

	return nil
}

// validateConfig 验证配置的完整性和正确性
func (app *Application) validateConfig(cfg config.Config) error {
	// 检查配置实例是否为 nil
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil")
	}

	// 检查必要的配置项是否存在
	// 这里可以根据实际需求添加更多验证逻辑
	requiredKeys := []string{
		"lynx.name",
		"lynx.version",
		"lynx.host",
	}

	for _, key := range requiredKeys {
		if _, err := cfg.Value(key).String(); err != nil {
			return fmt.Errorf("required configuration key '%s' is missing or invalid: %w", key, err)
		}
	}

	return nil
}

// setupConfigCleanup 确保在配置资源不再需要时进行正确的清理。
func (app *Application) setupConfigCleanup(cfg config.Config) error {
	// 检查配置实例是否为 nil
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil: cannot setup cleanup for nil config")
	}

	// 设置延迟清理函数
	app.cleanup = func() {
		// 关闭配置资源
		if err := cfg.Close(); err != nil {
			// 记录关闭配置失败的日志
			log.Errorf("failed to close configuration: %v", err)
		}
	}

	return nil
}

// GetName 获取应用程序名称
func (app *Application) GetName() string {
	if app.conf != nil {
		if name, err := app.conf.Value("lynx.name").String(); err == nil {
			return name
		}
	}
	return "lynx"
}

// GetHost 获取应用程序主机
func (app *Application) GetHost() string {
	if app.conf != nil {
		if host, err := app.conf.Value("lynx.host").String(); err == nil {
			return host
		}
	}
	return "localhost"
}

// GetVersion 获取应用程序版本
func (app *Application) GetVersion() string {
	if app.conf != nil {
		if version, err := app.conf.Value("lynx.version").String(); err == nil {
			return version
		}
	}
	return "unknown"
}
