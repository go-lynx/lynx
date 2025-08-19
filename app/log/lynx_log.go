// Package log provides a unified logging interface for the Lynx framework.
// It wraps the Kratos logging system and provides convenient methods for different log levels.
package log

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/rs/zerolog"
)

// LogLevel represents the logging level.
type LogLevel int32

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in production.
	DebugLevel LogLevel = iota
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
)

// SetLevel sets the global logging level.
func SetLevel(level LogLevel) {
	switch level {
	case DebugLevel:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case InfoLevel:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case WarnLevel:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case ErrorLevel:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case FatalLevel:
		// map to ErrorLevel for runtime logging; Fatal is per-entry behavior
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// GetLevel returns the current global logging level.
func GetLevel() LogLevel {
	switch zerolog.GlobalLevel() {
	case zerolog.DebugLevel:
		return DebugLevel
	case zerolog.InfoLevel:
		return InfoLevel
	case zerolog.WarnLevel:
		return WarnLevel
	case zerolog.ErrorLevel:
		return ErrorLevel
	case zerolog.FatalLevel:
		return FatalLevel
	default:
		return InfoLevel
	}
}

// helper returns the application's log helper instance.
// The helper provides simplified logging methods.
func helper() *log.Helper {
	return &LHelper
}

// Debug 使用日志辅助器记录调试级别的日志信息。
func Debug(a ...any) {
	helper().Debug(a...)
}

// DebugCtx 使用日志辅助器记录调试级别的日志信息，带上下文。
func DebugCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Debug(a...)
}

func Debugf(format string, a ...any) {
	helper().Debugf(format, a...)
}

func DebugfCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Debugf(format, a...)
}

func Debugw(keyvals ...any) {
	helper().Debugw(keyvals...)
}

func DebugwCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Debugw(keyvals...)
}

func Info(a ...any) {
	helper().Info(a...)
}

func InfoCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Info(a...)
}

func Infof(format string, a ...any) {
	helper().Infof(format, a...)
}

func InfofCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Infof(format, a...)
}

func Infow(keyvals ...any) {
	helper().Infow(keyvals...)
}

func InfowCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Infow(keyvals...)
}

func Warn(a ...any) {
	helper().Warn(a...)
}

func WarnCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Warn(a...)
}

func Warnf(format string, a ...any) {
	helper().Warnf(format, a...)
}

func WarnfCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Warnf(format, a...)
}

func Warnw(keyvals ...any) {
	helper().Warnw(keyvals...)
}

func WarnwCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Warnw(keyvals...)
}

func Error(a ...any) {
	helper().Error(a...)
}

func ErrorCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Error(a...)
}

func Errorf(format string, a ...any) {
	helper().Errorf(format, a...)
}

func ErrorfCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Errorf(format, a...)
}

func Errorw(keyvals ...any) {
	helper().Errorw(keyvals...)
}

func ErrorwCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Errorw(keyvals...)
}

// Fatal logs a message at FatalLevel.
func Fatal(a ...any) {
	// avoid double exit: zerolog's Fatal will exit once
	helper().Fatal("msg", fmt.Sprint(a...))
}

// FatalCtx logs a message at FatalLevel with context.
func FatalCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Fatal("msg", fmt.Sprint(a...))
}

// Fatalf logs a formatted message at FatalLevel.
func Fatalf(format string, a ...any) {
	helper().Fatal("msg", fmt.Sprintf(format, a...))
}

// FatalfCtx logs a formatted message at FatalLevel with context.
func FatalfCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Fatal("msg", fmt.Sprintf(format, a...))
}

// Fatalw logs key-value pairs at FatalLevel.
func Fatalw(keyvals ...any) {
	helper().Fatal(keyvals...)
}

// FatalwCtx logs key-value pairs at FatalLevel with context.
func FatalwCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Fatal(keyvals...)
}
