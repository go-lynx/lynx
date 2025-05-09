// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugins"
)

// lynxApp 是 Lynx 应用程序的单例实例。
// 整个应用程序中只有这一个实例，用于协调和管理各组件。
var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp *LynxApp
	// initOnce 用于确保 Lynx 应用程序只初始化一次。
	// 使用 sync.Once 保证在并发环境下初始化操作的原子性。
	initOnce sync.Once
)

// LynxApp represents the main application instance.
// It serves as the central coordinator for all application components,
// managing configuration, logging, plugins, and the control plane.
// LynxApp 代表 Lynx 应用程序的主实例。
// 作为所有应用程序组件的中央协调器，管理配置、日志、插件和控制平面。
type LynxApp struct {
	// host represents the application's host address.
	// Used for network communication and service registration.
	// host 表示应用程序的主机地址。
	// 用于网络通信和服务注册。
	host string

	// name is the unique identifier of the application.
	// Used for service discovery and logging.
	// name 是应用程序的唯一标识符。
	// 用于服务发现和日志记录。
	name string

	// version represents the application's version number.
	// Used for compatibility checks and deployment management.
	// version 表示应用程序的版本号。
	// 用于兼容性检查和部署管理。
	version string

	// cert holds the SSL/TLS certificate configuration.
	// Used for secure communication channels.
	// cert 保存 SSL/TLS 证书配置。
	// 用于安全通信通道。
	cert Cert

	// logger is the primary logging interface.
	// Provides structured logging capabilities for the application.
	// logger 是主要的日志记录接口。
	// 为应用程序提供结构化日志记录功能。
	logger log.Logger

	// logHelper is a convenience wrapper around logger.
	// Provides simplified logging methods with predefined fields.
	// logHelper 是 logger 的便捷包装器。
	// 提供带有预定义字段的简化日志记录方法。
	logHelper log.Helper

	// globalConf holds the application's global configuration.
	// Contains settings that apply across all components.
	// globalConf 保存应用程序的全局配置。
	// 包含适用于所有组件的设置。
	globalConf config.Config

	// controlPlane manages the application's control interface.
	// Handles dynamic configuration updates and system monitoring.
	// controlPlane 管理应用程序的控制接口。
	// 处理动态配置更新和系统监控。
	controlPlane ControlPlane

	// pluginManager handles plugin lifecycle and dependencies.
	// Responsible for loading, unloading, and coordinating plugins.
	// pluginManager 处理插件的生命周期和依赖关系。
	// 负责加载、卸载和协调插件。
	pluginManager LynxPluginManager
}

// Lynx returns the global LynxApp instance.
// It ensures thread-safe access to the singleton instance.
// Lynx 返回全局的 LynxApp 实例。
// 确保线程安全地访问单例实例。
func Lynx() *LynxApp {
	return lynxApp
}

// GetHost retrieves the hostname of the current application instance.
// Returns an empty string if the application is not initialized.
// GetHost 获取当前应用程序实例的主机名。
// 如果应用程序未初始化，则返回空字符串。
func GetHost() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.host
}

// GetName retrieves the application name.
// Returns an empty string if the application is not initialized.
// GetName 获取应用程序名称。
// 如果应用程序未初始化，则返回空字符串。
func GetName() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.name
}

// GetVersion retrieves the application version.
// Returns an empty string if the application is not initialized.
// GetVersion 获取应用程序版本。
// 如果应用程序未初始化，则返回空字符串。
func GetVersion() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.version
}

// NewApp creates a new Lynx application instance with the provided configuration and plugins.
// It initializes the application with system hostname and bootstrap configuration.
//
// Parameters:
//   - cfg: Configuration instance
//   - plugins: Optional list of plugins to initialize with
//
// Returns:
//   - *LynxApp: Initialized application instance
//   - error: Any error that occurred during initialization
//
// NewApp 使用提供的配置和插件创建一个新的 Lynx 应用程序实例。
// 它使用系统主机名和引导配置来初始化应用程序。
//
// 参数:
//   - cfg: 配置实例
//   - plugins: 可选的初始化插件列表
//
// 返回值:
//   - *LynxApp: 初始化后的应用程序实例
//   - error: 初始化过程中发生的任何错误
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// 检查配置是否为 nil，如果为 nil 则返回错误
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	var app *LynxApp
	var err error

	// 使用 sync.Once 确保应用程序只初始化一次
	initOnce.Do(func() {
		app, err = initializeApp(cfg, plugins...)
	})

	// 如果初始化过程中出现错误，则返回错误信息
	if err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	return app, nil
}

// initializeApp handles the actual initialization of the LynxApp instance.
// initializeApp 处理 LynxApp 实例的实际初始化工作。
func initializeApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// Get system hostname
	// 获取系统主机名
	hostname, err := os.Hostname()
	// 如果获取主机名失败，则返回错误信息
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Parse bootstrap configuration
	// 解析引导配置
	var bootConfig conf.Bootstrap
	// 将配置信息扫描到 bootConfig 结构体中，如果失败则返回错误信息
	if err := cfg.Scan(&bootConfig); err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap configuration: %w", err)
	}

	// Validate bootstrap configuration
	// 验证引导配置
	if bootConfig.Lynx == nil || bootConfig.Lynx.Application == nil {
		return nil, fmt.Errorf("invalid bootstrap configuration: missing required fields")
	}

	// Create new application instance
	// 创建新的应用程序实例
	app := &LynxApp{
		host:          hostname,
		name:          bootConfig.Lynx.Application.Name,
		version:       bootConfig.Lynx.Application.Version,
		globalConf:    cfg,
		pluginManager: NewLynxPluginManager(plugins...),
		controlPlane:  &DefaultControlPlane{},
	}

	// Validate required fields
	// 验证必填字段
	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

	// Set global singleton instance
	// 设置全局单例实例
	lynxApp = app

	return app, nil
}

// GetPluginManager returns the plugin manager instance.
// Returns nil if the application is not initialized.
// GetPluginManager 返回插件管理器实例。
// 如果应用程序未初始化，则返回 nil。
func (a *LynxApp) GetPluginManager() LynxPluginManager {
	if a == nil {
		return nil
	}
	return a.pluginManager
}

// GetGlobalConfig returns the global configuration instance.
// Returns nil if the application is not initialized.
// GetGlobalConfig 返回全局配置实例。
// 如果应用程序未初始化，则返回 nil。
func (a *LynxApp) GetGlobalConfig() config.Config {
	if a == nil {
		return nil
	}
	return a.globalConf
}

// SetGlobalConfig updates the global configuration instance.
// It properly closes the existing configuration before updating.
// SetGlobalConfig 更新全局配置实例。
// 在更新之前，会正确关闭现有的配置。
func (a *LynxApp) SetGlobalConfig(cfg config.Config) error {
	// 检查应用程序实例是否为 nil，如果为 nil 则返回错误
	if a == nil {
		return fmt.Errorf("application instance is nil")
	}

	// 检查新配置是否为 nil，如果为 nil 则返回错误
	if cfg == nil {
		return fmt.Errorf("new configuration cannot be nil")
	}

	// Close existing configuration if present
	// 如果现有的全局配置不为 nil，则关闭它
	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			// 记录关闭现有配置失败的错误信息
			a.logHelper.Errorf("Failed to close existing configuration: %v", err)
			return err
		}
	}

	// 更新全局配置
	a.globalConf = cfg
	return nil
}
