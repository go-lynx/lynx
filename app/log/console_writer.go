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

// NewConsoleWriter creates a console writer based on format
func NewConsoleWriter(config ConsoleWriterConfig) io.Writer {
	switch config.Format {
	case "pretty":
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: getTimeFormat(config.TimeFormat),
			NoColor:    config.NoColor || !config.ColorOutput,
		}
	case "text":
		// Simple text format writer
		return &textWriter{
			out:        os.Stdout,
			timeFormat: getTimeFormat(config.TimeFormat),
			color:      config.ColorOutput && !config.NoColor,
		}
	default:
		// JSON format (default)
		return os.Stdout
	}
}

// getTimeFormat returns time format string
func getTimeFormat(custom string) string {
	if custom != "" {
		return custom
	}
	return time.RFC3339
}

// textWriter provides simple text format output
type textWriter struct {
	out        io.Writer
	timeFormat string
	color      bool
}

func (tw *textWriter) Write(p []byte) (int, error) {
	// For text format, we need to parse JSON and format it
	// This is a simplified version - in production, you might want
	// to use a proper JSON parser
	return tw.out.Write(p)
}

