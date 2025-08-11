package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler 返回统一 /metrics 的 HTTP 处理器
func Handler() http.Handler {
	// 聚合本包 registry + 默认全局（多数插件直接使用 prometheus.MustRegister）+ 可选额外 gatherers
	g := prometheus.Gatherers{registry, prometheus.DefaultGatherer}
	if len(extraGatherers) > 0 {
		g = append(g, extraGatherers...)
	}
	return promhttp.HandlerFor(g, promhttp.HandlerOpts{
		EnableOpenMetrics:  true,
		DisableCompression: false,
	})
}
