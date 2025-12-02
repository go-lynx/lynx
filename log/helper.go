// Package log provides a unified logging interface for the Lynx framework.
// It wraps the Kratos logging system and provides convenient methods for different log levels.
package log

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Level represents the logging level.
type Level int32

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in production.
	DebugLevel Level = iota
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	ErrorLevel
	// FatalLevel logs are particularly important errors. In development the logger panics.
	FatalLevel
)

var (
	// Logger is the primary logging interface.
	// Provides structured logging capabilities for the application.
	Logger log.Logger

	// LHelper is a convenience wrapper around logger.
	// Provides simplified logging methods with predefined fields.
	LHelper log.Helper

	// helperStore stores *log.Helper atomically for safe hot-reload updates.
	helperStore atomic.Value // of *log.Helper
)

// SetLevel sets the global logging level.
func SetLevel(level Level) {
	// map public Level to Kratos level, apply, and rebuild logger
	var lvl log.Level
	switch level {
	case DebugLevel:
		lvl = log.LevelDebug
	case InfoLevel:
		lvl = log.LevelInfo
	case WarnLevel:
		lvl = log.LevelWarn
	case ErrorLevel, FatalLevel:
		lvl = log.LevelError
	default:
		lvl = log.LevelInfo
	}
	applyLevel(lvl)
	rebuildLogger()
}

// GetLevel returns the current global logging level.
func GetLevel() Level {
	// reflect current Kratos filter minimal level
	switch kratosMinLevel {
	case log.LevelDebug:
		return DebugLevel
	case log.LevelInfo:
		return InfoLevel
	case log.LevelWarn:
		return WarnLevel
	case log.LevelError:
		return ErrorLevel
	case log.LevelFatal:
		return FatalLevel
	default:
		return InfoLevel
	}
}

// fallbackLogger provides a simple fallback logger when main logger is not initialized
type fallbackLogger struct{}

func (f *fallbackLogger) logPlain(level, msg string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	formatted := fmt.Sprintf("[%s] [%s] [lynx-log-fallback] %s\n", timestamp, level, msg)
	os.Stderr.WriteString(formatted)
}

func (f *fallbackLogger) logFormat(level, format string, args ...any) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	formatted := fmt.Sprintf("[%s] [%s] [lynx-log-fallback] %s\n", timestamp, level, msg)
	os.Stderr.WriteString(formatted)
}

// helper returns the application's log helper instance.
// The helper provides simplified logging methods.
// If logger is not initialized, returns nil and logs will use fallback.
func helper() *log.Helper {
	if v := helperStore.Load(); v != nil {
		if h, ok := v.(*log.Helper); ok && h != nil {
			return h
		}
	}
	// Check if LHelper is properly initialized before returning
	if Logger != nil && loggerInitialized.Load() {
		return &LHelper
	}
	// Return nil if logger is not initialized - will use fallback in logging functions
	return nil
}

// fallbackLog provides fallback logging when logger is not initialized
var fallback = &fallbackLogger{}

// Debug uses the log helper to record debug-level log information.
func Debug(a ...any) {
	if h := helper(); h != nil {
		h.Debug(a...)
	} else {
		fallback.logPlain("DEBUG", fmt.Sprint(a...))
	}
}

// DebugCtx uses the log helper to record debug-level log information with context.
func DebugCtx(ctx context.Context, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Debug(a...)
	}
}

func Debugf(format string, a ...any) {
	if h := helper(); h != nil {
		h.Debugf(format, a...)
	} else {
		fallback.logFormat("DEBUG", format, a...)
	}
}

func DebugfCtx(ctx context.Context, format string, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Debugf(format, a...)
	}
}

func Debugw(keyvals ...any) {
	if h := helper(); h != nil {
		h.Debugw(keyvals...)
	}
}

func DebugwCtx(ctx context.Context, keyvals ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Debugw(keyvals...)
	}
}

func Info(a ...any) {
	if h := helper(); h != nil {
		h.Info(a...)
	} else {
		fallback.logPlain("INFO", fmt.Sprint(a...))
	}
}

func InfoCtx(ctx context.Context, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Info(a...)
	}
}

func Infof(format string, a ...any) {
	if h := helper(); h != nil {
		h.Infof(format, a...)
	} else {
		fallback.logFormat("INFO", format, a...)
	}
}

func InfofCtx(ctx context.Context, format string, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Infof(format, a...)
	}
}

func Infow(keyvals ...any) {
	if h := helper(); h != nil {
		h.Infow(keyvals...)
	}
}

func InfowCtx(ctx context.Context, keyvals ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Infow(keyvals...)
	}
}

func Warn(a ...any) {
	if h := helper(); h != nil {
		h.Warn(a...)
	} else {
		fallback.logPlain("WARN", fmt.Sprint(a...))
	}
}

func WarnCtx(ctx context.Context, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Warn(a...)
	}
}

func Warnf(format string, a ...any) {
	if h := helper(); h != nil {
		h.Warnf(format, a...)
	} else {
		fallback.logFormat("WARN", format, a...)
	}
}

func WarnfCtx(ctx context.Context, format string, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Warnf(format, a...)
	}
}

func Warnw(keyvals ...any) {
	if h := helper(); h != nil {
		h.Warnw(keyvals...)
	}
}

func WarnwCtx(ctx context.Context, keyvals ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Warnw(keyvals...)
	}
}

func Error(a ...any) {
	if h := helper(); h != nil {
		h.Error(a...)
	} else {
		fallback.logPlain("ERROR", fmt.Sprint(a...))
	}
}

func ErrorCtx(ctx context.Context, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Error(a...)
	}
}

func Errorf(format string, a ...any) {
	if h := helper(); h != nil {
		h.Errorf(format, a...)
	} else {
		fallback.logFormat("ERROR", format, a...)
	}
}

func ErrorfCtx(ctx context.Context, format string, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Errorf(format, a...)
	}
}

func Errorw(keyvals ...any) {
	if h := helper(); h != nil {
		h.Errorw(keyvals...)
	}
}

func ErrorwCtx(ctx context.Context, keyvals ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Errorw(keyvals...)
	}
}

// Fatal logs a message at FatalLevel.
func Fatal(a ...any) {
	// avoid double exit: zerolog's Fatal will exit once
	if h := helper(); h != nil {
		h.Fatal("msg", fmt.Sprint(a...))
	} else {
		fallback.logPlain("FATAL", fmt.Sprint(a...))
		os.Exit(1)
	}
}

// FatalCtx logs a message at FatalLevel with context.
func FatalCtx(ctx context.Context, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Fatal("msg", fmt.Sprint(a...))
	}
}

// Fatalf logs a formatted message at FatalLevel.
func Fatalf(format string, a ...any) {
	if h := helper(); h != nil {
		h.Fatal("msg", fmt.Sprintf(format, a...))
	} else {
		fallback.logFormat("FATAL", format, a...)
		os.Exit(1)
	}
}

// FatalfCtx logs a formatted message at FatalLevel with context.
func FatalfCtx(ctx context.Context, format string, a ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Fatal("msg", fmt.Sprintf(format, a...))
	}
}

// Fatalw logs key-value pairs at FatalLevel.
func Fatalw(keyvals ...any) {
	if h := helper(); h != nil {
		h.Fatal(keyvals...)
	}
}

// FatalwCtx logs key-value pairs at FatalLevel with context.
func FatalwCtx(ctx context.Context, keyvals ...any) {
	if h := helper(); h != nil {
		h.WithContext(ctx).Fatal(keyvals...)
	}
}
