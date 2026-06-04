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
	"sync"
	"sync/atomic"
	"time"

	kconf "github.com/go-kratos/kratos/v2/config"
	lconf "github.com/go-lynx/lynx/log/conf"

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
	globalMetrics  *LogPerformanceMetrics
	metricsEnabled bool

	loggerLifecycleMu sync.Mutex

	// Logger initialization state
	loggerInitialized atomic.Bool
	monitorStopCh     chan struct{} // channel to stop performance monitor
	configWatchStopCh chan struct{} // channel to stop config watch goroutine
)

// proxyLogger forwards Log calls to an inner logger stored atomically.
type proxyLogger struct{ inner atomic.Value } // of log.Logger

func (p *proxyLogger) Log(level log.Level, keys ...any) error {
	if p == nil {
		return nil
	}
	if v := p.inner.Load(); v != nil {
		if l, ok := v.(log.Logger); ok && l != nil {
			return l.Log(level, keys...)
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
		return
	}
	setSamplingConfig(
		sm.GetEnable(),
		float64(sm.GetInfoRatio()),
		float64(sm.GetDebugRatio()),
		int(sm.GetMaxInfoPerSec()),
		int(sm.GetMaxDebugPerSec()),
	)
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

// InitLogger builds the logging system from the "lynx.log" section of cfg using
// the given service name, host, and version as default fields. It is safe to
// call again to reinitialize; a prior logger is cleaned up first. name, host,
// and cfg must be non-empty/non-nil. Configuration changes are applied at
// runtime via Watch, falling back to 2s polling when the source lacks Watch.
func InitLogger(name string, host string, version string, cfg kconf.Config) error {
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()

	if loggerInitialized.Load() {
		cleanupLoggersLocked()
	}

	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if cfg == nil {
		return fmt.Errorf("configuration instance cannot be nil")
	}

	bufferedWriters = make(map[string]*BufferedWriter)
	asyncWriters = make(map[string]*AsyncLogWriter)
	globalMetrics = &LogPerformanceMetrics{
		lastReset: time.Now(),
	}
	metricsEnabled = false

	// Use fmt here: the logger isn't initialized yet.
	fmt.Println("[lynx] initializing logging component with performance optimizations")

	var logConfig lconf.Log
	if err := cfg.Value("lynx.log").Scan(&logConfig); err != nil {
		logConfig = lconf.Log{
			Level:         "info",
			ConsoleOutput: true,
		}
	}
	metricsEnabled = performanceMonitorEnabled(cfg)

	var writers []io.Writer

	if logConfig.GetConsoleOutput() {
		formatType := "json"
		consoleColor := true
		if f := logConfig.GetFormat(); f != nil {
			if t := f.GetType(); t != "" {
				formatType = t
			}
			consoleColor = f.GetConsoleColor()
		}
		// Console may use a distinct format; otherwise it shares the file format.
		consoleFormat := formatType
		if f := logConfig.GetFormat(); f != nil {
			if cf := f.GetConsoleFormat(); cf != "" {
				consoleFormat = cf
			}
		}

		consoleWriter := NewConsoleWriter(ConsoleWriterConfig{
			Format:      consoleFormat,
			ColorOutput: consoleColor,
			NoColor:     !consoleColor,
			TimeFormat:  "15:04:05.000",
		})

		bufferedConsole := NewBufferedWriter(consoleWriter, 32*1024) // 32KB console buffer
		bufferedWriters["console"] = bufferedConsole
		writers = append(writers, bufferedConsole)
	}

	if logConfig.GetFilePath() != "" {
		logDir := filepath.Dir(logConfig.GetFilePath())
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %v", err)
		}

		ms := int(logConfig.GetMaxSizeMb())
		if ms <= 0 {
			ms = 100 // default MB
		}
		mb := int(logConfig.GetMaxBackups())
		if mb < 0 {
			mb = 0
		}
		ma := int(logConfig.GetMaxAgeDays())
		if ma < 0 {
			ma = 0 // 0 means no age-based removal
		}
		maxTotalSizeMB := int(logConfig.GetMaxTotalSizeMb())
		if maxTotalSizeMB < 0 {
			maxTotalSizeMB = 0 // 0 means unlimited
		}
		rotationStrategyStr := logConfig.GetRotationStrategy()
		rotationIntervalStr := logConfig.GetRotationInterval()

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

		if rotationStrategy == RotationStrategyTime || rotationStrategy == RotationStrategyBoth {
			fileWriter = NewTimeRotationWriter(
				logConfig.GetFilePath(),
				ms, mb, ma, logConfig.GetCompress(),
				rotationStrategy,
				rotationInterval,
				maxTotalSizeMB,
			)
		} else {
			// Size-only rotation uses lumberjack directly.
			fileWriter = &lumberjack.Logger{
				Filename:   logConfig.GetFilePath(),
				MaxSize:    ms, // MB
				MaxBackups: mb, // files
				MaxAge:     ma, // days
				Compress:   logConfig.GetCompress(),
			}
		}

		// Files are always JSON regardless of format config, so they stay
		// machine-parseable; format settings affect console output only.

		batchSize := 64 * 1024 // 64KB
		batchFlushInterval := 100 * time.Millisecond
		asyncQueueSize := 2000
		enableDynamicAdjust := false // off by default for stability
		if perf := logConfig.GetPerformance(); perf != nil {
			if bs := perf.GetBatchSizeBytes(); bs > 0 {
				batchSize = int(bs)
			}
			if fi := perf.GetBatchFlushIntervalMs(); fi > 0 {
				batchFlushInterval = time.Duration(fi) * time.Millisecond
			}
			if qs := perf.GetAsyncQueueSize(); qs > 0 {
				asyncQueueSize = int(qs)
			}
			enableDynamicAdjust = perf.GetEnableDynamicAdjust()
		}

		// File output is batched then made async so logging never blocks callers.
		batchWriter := NewBatchWriter(fileWriter, batchSize, batchFlushInterval)
		asyncFile := NewAsyncLogWriter(batchWriter, asyncQueueSize, enableDynamicAdjust)
		asyncWriters["file"] = asyncFile
		writers = append(writers, asyncFile)
	}

	// Fall back to console output when nothing else is configured.
	if len(writers) == 0 {
		consoleWriter := NewOptimizedConsoleWriter(os.Stdout)
		bufferedConsole := NewBufferedWriter(consoleWriter, 32*1024)
		bufferedWriters["console"] = bufferedConsole
		writers = append(writers, bufferedConsole)
	}

	output := zerolog.MultiLevelWriter(writers...)

	zerolog.TimeFieldFormat = "15:04:05.000"
	if tz := strings.TrimSpace(logConfig.GetTimezone()); tz != "" {
		setTimezoneByName(tz)
	} else {
		setTimezoneByName("Local")
	}
	// Resolve the timezone per call so hot-reload changes take effect.
	zerolog.TimestampFunc = func() time.Time { return time.Now().In(getTZLoc()) }

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

	callerSkipCurrent = callerSkipDefault
	if v := logConfig.GetCallerSkip(); v > 0 {
		callerSkipCurrent = int(v)
	}

	applyStackFromConfig(&logConfig)
	applySamplingFromConfig(&logConfig)

	z := zerolog.New(output).With().Timestamp().Logger()

	// Retained for rebuildLogger on hot reload.
	serviceName = name
	serviceHost = host
	serviceVersion = version

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

	if logger == nil {
		return fmt.Errorf("failed to create logger")
	}

	lHelper := log.NewHelper(logger)
	if lHelper == nil {
		return fmt.Errorf("failed to create logger lHelper")
	}

	Logger = logger
	LHelper = *lHelper
	// Stored atomically so hot reloads can swap the helper without a data race.
	helperStore.Store(lHelper)

	GetProxyLogger() // ensure pLogger is initialized for the Kratos app
	if pLogger != nil {
		pLogger.inner.Store(logger)
	}

	monitorStopCh = make(chan struct{})
	configWatchStopCh = make(chan struct{})
	monitorCh := monitorStopCh
	watchStopCh := configWatchStopCh

	// Start performance monitoring only when explicitly enabled. The monitor
	// writes periodic fmt output and should not run by default in idle services.
	if metricsEnabled {
		go monitorLogPerformance(monitorCh)
	}

	loggerInitialized.Store(true)

	// apply reloads timezone, level, caller skip, stack, and sampling from a new
	// config and rebuilds the logger. Shared by the Watch and polling paths.
	apply := func(nc *lconf.Log) {
		if tz := strings.TrimSpace(nc.GetTimezone()); tz != "" {
			setTimezoneByName(tz)
		} else {
			setTimezoneByName("Local")
		}

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

		callerSkipCurrent = callerSkipDefault
		if v := nc.GetCallerSkip(); v > 0 {
			callerSkipCurrent = int(v)
		}

		applyStackFromConfig(nc)
		applySamplingFromConfig(nc)

		rebuildLogger()
	}

	// Prefer the config source's Watch; fall back to polling if unsupported.
	if err := cfg.Watch("lynx.log", func(key string, v kconf.Value) {
		var nc lconf.Log
		if err := v.Scan(&nc); err != nil {
			return
		}
		apply(&nc)
	}); err != nil {
		// Watch unsupported: poll lynx.log every 2s and apply on change.
		go func() {
			// signature condenses the watched fields into a string so changes
			// can be detected with a cheap comparison.
			signature := func(c *lconf.Log) string {
				if c == nil {
					return ""
				}
				var sb strings.Builder
				fmt.Fprintf(&sb, "lvl=%s;tz=%s;caller=%d;", strings.ToLower(c.GetLevel()), getTZName(), c.GetCallerSkip())
				if s := c.GetStack(); s != nil {
					fmt.Fprintf(&sb, "stack_en=%t;stack_lvl=%s;stack_skip=%d;stack_max=%d;", s.GetEnable(), strings.ToLower(s.GetLevel()), s.GetSkip(), s.GetMaxFrames())
					if fps := s.GetFilterPrefixes(); len(fps) > 0 {
						sb.WriteString("stack_fp=")
						sb.WriteString(strings.Join(fps, ","))
						sb.WriteString(";")
					}
				}
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
				case <-watchStopCh:
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

func performanceMonitorEnabled(cfg kconf.Config) bool {
	var raw struct {
		Performance struct {
			Enabled        bool `json:"enabled"`
			MonitorEnabled bool `json:"monitor_enabled"`
			MetricsEnabled bool `json:"metrics_enabled"`
		} `json:"performance"`
	}

	if err := cfg.Value("lynx.log").Scan(&raw); err != nil {
		return false
	}
	return raw.Performance.Enabled || raw.Performance.MonitorEnabled || raw.Performance.MetricsEnabled
}

// monitorLogPerformance monitors logging performance and reports metrics
func monitorLogPerformance(stopCh <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second) // Report every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if !metricsEnabled {
				continue
			}

			writersMu.RLock()

			for name, bw := range bufferedWriters {
				metrics := bw.GetMetrics()
				if metrics.TotalLogs > 0 {
					fmt.Printf("[lynx-log-perf] BufferedWriter[%s]: logs=%d, avg_write=%v, buffer_util=%.1f%%, flushes=%d, errors=%d\n",
						name, metrics.TotalLogs, metrics.AvgWriteTime, metrics.BufferUtilization, metrics.FlushCount, metrics.ErrorCount)
				}
			}

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
	loggerLifecycleMu.Lock()
	defer loggerLifecycleMu.Unlock()
	cleanupLoggersLocked()
}

func cleanupLoggersLocked() {
	loggerInitialized.Store(false)

	// Signal background goroutines to stop, tolerating already-closed channels.
	if monitorStopCh != nil {
		select {
		case <-monitorStopCh:
		default:
			close(monitorStopCh)
		}
		monitorStopCh = nil
	}
	if configWatchStopCh != nil {
		select {
		case <-configWatchStopCh:
		default:
			close(configWatchStopCh)
		}
		configWatchStopCh = nil
	}

	// Give goroutines time to observe the stop signal, then proceed regardless
	// to avoid deadlock. Both durations are env-overridable.
	goroutineWaitTime := 50 * time.Millisecond
	if envWait := os.Getenv("LYNX_LOG_GOROUTINE_WAIT_TIME"); envWait != "" {
		if parsed, err := time.ParseDuration(envWait); err == nil {
			goroutineWaitTime = parsed
		}
	}
	goroutineTimeout := 2 * time.Second
	if envTimeout := os.Getenv("LYNX_LOG_GOROUTINE_TIMEOUT"); envTimeout != "" {
		if parsed, err := time.ParseDuration(envTimeout); err == nil {
			goroutineTimeout = parsed
		}
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(goroutineWaitTime)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(goroutineTimeout):
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] CleanupLoggers timeout waiting for goroutines after %v\n", goroutineTimeout)
	}

	writersMu.Lock()
	defer writersMu.Unlock()

	// Close writers under a timeout (env-overridable) so a stuck writer can't
	// hang shutdown.
	writerCloseTimeout := 5 * time.Second
	if envTimeout := os.Getenv("LYNX_LOG_WRITER_CLOSE_TIMEOUT"); envTimeout != "" {
		if parsed, err := time.ParseDuration(envTimeout); err == nil {
			writerCloseTimeout = parsed
		}
	}

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
	case <-time.After(writerCloseTimeout):
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] CleanupLoggers timeout closing writers after %v\n", writerCloseTimeout)
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

// trimFilePath keeps the last depth path components of file (e.g. depth=2 turns
// "/a/b/c/d.go" into "c/d.go"). Returns "unknown" for an empty path or depth<=0,
// and the whole path when it has fewer components than depth.
func trimFilePath(file string, depth int) string {
	if file == "" || depth <= 0 {
		return "unknown"
	}

	var slashPos []int
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' { // handle Unix and Windows separators
			slashPos = append(slashPos, i)
			if len(slashPos) == depth {
				break
			}
		}
	}

	if len(slashPos) == 0 {
		return file
	}

	start := slashPos[len(slashPos)-1] + 1
	return file[start:]
}
