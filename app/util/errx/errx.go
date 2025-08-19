package errx

import (
	"errors"
	"fmt"
)

// All 将多个错误聚合为一个（过滤 nil）。当所有输入均为 nil 时返回 nil。
func All(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return errors.Join(filtered...)
}

// First 返回第一个非 nil 错误。
func First(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Wrap 使用 fmt.Errorf 追加上下文并保留可解包的原错误。
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// DeferRecover 用于 defer，捕获 panic 并交给 handler 处理。
// 示例：
//   defer util.DeferRecover(func(e any){ logger.Error().Any("panic", e).Msg("recovered") })
func DeferRecover(handler func(any)) {
	if r := recover(); r != nil {
		if handler != nil {
			handler(r)
		}
	}
}
