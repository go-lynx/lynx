package cast

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ToInt converts a value of any type to an int.
// It supports various numeric types, string representations of integers, and fmt.Stringer implementations.
// Returns an error if the conversion is not possible or if the value overflows.
func ToInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int8:
		return int(x), nil
	case int16:
		return int(x), nil
	case int32:
		return int(x), nil
	case int64:
		return int(x), nil
	case uint, uint8, uint16, uint32, uint64:
		return toIntFromUint(x)
	case float32:
		return int(x), nil
	case float64:
		return int(x), nil
	case string:
		i, err := strconv.Atoi(x)
		if err != nil {
			return 0, fmt.Errorf("ToInt: parse '%s': %w", x, err)
		}
		return i, nil
	case fmt.Stringer:
		return ToInt(x.String())
	default:
		return 0, fmt.Errorf("ToInt: unsupported type %T", v)
	}
}

// ToIntDefault converts a value of any type to an int, returning a default value if conversion fails.
// It uses ToInt for the conversion and returns the provided default value if an error occurs.
func ToIntDefault(v any, def int) int {
	if i, err := ToInt(v); err == nil {
		return i
	}
	return def
}

// toIntFromUint converts unsigned integer types to int.
// Handles overflow checking for uint64 values.
func toIntFromUint(v any) (int, error) {
	switch x := v.(type) {
	case uint:
		return int(x), nil
	case uint8:
		return int(x), nil
	case uint16:
		return int(x), nil
	case uint32:
		return int(x), nil
	case uint64:
		// Check for overflow when converting uint64 to int
		if x > uint64(^uint(0)>>1) {
			return 0, errors.New("ToInt: uint64 overflow")
		}
		return int(x), nil
	default:
		return 0, fmt.Errorf("ToInt: unsupported uint type %T", v)
	}
}

// ToBool converts a value of any type to a boolean.
// It supports boolean types, string representations of booleans, and numeric types (0 is false, non-zero is true).
// Returns an error if the conversion is not possible.
func ToBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(x)
		if err != nil {
			return false, fmt.Errorf("ToBool: parse '%s': %w", x, err)
		}
		return b, nil
	case int, int8, int16, int32, int64:
		i, _ := ToInt(x)
		return i != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		i, _ := ToInt(x)
		return i != 0, nil
	default:
		return false, fmt.Errorf("ToBool: unsupported type %T", v)
	}
}

// ToBoolDefault converts a value of any type to a boolean, returning a default value if conversion fails.
// It uses ToBool for the conversion and returns the provided default value if an error occurs.
func ToBoolDefault(v any, def bool) bool {
	if b, err := ToBool(v); err == nil {
		return b
	}
	return def
}

// ToFloat64 converts a value of any type to a float64.
// It supports various numeric types, string representations of floats, and fmt.Stringer implementations.
// Returns an error if the conversion is not possible.
func ToFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int, int8, int16, int32, int64:
		i, _ := ToInt(x)
		return float64(i), nil
	case uint, uint8, uint16, uint32, uint64:
		i, _ := ToInt(x)
		return float64(i), nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, fmt.Errorf("ToFloat64: parse '%s': %w", x, err)
		}
		return f, nil
	case fmt.Stringer:
		return ToFloat64(x.String())
	default:
		return 0, fmt.Errorf("ToFloat64: unsupported type %T", v)
	}
}

// ToFloat64Default converts a value of any type to a float64, returning a default value if conversion fails.
// It uses ToFloat64 for the conversion and returns the provided default value if an error occurs.
func ToFloat64Default(v any, def float64) float64 {
	if f, err := ToFloat64(v); err == nil {
		return f
	}
	return def
}

// ToDuration converts a value of any type to a time.Duration.
// It supports time.Duration, string representations (using time.ParseDuration),
// and numeric types (interpreted as seconds).
// Returns an error if the conversion is not possible.
func ToDuration(v any) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil
	case string:
		d, err := time.ParseDuration(x)
		if err == nil {
			return d, nil
		}
		// Also support plain numeric strings as seconds
		if i, e := strconv.ParseInt(x, 10, 64); e == nil {
			return time.Duration(i) * time.Second, nil
		}
		return 0, fmt.Errorf("ToDuration: parse '%s': %w", x, err)
	case int, int8, int16, int32, int64:
		i, _ := ToInt(x)
		return time.Duration(i) * time.Second, nil
	case uint, uint8, uint16, uint32, uint64:
		i, _ := ToInt(x)
		return time.Duration(i) * time.Second, nil
	default:
		return 0, fmt.Errorf("ToDuration: unsupported type %T", v)
	}
}

// ToDurationDefault converts a value of any type to a time.Duration, returning a default value if conversion fails.
// It uses ToDuration for the conversion and returns the provided default value if an error occurs.
func ToDurationDefault(v any, def time.Duration) time.Duration {
	if d, err := ToDuration(v); err == nil {
		return d
	}
	return def
}
