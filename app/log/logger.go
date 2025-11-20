// Package log provides core application functionality for the Lynx framework
package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	// base adapter and service metadata for rebuilds
	baseAdapter    zeroLogLogger
	serviceName    string
	serviceHost    string
	serviceVersion string

	// timezone for logging (atomic)
	tzLocAtomic  atomic.Value // *time.Location
	tzNameAtomic atomic.Value // string

	// proxy logger so Kratos app receives a stable logger whose inner can be hot-swapped
	pLogger *proxyLogger

	// Performance optimization components
	bufferedWriters map[string]*BufferedWriter
	asyncWriters    map[string]*AsyncLogWriter
	writersMu       sync.RWMutex
	
	// Performance monitoring
	globalMetrics *LogPerformanceMetrics
	metricsEnabled bool
	
	// Logger initialization state
	loggerInitialized atomic.Bool
	monitorStopCh     chan struct{} // channel to stop performance monitor
	configWatchStopCh chan struct{} // channel to stop config watch goroutine
)

// proxyLogger forwards Log calls to an inner logger stored atomically.
type proxyLogger struct{ inner atomic.Value } // of log.Logger

func (p *proxyLogger) Log(level log.Level, keyvals ...interface{}) error {
	if p == nil {
		return nil
	}
	if v := p.inner.Load(); v != nil {
		if l, ok := v.(log.Logger); ok && l != nil {
			return l.Log(level, keyvals...)
		}
	}
	return nil
}

// GetProxyLogger returns a process-wide proxy logger for passing into Kratos app.
func GetProxyLogger() log.Logger {
	if pLogger == nil {
		pLogger = &proxyLogger{}
	}
	return pLogger
}

// setTimezoneByName updates the timezone atomically. Accepts IANA names or "Local".
func setTimezoneByName(tz string) {
	var loc *time.Location
	name := tz
	if tz == "" || strings.EqualFold(tz, "local") {
		loc = time.Local
		name = "Local"
	} else if l, err := time.LoadLocation(tz); err == nil {
		loc = l
		name = tz
	} else {
		loc = time.Local
		name = "Local"
	}
	tzLocAtomic.Store(loc)
	tzNameAtomic.Store(name)
}

func getTZLoc() *time.Location {
	if v := tzLocAtomic.Load(); v != nil {
		if l, ok := v.(*time.Location); ok && l != nil {
			return l
		}
	}
	return time.Local
}

func getTZName() string {
	if v := tzNameAtomic.Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "Local"
}

// applyLevel sets zerolog and kratos levels in sync.
func applyLevel(lvl log.Level) {
	switch lvl {
	case log.LevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case log.LevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case log.LevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case log.LevelError, log.LevelFatal:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		lvl = log.LevelInfo
	}
	kratosMinLevel = lvl
}

// applyStackFromConfig applies stack configuration from proto config.
func applyStackFromConfig(c *lconf.Log) {
	if c == nil {
		return
	}
	s := c.GetStack()
	if s == nil {
		return
	}
	// map level string to log.Level
	var minLvl log.Level
	switch strings.ToLower(s.GetLevel()) {
	case "debug":
		minLvl = log.LevelDebug
	case "info":
		minLvl = log.LevelInfo
	case "warn":
		minLvl = log.LevelWarn
	case "error":
		minLvl = log.LevelError
	case "fatal":
		minLvl = log.LevelFatal
	default:
		minLvl = log.LevelError
	}
	setStackConfig(
		s.GetEnable(),
		minLvl,
		int(s.GetSkip()),
		int(s.GetMaxFrames()),
		s.GetFilterPrefixes(),
	)
}

// applySamplingFromConfig applies sampling configuration from proto config.
func applySamplingFromConfig(c *lconf.Log) {
	if c == nil {
		return
	}
	sm := c.GetSampling()
	if sm == nil {
		// leave defaults
		return
	}
	setSamplingConfig(
		sm.GetEnable(),
		float64(sm.GetInfoRatio()),
		float64(sm.GetDebugRatio()),
		int(sm.GetMaxInfoPerSec()),
		int(sm.GetMaxDebugPerSec()),
	)
	// Note: setSamplingConfig already updates samplingEnabled flag
}

