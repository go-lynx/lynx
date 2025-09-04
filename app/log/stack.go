package log

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

// stack trace configuration (can be overridden by config)
var (
	stackEnabled        = true
	stackSkip           = 6
	stackMaxFrames      = 32
	stackMinLevel       = log.LevelError
	stackFilterPrefixes = []string{
		"github.com/go-kratos/kratos",
		"github.com/rs/zerolog",
		"github.com/go-lynx/lynx/app/log",
	}
)

// captureStack collects a simple stack trace string with up to maxFrames frames, skipping 'skip' frames.
// Frames with Function or File starting with any of 'filterPrefixes' will be skipped.
// Format: FuncName file:line per line, joined by '\n'.
func captureStack(skip int, maxFrames int, filterPrefixes []string) string {
	if skip < 0 {
		skip = 0
	}
	if maxFrames <= 0 {
		maxFrames = 16
	}
	pcs := make([]uintptr, maxFrames)
	n := runtime.Callers(skip, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		fr, more := frames.Next()
		// Avoid empty frames
		if fr.Function != "" || fr.File != "" {
			// filter internal prefixes
			if !hasAnyPrefix(fr.Function, filterPrefixes) && !hasAnyPrefix(fr.File, filterPrefixes) {
				fmt.Fprintf(&b, "%s %s:%d\n", fr.Function, fr.File, fr.Line)
			}
		}
		if !more {
			break
		}
	}
	return b.String()
}

// hasAnyPrefix reports whether s starts with any prefix in the list.
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
