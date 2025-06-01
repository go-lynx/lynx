// Package app provides core application functionality for the Lynx framework
package log

import (
	"context"
	"embed"
	"fmt"
	kconf "github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/conf"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/rs/zerolog"
)

var (
	// Embedded banner file for application startup
	// 使用 //go:embed 指令将 banner.txt 文件嵌入到程序中
	//
	//go:embed banner.txt
	bannerFS embed.FS
)

// InitLogger initializes the application's logging system.
// 初始化应用的日志系统，设置主日志记录器并配置各种日志字段，
// 如时间戳、调用者信息、服务详情和追踪 ID 等。
// 返回一个错误对象，如果初始化过程中出现错误则返回相应错误，否则返回 nil。
func InitLogger(name string, host string, version string, cfg kconf.Config) error {
	// 检查 LynxApp 实例是否为 nil，如果为 nil 则返回错误
	if cfg == nil {
		return fmt.Errorf("lynx app instance is nil")
	}

	// Log the initialization of the logging component
	// 记录日志组件初始化开始的信息
	log.Info("Initializing Lynx logging component")

	// 启用控制台彩色输出
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339Nano,
	}

	// 用 zeroLogger 初始化底层日志器
	zeroLogger := zerolog.New(output).With().Timestamp().Logger()

	// Initialize the main logger with standard output and default fields
	// 初始化主日志记录器，将日志输出到标准输出，并设置默认日志字段
	logger := log.With(
		zeroLogLogger{zeroLogger},
		"caller", Caller(5), // 记录日志调用者信息
		"service.id", host, // 记录服务 ID，由 GetHost 函数提供
		"service.name", name, // 记录服务名称，由 GetName 函数提供
		"service.version", version, // 记录服务版本，由 GetVersion 函数提供
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
	Logger = logger
	LogHelper = *helper

	// Initialize and display the application banner
	// 初始化并显示应用启动横幅
	if err := initBanner(cfg); err != nil {
		// 若横幅初始化失败，记录警告信息，但不影响程序继续执行
		helper.Warnf("failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	// Log successful initialization
	// 记录日志组件初始化成功的信息
	helper.Info("lynx application logging component initialized successfully")

	return nil
}

// Caller returns a Valuer that returns a pkg/file:line description of the caller.
func Caller(depth int) log.Valuer {
	return func(context.Context) any {
		_, file, line, _ := runtime.Caller(depth)
		return trimFilePath(file, 3) + ":" + strconv.Itoa(line)
	}
}

func trimFilePath(file string, depth int) string {
	// 记录斜杠位置
	var slashPos []int
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			slashPos = append(slashPos, i)
			if len(slashPos) == depth {
				break
			}
		}
	}
	if len(slashPos) == 0 {
		return file // 没有斜杠，直接返回文件名
	}
	// 从最后第 depth 个 / 开始截取
	start := slashPos[len(slashPos)-1] + 1
	return file[start:]
}

// initBanner handles the initialization and display of the application banner.
// 处理应用启动横幅的初始化和显示操作。
// 从嵌入的文件系统中读取横幅内容，并根据配置决定是否显示横幅。
// 返回一个错误对象，如果初始化过程中出现错误则返回相应错误，否则返回 nil。
func initBanner(cfg kconf.Config) error {
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
	if err := cfg.Scan(&bootConfig); err != nil {
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
		_, err := fmt.Fprintln(os.Stdout, string(bannerData))
		if err != nil {
			return err
		}
	}

	return nil
}

type zeroLogLogger struct {
	logger zerolog.Logger
}

func (l zeroLogLogger) Log(level log.Level, keyvals ...interface{}) error {
	var event *zerolog.Event

	// 根据日志等级创建对应的 event
	switch level {
	case log.LevelDebug:
		event = l.logger.Debug()
	case log.LevelInfo:
		event = l.logger.Info()
	case log.LevelWarn:
		event = l.logger.Warn()
	case log.LevelError:
		event = l.logger.Error()
	case log.LevelFatal:
		event = l.logger.Fatal()
	default:
		event = l.logger.Info()
	}

	// 加 key-value 字段
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key, ok := keyvals[i].(string)
			if !ok {
				key = fmt.Sprintf("BAD_KEY_%d", i)
			}
			event = event.Interface(key, keyvals[i+1])
		}
	}

	event.Msg("") // 最终输出
	return nil
}
