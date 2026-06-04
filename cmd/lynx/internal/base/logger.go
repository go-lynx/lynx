package base

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// log levels: error < warn < info < debug
const (
	levelError = 1
	levelWarn  = 2
	levelInfo  = 3
	levelDebug = 4
)

// currentLevel resolves the active log level from the environment. LYNX_QUIET=1
// forces error-only; otherwise LYNX_LOG_LEVEL selects the level, defaulting to info.
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

// allow reports whether a message at level should be emitted.
func allow(level int) bool {
	return level <= currentLevel()
}

// Debugf prints a debug-level message to stdout when debug logging is enabled.
func Debugf(format string, a ...any) {
	if allow(levelDebug) {
		_, err := fmt.Fprintf(os.Stdout, format, a...)
		if err != nil {
			return
		}
	}
}

// Infof prints an info-level message to stdout when info logging is enabled.
func Infof(format string, a ...any) {
	if allow(levelInfo) {
		_, err := fmt.Fprintf(os.Stdout, format, a...)
		if err != nil {
			return
		}
	}
}

// Warnf prints a warning level formatted message to stderr (in yellow) if warn
// level or higher is enabled. Color is applied centrally here, so callers pass
// plain text; color.YellowString respects NO_COLOR and non-TTY output.
func Warnf(format string, a ...any) {
	if allow(levelWarn) {
		_, _ = fmt.Fprint(os.Stderr, color.YellowString(format, a...))
	}
}

// Errorf prints an error level formatted message to stderr (in red) if error
// level or higher is enabled. Color is applied centrally here, so callers pass
// plain text; color.RedString respects NO_COLOR and non-TTY output.
func Errorf(format string, a ...any) {
	if allow(levelError) {
		_, _ = fmt.Fprint(os.Stderr, color.RedString(format, a...))
	}
}

// Successf prints an info-level success message to stdout in green. Use for
// positive confirmations to keep success styling consistent across commands.
func Successf(format string, a ...any) {
	if allow(levelInfo) {
		_, _ = fmt.Fprint(os.Stdout, color.GreenString(format, a...))
	}
}
