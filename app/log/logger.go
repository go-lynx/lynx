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

	// Parse log configuration
	var logConfig lconf.Log
	if err := cfg.Value("lynx.log").Scan(&logConfig); err != nil {
		// If no configuration, use default configuration
		logConfig = lconf.Log{
			Level:         "info",
			ConsoleOutput: true,
		}
	}

	// Set up log output
	var writers []io.Writer

	// Configure console output
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

	// Configure file output
	if logConfig.GetFilePath() != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(logConfig.GetFilePath())
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		// Configure log rotation
		fileWriter := &lumberjack.Logger{
			Filename:   logConfig.GetFilePath(),
			MaxSize:    int(logConfig.GetMaxSizeMb()),  // Maximum size of single file, in MB
			MaxBackups: int(logConfig.GetMaxBackups()), // Maximum number of old files to keep
			MaxAge:     int(logConfig.GetMaxAgeDays()), // Maximum age of old files in days
			Compress:   logConfig.GetCompress(),        // Whether to compress old files
		}
		writers = append(writers, fileWriter)
	}

	// If no output is configured, default to console output
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// Create multi-output writer
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

	// Set global log level (unified zerolog + kratos)
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
		// Default to Info level
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		kratosMinLevel = log.LevelInfo
	}

	// callerSkip is configurable
	callerSkipCurrent = callerSkipDefault
	if v := logConfig.GetCallerSkip(); v > 0 {
		callerSkipCurrent = int(v)
	}

	// Initialize underlying logger with zeroLogger
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

	// Check if logger creation failed, return error if nil
	if logger == nil {
		return fmt.Errorf("failed to create logger")
	}

	// Create a lHelper for more convenient logging
	lHelper := log.NewHelper(logger)
	// Check if logger helper creation failed, return error if nil
	if lHelper == nil {
		return fmt.Errorf("failed to create logger lHelper")
	}

	// Store logger instances
	Logger = logger
	LHelper = *lHelper
	// Update atomic helper to avoid data race during hot updates
	helperStore.Store(lHelper)

	// Initialize and display the application banner
	if err := initBanner(cfg); err != nil {
		// If banner initialization fails, log warning but don't affect program execution
		lHelper.Warnf("failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	// First try Watch mechanism based on configuration source (e.g., local files, Polaris, etc. may support)
	apply := func(nc *lconf.Log) {
		// Apply updates: prefer timezone string, otherwise default to Local
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

		// Rebuild logger
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

	// Use Watch, fallback to polling if not supported
	if err := cfg.Watch("lynx.log", func(key string, v kconf.Value) {
		var nc lconf.Log
		if err := v.Scan(&nc); err != nil {
			return
		}
		apply(&nc)
	}); err != nil {
		// Lightweight polling hot update (fallback when backend doesn't support Watch)
		// Read lynx.log every 2s, apply if configuration changes
		go func() {
			// Generate signature function
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
