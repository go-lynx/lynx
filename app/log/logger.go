// Package log provides core application functionality for the Lynx framework
package log

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	kconf "github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/conf"
	lconf "github.com/go-lynx/lynx/app/log/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
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
// InitLogger initializes the application's logging system with the provided configuration.
// It returns an error if initialization fails.
//
// Parameters:
//   - name: The name of the service
//   - host: The host identifier
//   - version: The service version
//   - cfg: The configuration instance
//
// Returns:
//   - error: An error if initialization fails, nil otherwise
func InitLogger(name string, host string, version string, cfg kconf.Config) error {
	// Validate input parameters
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if cfg == nil {
		return fmt.Errorf("configuration instance cannot be nil")
	}

	// Log the initialization of the logging component
	// Use Info level as this is an important system event
	log.Info("initializing Lynx logging component")

	// 解析日志配置
	var logConfig lconf.Log
	if err := cfg.Value("lynx.log").Scan(&logConfig); err != nil {
		// 如果没有配置，使用默认配置
		logConfig = lconf.Log{
			Level:         "info",
			ConsoleOutput: true,
		}
	}

	// 设置日志输出
	var writers []io.Writer

	// 配置控制台输出
	if logConfig.GetConsoleOutput() {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339Nano,
			NoColor:    false,
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
			},
			FormatMessage: func(i interface{}) string {
				return fmt.Sprintf("msg=\"%v\"", i)
			},
		}
		writers = append(writers, consoleWriter)
	}

	// 配置文件输出
	if logConfig.GetFilePath() != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(logConfig.GetFilePath())
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		// 配置日志轮转
		fileWriter := &lumberjack.Logger{
			Filename:   logConfig.GetFilePath(),
			MaxSize:    int(logConfig.GetMaxSizeMb()),  // 单个文件最大大小，单位 MB
			MaxBackups: int(logConfig.GetMaxBackups()), // 最多保留的旧文件数
			MaxAge:     int(logConfig.GetMaxAgeDays()), // 旧文件最多保留天数
			Compress:   logConfig.GetCompress(),        // 是否压缩旧文件
		}
		writers = append(writers, fileWriter)
	}

	// 如果没有配置任何输出，默认输出到控制台
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// 创建多输出写入器
	output := zerolog.MultiLevelWriter(writers...)

	// Set global time format for consistency
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// 设置全局日志级别
	logLevel := strings.ToLower(logConfig.GetLevel())
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		// 默认使用 Info 级别
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
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

	// Create a lHelper for more convenient logging
	// 创建一个日志辅助对象，方便进行日志记录操作
	lHelper := log.NewHelper(logger)
	// 检查日志辅助对象是否创建失败，如果为 nil 则返回错误
	if lHelper == nil {
		return fmt.Errorf("failed to create logger lHelper")
	}

	// Store logger instances
	// 将日志记录器和日志辅助对象存储到 LynxApp 实例中
	Logger = logger
	LHelper = *lHelper

	// Initialize and display the application banner
	// 初始化并显示应用启动横幅
	if err := initBanner(cfg); err != nil {
		// 若横幅初始化失败，记录警告信息，但不影响程序继续执行
		lHelper.Warnf("failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	// Log successful initialization
	// 记录日志组件初始化成功的信息
	lHelper.Info("lynx application logging component initialized successfully")

	return nil
}

// Caller returns a log.Valuer that provides the caller's source location.
// The depth parameter determines how many stack frames to skip.
//
// Example output: "app/handler/user.go:42"
func Caller(depth int) log.Valuer {
	if depth < 0 {
		depth = 0
	}
	return func(context.Context) any {
		_, file, line, ok := runtime.Caller(depth)
		if !ok {
			return "unknown:0"
		}
		return trimFilePath(file, 3) + ":" + strconv.Itoa(line)
	}
}

// trimFilePath reduces a file path to its last 'depth' components.
// For example, with depth=2: "/a/b/c/d.go" becomes "c/d.go".
//
// Parameters:
//   - file: The full file path
//   - depth: Number of path components to keep
//
// Returns:
//   - The trimmed file path
func trimFilePath(file string, depth int) string {
	if file == "" || depth <= 0 {
		return "unknown"
	}

	// Find the last 'depth' number of slashes
	var slashPos []int
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' { // Handle both Unix and Windows paths
			slashPos = append(slashPos, i)
			if len(slashPos) == depth {
				break
			}
		}
	}

	// If we found fewer slashes than requested depth
	if len(slashPos) == 0 {
		return file
	}

	// Return the path from the last found slash
	start := slashPos[len(slashPos)-1] + 1
	return file[start:]
}

// initBanner initializes and displays the application banner.
// It first attempts to read from a local banner file, then falls back to an embedded banner.
// The banner display can be disabled through application configuration.
//
// Parameters:
//   - cfg: The configuration instance containing banner display preferences
//
// Returns:
//   - error: An error if banner initialization fails, nil otherwise
func initBanner(cfg kconf.Config) error {
	const (
		localBannerPath    = "configs/banner.txt"
		embeddedBannerPath = "banner.txt"
	)

	// Try to read banner data, with fallback options
	bannerData, err := loadBannerData(localBannerPath)
	if err != nil {
		// Log the local file read failure and try embedded banner
		log.Debugf("could not read local banner: %v, falling back to embedded banner", err)
		bannerData, err = fs.ReadFile(bannerFS, embeddedBannerPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded banner: %v", err)
		}
	}

	// Parse application configuration
	var bootConfig conf.Bootstrap
	if err := cfg.Scan(&bootConfig); err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	// Validate configuration structure
	app := bootConfig.GetLynx().GetApplication()
	if app == nil {
		return fmt.Errorf("invalid configuration: application settings not found")
	}

	// Display banner unless explicitly disabled
	if !app.GetCloseBanner() {
		if err := displayBanner(bannerData); err != nil {
			return fmt.Errorf("failed to display banner: %v", err)
		}
	}

	return nil
}

// loadBannerData attempts to read banner data from the specified file.
// It returns the banner content as bytes or an error if the read fails.
func loadBannerData(path string) ([]byte, error) {
	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read banner file: %v", err)
	}

	return data, nil
}

// displayBanner writes the banner data to standard output.
// It returns an error if the write operation fails.
func displayBanner(data []byte) error {
	_, err := fmt.Fprintln(os.Stdout, string(data))
	return err
}

type zeroLogLogger struct {
	logger zerolog.Logger
}

// Log implements the log.Logger interface.
// It converts Kratos log levels to zerolog levels and handles structured logging.
func (l zeroLogLogger) Log(level log.Level, keyvals ...interface{}) error {
	// Validate input parameters
	if len(keyvals)%2 != 0 {
		return fmt.Errorf("number of keyvals must be even")
	}

	// Map Kratos log levels to zerolog levels
	var event *zerolog.Event
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
		// Log unknown levels as warnings and include the original level
		event = l.logger.Warn().Interface("original_level", level)
	}

	// Add structured key-value fields
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("BAD_KEY_%d", i)
			event = event.Interface("original_key", keyvals[i])
		}

		// Special handling for "msg" field
		if key == "msg" {
			if str, ok := keyvals[i+1].(string); ok {
				msg = str
				continue
			}
		}

		// Add the field to the event
		event = event.Interface(key, keyvals[i+1])
	}

	// Output the log entry
	event.Msg(msg)
	return nil
}
