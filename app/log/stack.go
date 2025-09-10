package log

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/go-kratos/kratos/v2/log"
)

// stackCfg holds runtime-configurable stack trace settings.
type stackCfg struct {
	enabled        bool
	skip           int
	maxFrames      int
	minLevel       log.Level
	filterPrefixes []string
}

var stconf atomic.Value // *stackCfg

func init() {
	// defaults equivalent to previous constants
	stconf.Store(&stackCfg{
		enabled:   true,
		skip:      6,
		maxFrames: 32,
		minLevel:  log.LevelError,
		filterPrefixes: []string{
			"github.com/go-kratos/kratos",
			"github.com/rs/zerolog",
			"github.com/go-lynx/lynx/app/log",
		},
	})
}

func getStackConfig() *stackCfg {
	if v := stconf.Load(); v != nil {
		if c, ok := v.(*stackCfg); ok && c != nil {
			return c
		}
	}
	return &stackCfg{enabled: false}
}

func setStackConfig(enabled bool, minLevel log.Level, skip, maxFrames int, filterPrefixes []string) {
	if skip < 0 {
		skip = 0
	}
	if maxFrames <= 0 {
		maxFrames = 16
	}
	// shallow copy of prefixes slice to avoid external mutation
	var fp []string
	if len(filterPrefixes) > 0 {
		fp = append(fp, filterPrefixes...)
	}
	stconf.Store(&stackCfg{
		enabled:        enabled,
		skip:           skip,
		maxFrames:      maxFrames,
		minLevel:       minLevel,
		filterPrefixes: fp,
	})
}

// captureStack collects a simple stack trace string with up to maxFrames frames, skipping 'skip' frames.
// Frames with Function or File starting with any of 'filterPrefixes' will be skipped.
// Format: FuncName file:line per line, joined by '\n'.
func captureStack() string {
	cfg := getStackConfig()
	if !cfg.enabled {
		return ""
	}
	pcs := make([]uintptr, cfg.maxFrames)
	n := runtime.Callers(cfg.skip, pcs)
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
			if !hasAnyPrefix(fr.Function, cfg.filterPrefixes) && !hasAnyPrefix(fr.File, cfg.filterPrefixes) {
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
