package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns unified /metrics HTTP handler
func Handler() http.Handler {
	// Aggregate this package's registry + default global (most plugins use prometheus.MustRegister directly) + optional extra gatherers
	g := prometheus.Gatherers{registry, prometheus.DefaultGatherer}
	if len(extraGatherers) > 0 {
		g = append(g, extraGatherers...)
	}
	return promhttp.HandlerFor(g, promhttp.HandlerOpts{
		EnableOpenMetrics:  true,
		DisableCompression: false,
	})
}
