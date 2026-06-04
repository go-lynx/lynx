package log

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
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

var (
	stconf atomic.Value // *stackCfg
	// stackCache memoizes traces by caller location so repeated errors at the
	// same site avoid re-walking the stack. Keyed by "file:line:pc".
	stackCache sync.Map
)

func init() {
	stconf.Store(&stackCfg{
		enabled:   true,
		skip:      6,
		maxFrames: 32,
		minLevel:  log.LevelError,
		filterPrefixes: []string{
			"github.com/go-kratos/kratos",
			"github.com/rs/zerolog",
			"github.com/go-lynx/lynx/log",
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
	// Copy the prefixes so later caller mutations can't affect stored config.
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

// captureStack returns a stack trace of up to maxFrames frames, skipping the
// first 'skip' frames and any frame whose Function or File matches a configured
// filter prefix. Each line is "FuncName file:line". Returns "" when stack
// capture is disabled or no frames remain. Results are cached per caller site.
func captureStack() string {
	cfg := getStackConfig()
	if !cfg.enabled {
		return ""
	}

	// Skip 2 frames (this function + the adapter) to key the cache on the real caller.
	var callerKey string
	if pc, file, line, ok := runtime.Caller(2); ok {
		callerKey = fmt.Sprintf("%s:%d:%d", file, line, pc)
		if cached, ok := stackCache.Load(callerKey); ok {
			if stack, ok := cached.(string); ok && stack != "" {
				return stack
			}
		}
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
		if fr.Function != "" || fr.File != "" {
			if !hasAnyPrefix(fr.Function, cfg.filterPrefixes) && !hasAnyPrefix(fr.File, cfg.filterPrefixes) {
				fmt.Fprintf(&b, "%s %s:%d\n", fr.Function, fr.File, fr.Line)
			}
		}
		if !more {
			break
		}
	}
	stack := b.String()

	// Cache is unbounded; acceptable because keys are limited to distinct caller
	// sites. Consider LRU eviction if call sites become unbounded.
	if callerKey != "" {
		stackCache.Store(callerKey, stack)
	}

	return stack
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
