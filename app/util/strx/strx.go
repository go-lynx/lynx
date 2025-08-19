package strx

import (
	"strings"
	"unicode"
)

// HasPrefixAny 判断 s 是否具有任一前缀。
func HasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// HasSuffixAny 判断 s 是否具有任一后缀。
func HasSuffixAny(s string, suffixes ...string) bool {
	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf) {
			return true
		}
	}
	return false
}

// Truncate 对字符串进行 rune 安全截断；max<=0 返回空。
// 若提供 ellipsis 且实际截断，则在末尾拼接 ellipsis。
func Truncate(s string, max int, ellipsis string) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if ellipsis == "" {
		return string(runes[:max])
	}
	if max < len([]rune(ellipsis)) {
		// 省略号长度超过 max，直接返回截断
		return string(runes[:max])
	}
	cut := max - len([]rune(ellipsis))
	if cut < 0 {
		cut = 0
	}
	return string(runes[:cut]) + ellipsis
}

// TrimSpaceAndCompress 去除首尾空白并将内部连续空白压缩为单个空格。
func TrimSpaceAndCompress(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// SplitAndTrim 按分隔符拆分，去除各项空白并剔除空项。
func SplitAndTrim(s, sep string) []string {
	if sep == "" {
		return []string{strings.TrimSpace(s)}
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