// rebuildLogger rebuilds Logger and Helper and stores helper atomically.
func rebuildLogger() {
	newLogger := log.With(
		log.NewFilter(baseAdapter, log.FilterLevel(kratosMinLevel)),
		"caller", Caller(callerSkipCurrent),
		"service.id", serviceHost,
		"service.name", serviceName,
		"service.version", serviceVersion,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	if newLogger != nil {
		Logger = newLogger
		newHelper := log.NewHelper(newLogger)
		LHelper = *newHelper
		helperStore.Store(newHelper)
		if pLogger != nil {
			pLogger.inner.Store(newLogger)
		}
	}
}

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

	// Initialize performance components
	bufferedWriters = make(map[string]*BufferedWriter)
	asyncWriters = make(map[string]*AsyncLogWriter)
	globalMetrics = &LogPerformanceMetrics{
		lastReset: time.Now(),
	}
	metricsEnabled = true

	// Log the initialization of the logging component using fmt to avoid uninitialized logger discrepancies
	fmt.Println("[lynx] initializing logging component with performance optimizations")

	// Parse log configuration
	var logConfig lconf.Log
	if err := cfg.Value("lynx.log").Scan(&logConfig); err != nil {
		// If no configuration, use default configuration
		logConfig = lconf.Log{
			Level:         "info",
			ConsoleOutput: true,
		}
	}

	// Set up log output with performance optimizations
	var writers []io.Writer

	// Configure console output with optimizations
	if logConfig.GetConsoleOutput() {
		// Get format configuration
		formatType := "json" // default
		consoleFormat := formatType
		consoleColor := true
		
		// Try to get format config using reflection (until proto is regenerated)
		if formatVal := reflect.ValueOf(&logConfig).Elem(); formatVal.IsValid() {
			if formatField := formatVal.FieldByName("Format"); formatField.IsValid() && formatField.CanInterface() {
				formatPtr := formatField.Interface()
				if formatPtr != nil {
					formatReflect := reflect.ValueOf(formatPtr).Elem()
					if formatReflect.IsValid() {
						if formatTypeVal := formatReflect.FieldByName("Type"); formatTypeVal.IsValid() && formatTypeVal.CanInterface() {
							if t, ok := formatTypeVal.Interface().(string); ok && t != "" {
								formatType = t
							}
						}
						if consoleFormatVal := formatReflect.FieldByName("ConsoleFormat"); consoleFormatVal.IsValid() && consoleFormatVal.CanInterface() {
							if cf, ok := consoleFormatVal.Interface().(string); ok && cf != "" {
								consoleFormat = cf
							} else {
								consoleFormat = formatType
							}
						}
						if consoleColorVal := formatReflect.FieldByName("ConsoleColor"); consoleColorVal.IsValid() && consoleColorVal.CanInterface() {
							if cc, ok := consoleColorVal.Interface().(bool); ok {
								consoleColor = cc
							}
						}
					}
				}
			}
		}
		
		// Create console writer based on format
		consoleWriter := NewConsoleWriter(ConsoleWriterConfig{
			Format:      consoleFormat,
			ColorOutput: consoleColor,
			NoColor:     !consoleColor,
			TimeFormat:  "15:04:05.000",
		})
		
		// Wrap with buffered writer for better performance
		bufferedConsole := NewBufferedWriter(consoleWriter, 32*1024) // 32KB buffer for console
		bufferedWriters["console"] = bufferedConsole
		writers = append(writers, bufferedConsole)
	}

	// Configure file output with optimizations
	if logConfig.GetFilePath() != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(logConfig.GetFilePath())
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		// Configure log rotation
		ms := int(logConfig.GetMaxSizeMb())
		if ms <= 0 {
			ms = 100 // default MB
		}
		mb := int(logConfig.GetMaxBackups())
		if mb < 0 {
			mb = 0 // clamp to non-negative
		}
		ma := int(logConfig.GetMaxAgeDays())
		if ma < 0 {
			ma = 0 // 0 means no age-based removal
		}
		// Get new configuration fields (with fallback for compatibility)
		// Note: These fields will be available after regenerating proto files
		// For now, use reflection to access them if they exist
		maxTotalSizeMB := 0
		rotationStrategyStr := ""
		rotationIntervalStr := ""
		
		// Try to get new fields using reflection
		// If proto file hasn't been regenerated, use defaults
		configVal := reflect.ValueOf(&logConfig).Elem()
		if configVal.IsValid() {
			if field := configVal.FieldByName("MaxTotalSizeMb"); field.IsValid() && field.CanInterface() {
				if val, ok := field.Interface().(int32); ok {
					maxTotalSizeMB = int(val)
				}
			}
			if field := configVal.FieldByName("RotationStrategy"); field.IsValid() && field.CanInterface() {
				if val, ok := field.Interface().(string); ok {
					rotationStrategyStr = val
				}
			}
			if field := configVal.FieldByName("RotationInterval"); field.IsValid() && field.CanInterface() {
				if val, ok := field.Interface().(string); ok {
					rotationIntervalStr = val
				}
			}
		}
		
		if maxTotalSizeMB < 0 {
			maxTotalSizeMB = 0 // 0 means unlimited
		}

		// Determine rotation strategy
		rotationStrategy := RotationStrategy(strings.ToLower(rotationStrategyStr))
		if rotationStrategy == "" {
			rotationStrategy = RotationStrategySize // default
		}
		if rotationStrategy != RotationStrategySize && 
		   rotationStrategy != RotationStrategyTime && 
		   rotationStrategy != RotationStrategyBoth {
			rotationStrategy = RotationStrategySize // fallback to size
		}

		rotationInterval := RotationInterval(strings.ToLower(rotationIntervalStr))
		if rotationInterval == "" {
			rotationInterval = RotationIntervalDaily // default
		}
		if rotationInterval != RotationIntervalHourly && 
		   rotationInterval != RotationIntervalDaily && 
		   rotationInterval != RotationIntervalWeekly {
			rotationInterval = RotationIntervalDaily // fallback to daily
		}

		var fileWriter io.Writer
		
		// Use TimeRotationWriter if time-based rotation is needed
		if rotationStrategy == RotationStrategyTime || rotationStrategy == RotationStrategyBoth {
			fileWriter = NewTimeRotationWriter(
				logConfig.GetFilePath(),
				ms, mb, ma, logConfig.GetCompress(),
				rotationStrategy,
				rotationInterval,
				maxTotalSizeMB,
			)
		} else {
			// Use standard lumberjack for size-only rotation
			fileWriter = &lumberjack.Logger{
				Filename:   logConfig.GetFilePath(),
				MaxSize:    ms, // MB
				MaxBackups: mb, // files
				MaxAge:     ma, // days
				Compress:   logConfig.GetCompress(),
			}
		}

		// For file output, always use JSON format (structured logging)
		// Format configuration mainly affects console output
		// This ensures log files remain parseable and searchable
		
		// Get performance configuration with defaults
		batchSize := 64 * 1024 // 64KB default
		batchFlushInterval := 100 * time.Millisecond
		asyncQueueSize := 2000 // default queue size
		enableDynamicAdjust := false // default: disabled for stability
		
		// Try to get performance config using reflection
		perfConfigVal := reflect.ValueOf(&logConfig).Elem()
		if perfConfigVal.IsValid() {
			if perfField := perfConfigVal.FieldByName("Performance"); perfField.IsValid() && perfField.CanInterface() {
				perfPtr := perfField.Interface()
				if perfPtr != nil {
					perfReflect := reflect.ValueOf(perfPtr).Elem()
					if perfReflect.IsValid() {
						if batchSizeVal := perfReflect.FieldByName("BatchSizeBytes"); batchSizeVal.IsValid() && batchSizeVal.CanInterface() {
							if bs, ok := batchSizeVal.Interface().(int32); ok && bs > 0 {
								batchSize = int(bs)
							}
						}
						if flushIntervalVal := perfReflect.FieldByName("BatchFlushIntervalMs"); flushIntervalVal.IsValid() && flushIntervalVal.CanInterface() {
							if fi, ok := flushIntervalVal.Interface().(int32); ok && fi > 0 {
								batchFlushInterval = time.Duration(fi) * time.Millisecond
							}
						}
						if queueSizeVal := perfReflect.FieldByName("AsyncQueueSize"); queueSizeVal.IsValid() && queueSizeVal.CanInterface() {
							if qs, ok := queueSizeVal.Interface().(int32); ok && qs > 0 {
								asyncQueueSize = int(qs)
							}
						}
					}
				}
			}
		}
		
		// Apply batch writing optimization
		batchWriter := NewBatchWriter(fileWriter, batchSize, batchFlushInterval)
		
		// Use async writer for file output to improve performance
		// Enable dynamic adjustment for production environments (can be configured)
		asyncFile := NewAsyncLogWriter(batchWriter, asyncQueueSize, enableDynamicAdjust)
		asyncWriters["file"] = asyncFile
		writers = append(writers, asyncFile)
	}

	// If no output is configured, default to optimized console output
	if len(writers) == 0 {
		consoleWriter := NewOptimizedConsoleWriter(os.Stdout)
		bufferedConsole := NewBufferedWriter(consoleWriter, 32*1024)
		bufferedWriters["console"] = bufferedConsole
		writers = append(writers, bufferedConsole)
	}

	// Create multi-output writer
	output := zerolog.MultiLevelWriter(writers...)

	// Set global time format and timezone with optimizations
	zerolog.TimeFieldFormat = "15:04:05.000" // Shorter format for better performance
	// timezone from log config (prefer scanned field)
	if tz := strings.TrimSpace(logConfig.GetTimezone()); tz != "" {
		setTimezoneByName(tz)
	} else {
		setTimezoneByName("Local")
	}
	// Timestamp function reads current timezone atomically
	zerolog.TimestampFunc = func() time.Time { return time.Now().In(getTZLoc()) }

	// Set global log level (unified via applyLevel)
	logLevel := strings.ToLower(logConfig.GetLevel())
	var initLvl log.Level
	switch logLevel {
	case "debug":
		initLvl = log.LevelDebug
	case "info":
		initLvl = log.LevelInfo
	case "warn":
		initLvl = log.LevelWarn
	case "error":
		initLvl = log.LevelError
	default:
		initLvl = log.LevelInfo
	}
	applyLevel(initLvl)

	// callerSkip is configurable
	callerSkipCurrent = callerSkipDefault
	if v := logConfig.GetCallerSkip(); v > 0 {
		callerSkipCurrent = int(v)
	}

	// apply stack & sampling initial config
	applyStackFromConfig(&logConfig)
	applySamplingFromConfig(&logConfig)

	// Initialize underlying logger with zeroLogger and performance optimizations
	z := zerolog.New(output).With().Timestamp().Logger()

	// Store service metadata for rebuilds
	serviceName = name
	serviceHost = host
	serviceVersion = version

	// Initialize the main logger with level filter and default fields
	baseAdapter = zeroLogLogger{z}
	filtered := log.NewFilter(baseAdapter, log.FilterLevel(kratosMinLevel))
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

	// Initialize proxy logger inner for Kratos app consumption
	GetProxyLogger() // ensure pLogger initialized
	if pLogger != nil {
		pLogger.inner.Store(logger)
	}

	// Initialize stop channels for background goroutines
	monitorStopCh = make(chan struct{})
	configWatchStopCh = make(chan struct{})

	// Start performance monitoring goroutine
	go monitorLogPerformance()

	// Mark logger as initialized
	loggerInitialized.Store(true)

	// Banner display has been decoupled from logger initialization (see app/banner)

	// First try Watch mechanism based on configuration source (e.g., local files, Polaris, etc. may support)
	apply := func(nc *lconf.Log) {
		// timezone update (use nc field)
		if tz := strings.TrimSpace(nc.GetTimezone()); tz != "" {
			setTimezoneByName(tz)
		} else {
			setTimezoneByName("Local")
		}

		// level update
		switch strings.ToLower(nc.GetLevel()) {
		case "debug":
			applyLevel(log.LevelDebug)
		case "info":
			applyLevel(log.LevelInfo)
		case "warn":
			applyLevel(log.LevelWarn)
		case "error":
			applyLevel(log.LevelError)
		}

		// caller skip update
		callerSkipCurrent = callerSkipDefault
		if v := nc.GetCallerSkip(); v > 0 {
			callerSkipCurrent = int(v)
		}

		// stack & sampling update
		applyStackFromConfig(nc)
		applySamplingFromConfig(nc)

		// Rebuild logger to reflect caller skip and any changes
		rebuildLogger()
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
			// Generate signature
			signature := func(c *lconf.Log) string {
				if c == nil {
					return ""
				}
				var sb strings.Builder
				// base fields
				fmt.Fprintf(&sb, "lvl=%s;tz=%s;caller=%d;", strings.ToLower(c.GetLevel()), getTZName(), c.GetCallerSkip())
				// stack fields
				if s := c.GetStack(); s != nil {
					fmt.Fprintf(&sb, "stack_en=%t;stack_lvl=%s;stack_skip=%d;stack_max=%d;", s.GetEnable(), strings.ToLower(s.GetLevel()), s.GetSkip(), s.GetMaxFrames())
					if fps := s.GetFilterPrefixes(); len(fps) > 0 {
						sb.WriteString("stack_fp=")
						sb.WriteString(strings.Join(fps, ","))
						sb.WriteString(";")
					}
				}
				// sampling fields
				if sm := c.GetSampling(); sm != nil {
					fmt.Fprintf(&sb, "smp_en=%t;smp_ir=%.3f;smp_dr=%.3f;smp_i_max=%d;smp_d_max=%d;", sm.GetEnable(), sm.GetInfoRatio(), sm.GetDebugRatio(), sm.GetMaxInfoPerSec(), sm.GetMaxDebugPerSec())
				}
				return sb.String()
			}

			prevSig := signature(&logConfig)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-configWatchStopCh:
					return
				case <-ticker.C:
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
			}
		}()
	}

	// Log successful initialization
	lHelper.Info("lynx application logging component initialized successfully with performance optimizations")

	return nil
}

