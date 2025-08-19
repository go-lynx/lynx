package cast

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ---- Int ----
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

func ToIntDefault(v any, def int) int {
	if i, err := ToInt(v); err == nil {
		return i
	}
	return def
}

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
		if x > uint64(^uint(0)>>1) {
			return 0, errors.New("ToInt: uint64 overflow")
		}
		return int(x), nil
	default:
		return 0, fmt.Errorf("ToInt: unsupported uint type %T", v)
	}
}

// ---- Bool ----
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

func ToBoolDefault(v any, def bool) bool {
	if b, err := ToBool(v); err == nil {
		return b
	}
	return def
}

// ---- Float64 ----
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

func ToFloat64Default(v any, def float64) float64 {
	if f, err := ToFloat64(v); err == nil {
		return f
	}
	return def
}

// ---- Duration ----
// 支持字符串（time.ParseDuration）、数字（按秒）
func ToDuration(v any) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil
	case string:
		d, err := time.ParseDuration(x)
		if err == nil {
			return d, nil
		}
		// 兼容纯数字字符串表示秒
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

func ToDurationDefault(v any, def time.Duration) time.Duration {
	if d, err := ToDuration(v); err == nil {
		return d
	}
	return def
}
