package envx

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Get reads a string env var; returns default if empty.
func Get(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// GetInt reads an int env var; returns default on parse failure.
func GetInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

// GetBool reads a bool env var; returns default on parse failure.
func GetBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// GetDuration reads a duration; supports time.ParseDuration and plain numbers (seconds).
func GetDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Duration(i) * time.Second
	}
	return def
}
