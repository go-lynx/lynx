package plugins

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginError_ErrorAndUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	e := NewPluginError("redis", "Start", "connection failed", cause)

	msg := e.Error()
	require.Contains(t, msg, "plugin 'redis'")
	require.Contains(t, msg, "operation 'Start'")
	require.Contains(t, msg, "connection failed")
	require.Contains(t, msg, "root cause")

	// Unwrap must expose the cause so errors.Is/As work through the chain.
	require.ErrorIs(t, e, cause)
	require.Equal(t, cause, e.Unwrap())
}

func TestPluginError_WithCodeContextStack(t *testing.T) {
	e := NewPluginErrorWithCode(ErrorCodePluginOperationTimeout, "etcd", "Stop", "timed out", nil)
	require.Contains(t, e.Error(), "[PLUGIN_OPERATION_TIMEOUT]")

	e.WithContext("attempt", 3).WithContext("addr", "127.0.0.1")
	require.Equal(t, 3, e.Context["attempt"])
	require.Equal(t, "127.0.0.1", e.Context["addr"])

	e.WithStackTrace()
	require.NotEmpty(t, e.StackTrace, "stack trace should be captured")
}

func TestIsAndGetPluginError(t *testing.T) {
	cause := errors.New("inner")
	pe := NewPluginError("p", "op", "m", cause)

	require.True(t, IsPluginError(pe))
	require.False(t, IsPluginError(errors.New("plain")))

	// GetPluginError must find the PluginError even when wrapped further down the chain.
	wrapped := fmt.Errorf("layer: %w", pe)
	require.Equal(t, pe, GetPluginError(wrapped))
	require.Nil(t, GetPluginError(errors.New("plain")))
}

func TestFormatErrorForUserAndDeveloper(t *testing.T) {
	pe := NewPluginError("kafka", "Start", "broker unreachable", errors.New("dial tcp: timeout"))
	pe.WithContext("broker", "b1")

	user := FormatErrorForUser(pe)
	require.Contains(t, user, "Plugin 'kafka' failed during 'Start'")
	require.Contains(t, user, "broker unreachable")
	// User-facing message must not leak the low-level cause.
	require.NotContains(t, user, "dial tcp")

	dev := FormatErrorForDeveloper(pe)
	require.Contains(t, dev, "broker unreachable")
	require.Contains(t, dev, "Context")
	require.Contains(t, dev, "broker")

	// Plain (non-plugin) errors fall back to their own message.
	plain := errors.New("just a plain error")
	require.Equal(t, "just a plain error", FormatErrorForUser(plain))
	require.True(t, strings.Contains(FormatErrorForDeveloper(plain), "just a plain error"))
}

func TestStandardError(t *testing.T) {
	require.Contains(t, ErrPluginNotFound.Error(), "plugin not found")
	require.Contains(t, ErrPluginNotInitialized.Error(), "not initialized")
}
