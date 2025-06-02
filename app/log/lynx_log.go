package log

import (
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
// 参数 a 是可变参数，代表要记录的日志内容。
func Debug(a ...any) {
	helper().Debug(a...)
}

// Debugf 使用日志辅助器记录格式化的调试级别的日志信息。
// 参数 format 是格式化字符串，参数 a 是可变参数，用于填充格式化字符串。
func Debugf(format string, a ...any) {
	helper().Debugf(format, a...)
}

// Debugw 使用日志辅助器记录带有关键值对的调试级别的日志信息。
// 参数 keyvals 是可变参数，代表要记录的键值对。
func Debugw(keyvals ...any) {
	helper().Debugw(keyvals...)
}

// Info 使用日志辅助器记录信息级别的日志信息。
// 参数 a 是可变参数，代表要记录的日志内容。
func Info(a ...any) {
	helper().Info(a...)
}

// Infof 使用日志辅助器记录格式化的信息级别的日志信息。
// 参数 format 是格式化字符串，参数 a 是可变参数，用于填充格式化字符串。
func Infof(format string, a ...any) {
	helper().Infof(format, a...)
}

// Infow 使用日志辅助器记录带有关键值对的信息级别的日志信息。
// 参数 keyvals 是可变参数，代表要记录的键值对。
func Infow(keyvals ...any) {
	helper().Infow(keyvals...)
}

// Warn 使用日志辅助器记录警告级别的日志信息。
// 参数 a 是可变参数，代表要记录的日志内容。
func Warn(a ...any) {
	helper().Warn(a...)
}

// Warnf 使用日志辅助器记录格式化的警告级别的日志信息。
// 参数 format 是格式化字符串，参数 a 是可变参数，用于填充格式化字符串。
func Warnf(format string, a ...any) {
	helper().Warnf(format, a...)
}

// Warnw 使用日志辅助器记录带有关键值对的警告级别的日志信息。
// 参数 keyvals 是可变参数，代表要记录的键值对。
func Warnw(keyvals ...any) {
	helper().Warnw(keyvals...)
}

// Error 使用日志辅助器记录错误级别的日志信息。
// 参数 a 是可变参数，代表要记录的日志内容。
func Error(a ...any) {
	helper().Error(a...)
}

// Errorf 使用日志辅助器记录格式化的错误级别的日志信息。
// 参数 format 是格式化字符串，参数 a 是可变参数，用于填充格式化字符串。
func Errorf(format string, a ...any) {
	helper().Errorf(format, a...)
}

// Errorw 使用日志辅助器记录带有关键值对的错误级别的日志信息。
// 参数 keyvals 是可变参数，代表要记录的键值对。
func Errorw(keyvals ...any) {
	helper().Errorw(keyvals...)
}

// Fatal 使用日志辅助器记录致命级别的日志信息，并在记录后退出程序。
// 参数 a 是可变参数，代表要记录的日志内容。
// 程序退出状态码为 1。
func Fatal(a ...any) {
	helper().Fatal(a...)
	os.Exit(1)
}

// Fatalf 使用日志辅助器记录格式化的致命级别的日志信息，并在记录后退出程序。
// 参数 format 是格式化字符串，参数 a 是可变参数，用于填充格式化字符串。
// 程序退出状态码为 1。
func Fatalf(format string, a ...any) {
	helper().Fatalf(format, a...)
	os.Exit(1)
}

// Fatalw 使用日志辅助器记录带有关键值对的致命级别的日志信息，并在记录后退出程序。
// 参数 keyvals 是可变参数，代表要记录的键值对。
// 程序退出状态码为 1。
func Fatalw(keyvals ...any) {
	helper().Fatalw(keyvals...)
	os.Exit(1)
}
