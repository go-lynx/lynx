package elasticsearch

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-lynx/lynx/app/log"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/elasticsearch/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Initialize Elasticsearch plugin
func (p *PlugElasticsearch) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	err := p.BasePlugin.Initialize(plugin, rt)
	if err != nil {
		log.Error(err)
		return err
	}

	// Get configuration from runtime
	cfg := rt.GetConfig()
	if cfg == nil {
		return fmt.Errorf("failed to get config from runtime")
	}

	// Parse configuration
	if err := p.parseConfig(cfg); err != nil {
		return fmt.Errorf("failed to parse elasticsearch config: %w", err)
	}

	// Create Elasticsearch client
	if err := p.createClient(); err != nil {
		return fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// Start metrics collection
	if p.conf.EnableMetrics {
		p.startMetricsCollection()
	}

	// Start health check
	if p.conf.EnableHealthCheck {
		p.startHealthCheck()
	}

	log.Info("elasticsearch plugin initialized successfully")
	return nil
}

// Start Elasticsearch plugin
func (p *PlugElasticsearch) Start(plugin plugins.Plugin) error {
	err := p.BasePlugin.Start(plugin)
	if err != nil {
		log.Error(err)
		return err
	}

	// Test connection
	if err := p.testConnection(); err != nil {
		return fmt.Errorf("failed to test elasticsearch connection: %w", err)
	}

	// Ensure shared quit channel is initialized when any background task is enabled
	if p.statsQuit == nil && (p.conf.EnableMetrics || p.conf.EnableHealthCheck) {
		p.statsQuit = make(chan struct{})
	}

	log.Info("elasticsearch plugin started successfully")
	return nil
}

// Stop Elasticsearch plugin
func (p *PlugElasticsearch) Stop(plugin plugins.Plugin) error {
	err := p.BasePlugin.Stop(plugin)
	if err != nil {
		log.Error(err)
		return err
	}

	// Stop metrics collection
	if p.conf.EnableMetrics {
		p.stopMetricsCollection()
	}

	// Stop health check
	if p.conf.EnableHealthCheck {
		p.stopHealthCheck()
	}

	log.Info("elasticsearch plugin stopped successfully")
	return nil
}

// parseConfig Parse configuration
func (p *PlugElasticsearch) parseConfig(cfg config.Config) error {
	// Read elasticsearch configuration from config
	var elasticsearchConf conf.Elasticsearch
	if err := cfg.Scan(&elasticsearchConf); err != nil {
		return err
	}
	p.conf = &elasticsearchConf

	// Set default values
	if len(p.conf.Addresses) == 0 {
		p.conf.Addresses = []string{"http://localhost:9200"}
	}
	if p.conf.MaxRetries == 0 {
		p.conf.MaxRetries = 3
	}
	if p.conf.ConnectTimeout == nil {
		p.conf.ConnectTimeout = durationpb.New(30 * time.Second)
	}
	if p.conf.HealthCheckInterval == nil {
		p.conf.HealthCheckInterval = durationpb.New(30 * time.Second)
	}

	return nil
}

// createClient Create Elasticsearch client
func (p *PlugElasticsearch) createClient() error {
	// Build client configuration
	clientConfig := elasticsearch.Config{
		Addresses:           p.conf.Addresses,
		MaxRetries:          int(p.conf.MaxRetries),
		CompressRequestBody: p.conf.CompressRequestBody,
	}

	// Set authentication information
	if p.conf.Username != "" && p.conf.Password != "" {
		clientConfig.Username = p.conf.Username
		clientConfig.Password = p.conf.Password
	}

	if p.conf.ApiKey != "" {
		clientConfig.APIKey = p.conf.ApiKey
	}

	if p.conf.ServiceToken != "" {
		clientConfig.ServiceToken = p.conf.ServiceToken
	}

	if p.conf.CertificateFingerprint != "" {
		clientConfig.CertificateFingerprint = p.conf.CertificateFingerprint
	}

	// Create client
	client, err := elasticsearch.NewClient(clientConfig)
	if err != nil {
		return err
	}

	p.client = client
	return nil
}

