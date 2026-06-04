// Package log - zerolog adapter for Kratos log.Logger
package log

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/rs/zerolog"
)

type zeroLogLogger struct {
	logger zerolog.Logger
}

// Log implements the log.Logger interface.
// It converts Kratos log levels to zerolog levels and handles structured logging.
func (l zeroLogLogger) Log(level log.Level, keyvals ...any) error {
	if !allowLog(level) {
		return nil
	}
	// Tolerate an odd keyvals count by padding with a placeholder value.
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "BAD_VALUE")
	}

	var event *zerolog.Event
	switch level {
	case log.LevelDebug:
		event = l.logger.Debug()
	case log.LevelInfo:
		event = l.logger.Info()
	case log.LevelWarn:
		event = l.logger.Warn()
	case log.LevelError:
		event = l.logger.Error()
	case log.LevelFatal:
		event = l.logger.Fatal()
	default:
		// Unknown levels are logged as warnings with the original level attached.
		event = l.logger.Warn().Interface("original_level", level)
	}

	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("BAD_KEY_%d", i)
			event = event.Interface("original_key", keyvals[i])
		}

		val := keyvals[i+1]

		// "msg" becomes the log message rather than a field.
		if key == "msg" {
			if str, ok := val.(string); ok {
				msg = str
			} else {
				msg = fmt.Sprint(val)
			}
			continue
		}

		// Record error values via Err so zerolog formats them consistently.
		if key == "err" || key == "error" {
			if e, ok := val.(error); ok {
				event = event.Err(e)
				continue
			}
		}

		event = event.Interface(key, val)
	}

	// Attach a stack trace when enabled and the level meets the configured threshold.
	if sc := getStackConfig(); sc != nil && sc.enabled && level >= sc.minLevel {
		if stack := captureStack(); stack != "" {
			event = event.Str("stack", stack)
		}
	}

	event.Msg(msg)
	return nil
}
