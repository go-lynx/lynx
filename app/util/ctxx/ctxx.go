package ctxx

import (
	"context"
	"time"
)

// WithTimeout 直接代理 context.WithTimeout，便于统一入口与可替换。
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// Value 从 context 中以类型安全方式取值。
func Value[T any](ctx context.Context, key any) (T, bool) {
	var zero T
	v := ctx.Value(key)
	if v == nil {
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

// Detach 丢弃取消和截止信息，仅保留值（Go 1.21+ 提供 WithoutCancel）。
func Detach(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}
