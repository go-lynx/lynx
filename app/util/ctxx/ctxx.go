package ctxx

import (
	"context"
	"time"
)

// WithTimeout proxies context.WithTimeout to keep a unified/replaceable entry.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// Value fetches a value from context in a type-safe way.
func Value[T any](ctx context.Context, key any) (T, bool) {
	var zero T
	v := ctx.Value(key)
	if v == nil {
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

// Detach drops cancel/deadline info while preserving values (Go 1.21+ provides WithoutCancel).
func Detach(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}
