package base

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunCMD executes external commands in a unified way, returns stdout+stderr text, and supports simple retries.
// dir: Process working directory; retries: Additional retry count (total attempts = 1 + retries).
func RunCMD(ctx context.Context, dir, name string, args []string, retries int) (string, error) {
	attempts := 1 + retries
	var out []byte
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		buf := &bytes.Buffer{}
		cmd.Stdout = buf
		cmd.Stderr = buf
		err = cmd.Run()
		out = buf.Bytes()

		// Return on success or when retry is not needed
		if err == nil || !shouldRetry(buf.String(), attempt, attempts) || ctx.Err() != nil {
			break
		}

		// Read configurable parameters
		maxRetries, maxBackoff := getRetryConfigs(retries)
		attempts = 1 + maxRetries
		// Exponential backoff (based on 200ms) and limit maximum backoff
		delay := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
		if delay > maxBackoff {
			delay = maxBackoff
		}
		// Debug level retry log
		Debugf("retrying command (attempt %d/%d) in %s: %s %s\n", attempt+1, attempts, delay, name, strings.Join(args, " "))
		select {
		case <-ctx.Done():
			return string(out), ctx.Err()
		case <-time.After(delay):
		}
	}
	if err != nil {
		return string(out), fmt.Errorf("exec failed: %s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return string(out), nil
}

// shouldRetry determines whether retry is worthwhile based on output characteristics (network/temporary errors).
func shouldRetry(output string, attempt, attempts int) bool {
	if attempt >= attempts {
		return false
	}
	low := strings.ToLower(output)
	// Common network/temporary error signals
	keys := []string{
		"timeout", "timed out", "temporary failure", "tls: handshake failure",
		"connection reset", "connection refused", "no route to host", "i/o timeout",
		"couldn't resolve host", "could not resolve host", "name or service not known",
		"remote error", "http 5", "internal server error", "rate limit",
	}
	for _, k := range keys {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// getRetryConfigs reads retry configuration from environment variables:
// LYNX_RETRIES: Maximum retry count (default uses passed retries)
// LYNX_MAX_BACKOFF_MS: Maximum backoff time (milliseconds, default 2000ms)
func getRetryConfigs(defaultRetries int) (int, time.Duration) {
	r := defaultRetries
	if v := strings.TrimSpace(os.Getenv("LYNX_RETRIES")); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			r = n
		}
	}
	maxBackoff := 2000 * time.Millisecond
	if v := strings.TrimSpace(os.Getenv("LYNX_MAX_BACKOFF_MS")); v != "" {
		if n, err := parsePositiveInt(v); err == nil && n > 0 {
			maxBackoff = time.Duration(n) * time.Millisecond
		}
	}
	return r, maxBackoff
}

// parsePositiveInt parses a string into a positive integer.
func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		n = 0
	}
	return n, nil
}
