// Package app provides core application functionality for the Lynx framework
package app

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-lynx/lynx/conf"
)

// Embedded banner file for application startup
// 使用 //go:embed 指令将 banner.txt 文件嵌入到程序中
//
//go:embed banner.txt
var bannerFS embed.FS

// InitLogger initializes the application's logging system.
// 初始化应用的日志系统，设置主日志记录器并配置各种日志字段，
// 如时间戳、调用者信息、服务详情和追踪 ID 等。
// 返回一个错误对象，如果初始化过程中出现错误则返回相应错误，否则返回 nil。
func (a *LynxApp) InitLogger() error {
	// 检查 LynxApp 实例是否为 nil，如果为 nil 则返回错误
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}

	// Log the initialization of the logging component
	// 记录日志组件初始化开始的信息
	log.Info("Initializing Lynx logging component")

	// Initialize the main logger with standard output and default fields
	// 初始化主日志记录器，将日志输出到标准输出，并设置默认日志字段
	logger := log.With(
		log.NewStdLogger(os.Stdout),
		"timestamp", log.DefaultTimestamp, // 记录日志时间戳
		"caller", log.DefaultCaller, // 记录日志调用者信息
		"service.id", GetHost(), // 记录服务 ID，由 GetHost 函数提供
		"service.name", GetName(), // 记录服务名称，由 GetName 函数提供
		"service.version", GetVersion(), // 记录服务版本，由 GetVersion 函数提供
		"trace.id", tracing.TraceID(), // 记录追踪 ID
		"span.id", tracing.SpanID(), // 记录跨度 ID
	)

	// 检查日志记录器是否创建失败，如果为 nil 则返回错误
	if logger == nil {
		return fmt.Errorf("failed to create logger")
	}

	// Create a helper for more convenient logging
	// 创建一个日志辅助对象，方便进行日志记录操作
	helper := log.NewHelper(logger)
	// 检查日志辅助对象是否创建失败，如果为 nil 则返回错误
	if helper == nil {
		return fmt.Errorf("failed to create logger helper")
	}

	// Store logger instances
	// 将日志记录器和日志辅助对象存储到 LynxApp 实例中
	a.logger = logger
	a.logHelper = *helper

	// Log successful initialization
	// 记录日志组件初始化成功的信息
	helper.Info("Lynx logging component initialized successfully")

	// Initialize and display the application banner
	// 初始化并显示应用启动横幅
	if err := a.initBanner(); err != nil {
		// 若横幅初始化失败，记录警告信息，但不影响程序继续执行
		helper.Warnf("Failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	return nil
}

// initBanner handles the initialization and display of the application banner.
// 处理应用启动横幅的初始化和显示操作。
// 从嵌入的文件系统中读取横幅内容，并根据配置决定是否显示横幅。
// 返回一个错误对象，如果初始化过程中出现错误则返回相应错误，否则返回 nil。
func (a *LynxApp) initBanner() error {
	// Read banner content from embedded filesystem
	// 从嵌入的文件系统中读取横幅文件内容
	bannerData, err := fs.ReadFile(bannerFS, "banner.txt")
	// 若读取失败，返回错误信息
	if err != nil {
		return fmt.Errorf("failed to read banner: %v", err)
	}

	// Read application configuration
	// 读取应用的启动配置
	var bootConfig conf.Bootstrap
	// 将全局配置扫描到 bootConfig 结构体中
	if err := a.GetGlobalConfig().Scan(&bootConfig); err != nil {
		return fmt.Errorf("failed to read configuration: %v", err)
	}

	// Check if banner display is enabled
	// 检查配置结构是否有效，若无效则返回错误信息
	if bootConfig.GetLynx() == nil ||
		bootConfig.GetLynx().GetApplication() == nil {
		return fmt.Errorf("invalid configuration structure")
	}

	// Display banner if not disabled in configuration
	// 若配置中未禁用横幅显示，则打印横幅内容
	if !bootConfig.GetLynx().GetApplication().GetCloseBanner() {
		a.logHelper.Infof("\n%s", bannerData)
	}

	return nil
}

// GetLogHelper returns the application's log helper instance.
// 该方法用于获取应用的日志辅助对象实例。
// 此辅助对象提供了不同日志级别的便捷记录方法。
// 返回指向日志辅助对象的指针。
func (a *LynxApp) GetLogHelper() *log.Helper {
	return &a.logHelper
}

// GetLogger returns the application's main logger instance.
// 该方法用于获取应用的主日志记录器实例。
// 此日志记录器提供了核心的日志记录功能。
// 返回主日志记录器实例。
func (a *LynxApp) GetLogger() log.Logger {
	return a.logger
}