// testConnection Test connection
func (p *PlugElasticsearch) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send ping request
	res, err := p.client.Ping(p.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error(err)
		}
	}(res.Body)

	if res.IsError() {
		return fmt.Errorf("elasticsearch ping failed with status: %d", res.StatusCode)
	}

	return nil
}

// startMetricsCollection Start metrics collection
func (p *PlugElasticsearch) startMetricsCollection() {
	if p.statsQuit == nil {
		p.statsQuit = make(chan struct{})
	}
	p.statsWG.Add(1)

	go func() {
		defer p.statsWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.collectMetrics()
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopMetricsCollection Stop metrics collection
func (p *PlugElasticsearch) stopMetricsCollection() {
	if p.statsQuit != nil {
		p.closeStatsQuitOnce()
		p.statsWG.Wait()
	}
}

// collectMetrics Collect metrics
func (p *PlugElasticsearch) collectMetrics() {
	// Here you can collect Elasticsearch cluster metrics
	// For example: node status, index statistics, query performance, etc.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get cluster health status
	healthRes, err := p.client.Cluster.Health(p.client.Cluster.Health.WithContext(ctx))
	if err != nil {
		log.Errorf("failed to get cluster health: %v", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error(err)
		}
	}(healthRes.Body)

	// Get cluster statistics
	statsRes, err := p.client.Cluster.Stats(p.client.Cluster.Stats.WithContext(ctx))
	if err != nil {
		log.Errorf("failed to get cluster stats: %v", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error(err)
		}
	}(statsRes.Body)

	// Here you can send metrics to monitoring system
	log.Debug("elasticsearch metrics collected")
}

// startHealthCheck Start health check
func (p *PlugElasticsearch) startHealthCheck() {
	interval := p.conf.HealthCheckInterval.AsDuration()

	// Ensure quit channel exists
	if p.statsQuit == nil {
		p.statsQuit = make(chan struct{})
	}

	p.statsWG.Add(1)
	go func() {
		defer p.statsWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := p.checkHealth(); err != nil {
					log.Errorf("elasticsearch health check failed: %v", err)
				}
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopHealthCheck Stop health check
func (p *PlugElasticsearch) stopHealthCheck() {
	if p.statsQuit != nil {
		p.closeStatsQuitOnce()
		// Wait for all goroutines (metrics + health) to exit, set timeout to avoid infinite wait
		done := make(chan struct{})
		go func() {
			p.statsWG.Wait()
			close(done)
		}()
		select {
		case <-done:
			log.Infof("elasticsearch background tasks stopped successfully")
		case <-time.After(10 * time.Second):
			log.Warnf("timeout waiting for elasticsearch background tasks to stop")
		}
	}
}

// closeStatsQuitOnce closes statsQuit only once in a thread-safe way
func (p *PlugElasticsearch) closeStatsQuitOnce() {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	if !p.statsClosed && p.statsQuit != nil {
		close(p.statsQuit)
		p.statsClosed = true
	}
}

// checkHealth Perform health check
func (p *PlugElasticsearch) checkHealth() error {
	// Use lightweight Ping for health check to avoid higher overhead of Cluster Health
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	res, err := p.client.Ping(p.client.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error(err)
		}
	}(res.Body)
	latency := time.Since(start)

	if res.IsError() {
		return fmt.Errorf("ping health check failed: status=%d, latency=%s", res.StatusCode, latency)
	}

	log.Debugf("elasticsearch ping ok: status=%d, latency=%s", res.StatusCode, latency)
	return nil
}

// GetClient Get Elasticsearch client
func (p *PlugElasticsearch) GetClient() *elasticsearch.Client {
	return p.client
}

// GetConnectionStats Get connection statistics
func (p *PlugElasticsearch) GetConnectionStats() map[string]any {
	stats := make(map[string]any)

	if p.client != nil {
		// Get client statistics
		stats["client_initialized"] = true
		stats["addresses"] = p.conf.Addresses
		stats["max_retries"] = p.conf.MaxRetries
		stats["compression_enabled"] = p.conf.CompressRequestBody
	} else {
		stats["client_initialized"] = false
	}

	return stats
}
