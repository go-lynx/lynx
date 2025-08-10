package tracer

import (
    "github.com/go-lynx/lynx/plugins/tracer/conf"
    traceSdk "go.opentelemetry.io/otel/sdk/trace"
)

// buildSpanLimits 根据配置构建 OpenTelemetry SpanLimits。
// 仅设置当前 SDK 版本支持的字段：
// - AttributeCountLimit
// - AttributeValueLengthLimit
// - EventCountLimit
// - LinkCountLimit
// 未配置或值 <= 0 的字段将被忽略，返回 nil 表示不覆盖默认限额。
func buildSpanLimits(c *conf.Tracer) *traceSdk.SpanLimits {
    // 读取 modular 配置；若未提供 limits 则返回 nil，表示沿用 SDK 默认限额
    cfg := c.GetConfig()
    if cfg == nil || cfg.Limits == nil {
        return nil
    }
    // 取出限额配置
    l := cfg.GetLimits()
    // 初始化空的 SpanLimits；仅对 >0 的值进行赋值
    limits := &traceSdk.SpanLimits{}
    // 每个 Span 允许的最大 attribute 数量
    if v := l.GetAttributeCountLimit(); v > 0 {
        limits.AttributeCountLimit = int(v)
    }
    // 单个 attribute 的最大值长度（字符）
    if v := l.GetAttributeValueLengthLimit(); v > 0 {
        limits.AttributeValueLengthLimit = int(v)
    }
    // 每个 Span 允许的最大 event 数量
    if v := l.GetEventCountLimit(); v > 0 {
        limits.EventCountLimit = int(v)
    }
    // 每个 Span 允许的最大 link 数量
    if v := l.GetLinkCountLimit(); v > 0 {
        limits.LinkCountLimit = int(v)
    }
    // 注意：若以上字段均未设置（保持 0），SDK 会采用其默认值；返回空结构即可
    return limits
}
