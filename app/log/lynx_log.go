package log

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"os"
)

var (
	// Logger is the primary logging interface.
	// Provides structured logging capabilities for the application.
	// logger 是主要的日志记录接口。
	// 为应用程序提供结构化日志记录功能。
	Logger log.Logger

	// LHelper logHelper is a convenience wrapper around logger.
	// Provides simplified logging methods with predefined fields.
	// logHelper 是 logger 的便捷包装器。
	// 提供带有预定义字段的简化日志记录方法。
	LHelper log.Helper
)

// helper 函数用于获取应用程序的日志辅助器实例。
// 该辅助器提供了简化的日志记录方法。
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

func Fatal(a ...any) {
	helper().Fatal(a...)
	os.Exit(1)
}

func FatalCtx(ctx context.Context, a ...any) {
	helper().WithContext(ctx).Fatal(a...)
	os.Exit(1)
}

func Fatalf(format string, a ...any) {
	helper().Fatalf(format, a...)
	os.Exit(1)
}

func FatalfCtx(ctx context.Context, format string, a ...any) {
	helper().WithContext(ctx).Fatalf(format, a...)
	os.Exit(1)
}

func Fatalw(keyvals ...any) {
	helper().Fatalw(keyvals...)
	os.Exit(1)
}

func FatalwCtx(ctx context.Context, keyvals ...any) {
	helper().WithContext(ctx).Fatalw(keyvals...)
	os.Exit(1)
}
