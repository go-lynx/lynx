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

// currentLevel returns the current logging level based on environment variables.
// It checks LYNX_QUIET first (if set to "1", only errors are logged),
// then LYNX_LOG_LEVEL for specific level (error, warn, info, debug).
// Defaults to info level if neither is set.
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

// allow checks if the given log level should be output based on the current level.
// Returns true if the provided level is less than or equal to the current level.
func allow(level int) bool {
	return level <= currentLevel()
}

// Debugf prints a debug level formatted message to stdout if debug level is enabled.
// Takes a format string and variadic arguments similar to fmt.Printf.
func Debugf(format string, a ...any) {
	if allow(levelDebug) {
		_, err := fmt.Fprintf(os.Stdout, format, a...)
		if err != nil {
			return
		}
	}
}

// Infof prints an info level formatted message to stdout if info level or higher is enabled.
// Takes a format string and variadic arguments similar to fmt.Printf.
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