// monitorLogPerformance monitors logging performance and reports metrics
func monitorLogPerformance() {
	ticker := time.NewTicker(30 * time.Second) // Report every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-monitorStopCh:
			return
		case <-ticker.C:
			if !metricsEnabled {
				continue
			}

			writersMu.RLock()
			
			// Collect metrics from buffered writers
			for name, bw := range bufferedWriters {
				metrics := bw.GetMetrics()
				if metrics.TotalLogs > 0 {
					fmt.Printf("[lynx-log-perf] BufferedWriter[%s]: logs=%d, avg_write=%v, buffer_util=%.1f%%, flushes=%d, errors=%d\n",
						name, metrics.TotalLogs, metrics.AvgWriteTime, metrics.BufferUtilization, metrics.FlushCount, metrics.ErrorCount)
				}
			}

			// Collect metrics from async writers
			for name, aw := range asyncWriters {
				metrics := aw.GetMetrics()
				if metrics.TotalLogs > 0 {
					fmt.Printf("[lynx-log-perf] AsyncWriter[%s]: logs=%d, dropped=%d, avg_write=%v, queue_util=%.1f%%, errors=%d\n",
						name, metrics.TotalLogs, metrics.DroppedLogs, metrics.AvgWriteTime, metrics.BufferUtilization, metrics.ErrorCount)
				}
			}
			
			writersMu.RUnlock()
		}
	}
}

