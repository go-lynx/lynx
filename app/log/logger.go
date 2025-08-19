// Package log provides core application functionality for the Lynx framework
package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	kconf "github.com/go-kratos/kratos/v2/config"
	lconf "github.com/go-lynx/lynx/app/log/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// unified level filtering (kratos + zerolog)
	kratosMinLevel = log.LevelInfo

	// caller depth configurable
	callerSkipDefault = 5
	callerSkipCurrent = 5

	// timezone for logging
	tzLoc  *time.Location
	tzName string
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

	// Set global time format and timezone
	zerolog.TimeFieldFormat = time.RFC3339Nano
	// Prefer explicit timezone string: lynx.log.timezone (e.g., "Asia/Shanghai", "UTC")
	if tz, err := cfg.Value("lynx.log.timezone").String(); err == nil && tz != "" {
		if loc, e := time.LoadLocation(tz); e == nil {
			tzLoc = loc
			tzName = tz
			zerolog.TimestampFunc = func() time.Time { return time.Now().In(tzLoc) }
		} else {
			// invalid timezone -> default to Local
			tzLoc = time.Local
			tzName = "Local"
			zerolog.TimestampFunc = func() time.Time { return time.Now().In(tzLoc) }
		}
	} else {
		// no timezone configured -> default to Local
		tzLoc = time.Local
		tzName = "Local"
		zerolog.TimestampFunc = func() time.Time { return time.Now().In(tzLoc) }
	}

	// 设置全局日志级别（统一 zerolog + kratos）
	logLevel := strings.ToLower(logConfig.GetLevel())
	switch logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		kratosMinLevel = log.LevelDebug
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		kratosMinLevel = log.LevelInfo
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		kratosMinLevel = log.LevelWarn
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		kratosMinLevel = log.LevelError
	default:
		// 默认使用 Info 级别
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		kratosMinLevel = log.LevelInfo
	}

	// callerSkip 可配置
	callerSkipCurrent = callerSkipDefault
	if v := logConfig.GetCallerSkip(); v > 0 {
		callerSkipCurrent = int(v)
	}

	// 用 zeroLogger 初始化底层日志器
	zeroLogger := zerolog.New(output).With().Timestamp().Logger()

	// Initialize the main logger with level filter and default fields
	base := zeroLogLogger{zeroLogger}
	filtered := log.NewFilter(base, log.FilterLevel(kratosMinLevel))
	logger := log.With(
		filtered,
		"caller", Caller(callerSkipCurrent),
		"service.id", host,
		"service.name", name,
		"service.version", version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
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
	// 更新原子 helper，避免热更新期间的数据竞争
	helperStore.Store(lHelper)

	// Initialize and display the application banner
	// 初始化并显示应用启动横幅
	if err := initBanner(cfg); err != nil {
		// 若横幅初始化失败，记录警告信息，但不影响程序继续执行
		lHelper.Warnf("failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	// 先尝试基于配置源的 Watch 机制（例如本地文件、Polaris 等可能支持）
	apply := func(nc *lconf.Log) {
		// 应用更新：优先使用 timezone 字符串，否则默认 Local
		if tz, err := cfg.Value("lynx.log.timezone").String(); err == nil && tz != "" {
			if loc, e := time.LoadLocation(tz); e == nil {
				tzLoc = loc
				tzName = tz
			} else {
				tzLoc = time.Local
				tzName = "Local"
			}
		} else {
			tzLoc = time.Local
			tzName = "Local"
		}

		switch strings.ToLower(nc.GetLevel()) {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			kratosMinLevel = log.LevelDebug
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			kratosMinLevel = log.LevelInfo
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
			kratosMinLevel = log.LevelWarn
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			kratosMinLevel = log.LevelError
		}

		callerSkipCurrent = callerSkipDefault
		if v := nc.GetCallerSkip(); v > 0 {
			callerSkipCurrent = int(v)
		}

		// 重建 logger
		newLogger := log.With(
			log.NewFilter(base, log.FilterLevel(kratosMinLevel)),
			"caller", Caller(callerSkipCurrent),
			"service.id", host,
			"service.name", name,
			"service.version", version,
			"trace.id", tracing.TraceID(),
			"span.id", tracing.SpanID(),
		)
		if newLogger != nil {
			Logger = newLogger
			newHelper := log.NewHelper(newLogger)
			LHelper = *newHelper
		}
	}

	// 使用 Watch，如果不支持则回退到轮询
	if err := cfg.Watch("lynx.log", func(key string, v kconf.Value) {
		var nc lconf.Log
		if err := v.Scan(&nc); err != nil {
			return
		}
		apply(&nc)
	}); err != nil {
		// 轻量轮询热更新（后端不支持 Watch 时的降级方案）
		// 每 2s 读取一次 lynx.log，配置变化则应用
		go func() {
			// 生成签名函数
			signature := func(c *lconf.Log) string {
				if c == nil {
					return ""
				}
				var sb strings.Builder
				// include timezone name if present
				fmt.Fprintf(&sb, "lvl=%s;tz=%s;caller=%d;", strings.ToLower(c.GetLevel()), tzName, c.GetCallerSkip())
				return sb.String()
			}

			prevSig := signature(&logConfig)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				var nc lconf.Log
				if err := cfg.Value("lynx.log").Scan(&nc); err != nil {
					continue
				}
				sig := signature(&nc)
				if sig == prevSig {
					continue
				}
				prevSig = sig
				apply(&nc)
			}
		}()
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
