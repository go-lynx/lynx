package kafka

import (
	"time"

	appmetrics "github.com/go-lynx/lynx/app/observability/metrics"
)

// hcProvider 适配 kafka 的 HealthChecker 到平台 HealthProvider
type hcProvider struct {
	hc *HealthChecker
	// 额外标签（暂不下发为动态标签，仅预留）
	labels map[string]string
}

func (p *hcProvider) IsHealthy() bool              { return p.hc.IsHealthy() }
func (p *hcProvider) ErrorCount() int              { return p.hc.GetErrorCount() }
func (p *hcProvider) LastCheck() time.Time         { return p.hc.GetLastCheck() }
func (p *hcProvider) Labels() map[string]string    { return p.labels }

// registerHealthForProducer 在全局注册表注册生产者实例的健康指标
func (k *Client) registerHealthForProducer(instance string) {
	k.mu.RLock()
	cm := k.prodConnMgrs[instance]
	k.mu.RUnlock()
	if cm == nil || cm.healthChecker == nil {
		return
	}
	provider := &hcProvider{hc: cm.healthChecker, labels: map[string]string{"role": "producer"}}
	collector := appmetrics.NewHealthCollector("kafka", "producer:"+instance, pluginVersion, provider)
	// 使用 MustRegister，重复注册会 panic；保证仅在首次启动连接管理器后注册
	appmetrics.MustRegister(collector)
}

// registerHealthForConsumer 在全局注册表注册消费者实例的健康指标
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
