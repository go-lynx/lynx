package errx

import (
	"errors"
	"fmt"
)

// All aggregates multiple errors into one (filters nil). Returns nil if all inputs are nil.
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

// First returns the first non-nil error.
func First(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Wrap adds context using fmt.Errorf and preserves the original error for unwrapping.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// DeferRecover is used in defer to catch panics and pass them to handler.
// Example:
//   defer util.DeferRecover(func(e any){ logger.Error().Any("panic", e).Msg("recovered") })
func DeferRecover(handler func(any)) {
	if r := recover(); r != nil {
		if handler != nil {
			handler(r)
		}
	}
}
