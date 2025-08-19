package timex

import (
	"fmt"
	"math/rand"
	"time"
)

// NowUTC 返回当前 UTC 时间。
func NowUTC() time.Time { return time.Now().UTC() }

// ParseAny 依次尝试使用 layouts 解析时间字符串。
func ParseAny(layouts []string, s string) (time.Time, error) {
	var lastErr error
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return time.Time{}, lastErr
	}
	return time.Time{}, fmt.Errorf("ParseAny: no layouts provided for input %q", s)
}

// Align 将时间向下对齐到给定的间隔边界（如 5m、1h）。
func Align(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	// 转为自 Unix 纪元的纳秒数进行对齐
	unixNano := t.UnixNano()
	aligned := unixNano - (unixNano % int64(d))
	return time.Unix(0, aligned).In(t.Location())
}

// Jitter 在 [0, ratio] 范围内对时长做乘法抖动，ratio<0 时按 0 处理，ratio>1 时上限为 1。
func Jitter(d time.Duration, ratio float64) time.Duration {
	if d <= 0 || ratio == 0 {
		return d
	}
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	// 生成 [0, ratio] 区间的随机浮点
	f := rand.Float64() * ratio
	return time.Duration(float64(d) * (1 + f))
}

// Within 判断 t 是否在 [start, end] 闭区间内。
func Within(t, start, end time.Time) bool {
	if end.Before(start) {
		start, end = end, start
	}
	return (t.Equal(start) || t.After(start)) && (t.Equal(end) || t.Before(end))
}
