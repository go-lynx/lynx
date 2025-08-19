package base

import (
	"fmt"
	"os"
	"strings"
)

// log levels: error < warn < info < debug
const (
	levelError = 1
	levelWarn  = 2
	levelInfo  = 3
	levelDebug = 4
)

func currentLevel() int {
	if os.Getenv("LYNX_QUIET") == "1" {
		return levelError
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LYNX_LOG_LEVEL"))) {
	case "error":
		return levelError
	case "warn", "warning":
		return levelWarn
	case "debug":
		return levelDebug
	default:
		return levelInfo
	}
}

func allow(level int) bool {
	return level <= currentLevel()
}

func Debugf(format string, a ...any) {
	if allow(levelDebug) {
		fmt.Fprintf(os.Stdout, format, a...)
	}
}

func Infof(format string, a ...any) {
	if allow(levelInfo) {
		fmt.Fprintf(os.Stdout, format, a...)
	}
}

func Warnf(format string, a ...any) {
	if allow(levelWarn) {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func Errorf(format string, a ...any) {
	if allow(levelError) {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}
