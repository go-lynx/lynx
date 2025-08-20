package pgsql

import (
    "time"
    "github.com/go-lynx/lynx/plugins/db/pgsql/conf"
)

// 本文件为默认构建的占位实现，提供全局变量以避免未启用 hooks 时的编译错误。
// 真正的 hooks 实现在 hooks.go 中（需要使用 -tags lynx_pgsql_hooks 构建）。

var (
    globalPgsqlMetrics *PrometheusMetrics
    globalSlowThreshold = 200 * time.Millisecond
    globalPgsqlConf     *conf.Pgsql
)