// GetLogPerformanceMetrics returns aggregated performance metrics
func GetLogPerformanceMetrics() map[string]LogPerformanceMetrics {
	writersMu.RLock()
	defer writersMu.RUnlock()

	metrics := make(map[string]LogPerformanceMetrics)

	for name, bw := range bufferedWriters {
		metrics["buffered_"+name] = bw.GetMetrics()
	}

	for name, aw := range asyncWriters {
		metrics["async_"+name] = aw.GetMetrics()
	}

	return metrics
}

// ResetLogPerformanceMetrics resets all performance metrics
func ResetLogPerformanceMetrics() {
	writersMu.RLock()
	defer writersMu.RUnlock()

	for _, bw := range bufferedWriters {
		bw.ResetMetrics()
	}

	for range asyncWriters {
		// Async writers don't have reset method, but we can note the reset time
		// This would need to be implemented in the AsyncLogWriter if needed
	}
}

// EnablePerformanceMonitoring enables or disables performance monitoring
func EnablePerformanceMonitoring(enabled bool) {
	metricsEnabled = enabled
}

// CleanupLoggers properly closes all writers and cleans up resources
func CleanupLoggers() {
	// Mark logger as not initialized
	loggerInitialized.Store(false)

	// Stop background goroutines safely
	if monitorStopCh != nil {
		select {
		case <-monitorStopCh:
			// already closed
		default:
			close(monitorStopCh)
		}
		monitorStopCh = nil
	}
	if configWatchStopCh != nil {
		select {
		case <-configWatchStopCh:
			// already closed
		default:
			close(configWatchStopCh)
		}
		configWatchStopCh = nil
	}

	// Wait for goroutines to stop with timeout
	done := make(chan struct{})
	go func() {
		// Give goroutines time to see stop signals
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()
	
	select {
	case <-done:
		// Goroutines should have stopped
	case <-time.After(2 * time.Second):
		// Timeout: continue anyway to prevent deadlock
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] CleanupLoggers timeout waiting for goroutines\n")
	}

	writersMu.Lock()
	defer writersMu.Unlock()

	// Close all writers with timeout protection
	closeDone := make(chan struct{})
	go func() {
		for _, bw := range bufferedWriters {
			if err := bw.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "[lynx-log-error] Failed to close buffered writer: %v\n", err)
			}
		}

		for _, aw := range asyncWriters {
			if err := aw.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "[lynx-log-error] Failed to close async writer: %v\n", err)
			}
		}
		close(closeDone)
	}()
	
	select {
	case <-closeDone:
		// All writers closed successfully
	case <-time.After(5 * time.Second):
		// Timeout: continue anyway
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] CleanupLoggers timeout closing writers\n")
	}

	bufferedWriters = make(map[string]*BufferedWriter)
	asyncWriters = make(map[string]*AsyncLogWriter)
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
