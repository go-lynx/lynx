package kafka

import (
	"time"

	appmetrics "github.com/go-lynx/lynx/app/observability/metrics"
)

// hcProvider adapts kafka's HealthChecker to platform HealthProvider
type hcProvider struct {
	hc *HealthChecker
	// Additional labels (not yet distributed as dynamic labels, reserved only)
	labels map[string]string
}

func (p *hcProvider) IsHealthy() bool           { return p.hc.IsHealthy() }
func (p *hcProvider) ErrorCount() int           { return p.hc.GetErrorCount() }
func (p *hcProvider) LastCheck() time.Time      { return p.hc.GetLastCheck() }
func (p *hcProvider) Labels() map[string]string { return p.labels }

// registerHealthForProducer registers producer instance health metrics in global registry
func (k *Client) registerHealthForProducer(instance string) {
	k.mu.RLock()
	cm := k.prodConnMgrs[instance]
	k.mu.RUnlock()
	if cm == nil || cm.healthChecker == nil {
		return
	}
	provider := &hcProvider{hc: cm.healthChecker, labels: map[string]string{"role": "producer"}}
	collector := appmetrics.NewHealthCollector("kafka", "producer:"+instance, pluginVersion, provider)
	// Use MustRegister, duplicate registration will panic; ensure registration only after first connection manager startup
	appmetrics.MustRegister(collector)
}

// registerHealthForConsumer registers consumer instance health metrics in global registry
func (k *Client) registerHealthForConsumer(instance string) {
	k.mu.RLock()
	cm := k.consConnMgrs[instance]
	k.mu.RUnlock()
	if cm == nil || cm.healthChecker == nil {
		return
	}
	provider := &hcProvider{hc: cm.healthChecker, labels: map[string]string{"role": "consumer"}}
	collector := appmetrics.NewHealthCollector("kafka", "consumer:"+instance, pluginVersion, provider)
	appmetrics.MustRegister(collector)
}
