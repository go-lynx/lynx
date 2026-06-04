package log

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// ConsoleWriterConfig configures console output format
type ConsoleWriterConfig struct {
	Format      string // "json", "text", "pretty"
	ColorOutput bool   // Enable color output
	NoColor     bool   // Disable color output
	TimeFormat  string // Time format string
}

// NewConsoleWriter returns a writer for the given format, defaulting to raw
// JSON on os.Stdout for any unrecognized format.
func NewConsoleWriter(config ConsoleWriterConfig) io.Writer {
	switch config.Format {
	case "pretty":
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: getTimeFormat(config.TimeFormat),
			NoColor:    config.NoColor || !config.ColorOutput,
		}
	case "text":
		return &textWriter{
			out:        os.Stdout,
			timeFormat: getTimeFormat(config.TimeFormat),
			color:      config.ColorOutput && !config.NoColor,
		}
	default:
		return os.Stdout
	}
}

// getTimeFormat returns custom if non-empty, otherwise time.RFC3339.
func getTimeFormat(custom string) string {
	if custom != "" {
		return custom
	}
	return time.RFC3339
}

type textWriter struct {
	out        io.Writer
	timeFormat string
	color      bool
}

// Write currently passes bytes through unchanged; text formatting of the
// underlying JSON is not yet implemented.
func (tw *textWriter) Write(p []byte) (int, error) {
	return tw.out.Write(p)
}
