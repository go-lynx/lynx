package envx

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Get 读取字符串环境变量，若为空则返回默认值。
func Get(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// GetInt 读取整型环境变量；解析失败返回默认值。
func GetInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

// GetBool 读取布尔环境变量；解析失败返回默认值。
func GetBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// GetDuration 读取 duration；支持 time.ParseDuration 与纯数字（按秒）。
func GetDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Duration(i) * time.Second
	}
	return def
}
