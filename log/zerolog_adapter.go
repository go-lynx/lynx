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
func (l zeroLogLogger) Log(level log.Level, keyvals ...interface{}) error {
	// sampling / rate limit
	if !allowLog(level) {
		return nil
	}
	// Tolerate odd number of keyvals by appending a placeholder value
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "BAD_VALUE")
	}

	// Map Kratos log levels to zerolog levels
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
		// Log unknown levels as warnings and include the original level
		event = l.logger.Warn().Interface("original_level", level)
	}

	// Add structured key-value fields
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("BAD_KEY_%d", i)
			event = event.Interface("original_key", keyvals[i])
		}

		val := keyvals[i+1]

		// Special handling for message field
		if key == "msg" {
			if str, ok := val.(string); ok {
				msg = str
			} else {
				msg = fmt.Sprint(val)
			}
			continue
		}

		// Error value handling
		if key == "err" || key == "error" {
			if e, ok := val.(error); ok {
				event = event.Err(e)
				continue
			}
		}

		// Add the field to the event
		event = event.Interface(key, val)
	}

	// Attach stack trace if enabled and level reaches threshold (runtime-configurable)
	if sc := getStackConfig(); sc != nil && sc.enabled && level >= sc.minLevel {
		if stack := captureStack(); stack != "" {
			event = event.Str("stack", stack)
		}
	}

	// Output the log entry
	event.Msg(msg)
	return nil
}
