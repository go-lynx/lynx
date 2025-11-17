package nacos

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/nacos/conf"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosConfigSource implements config.Source interface for Nacos
type NacosConfigSource struct {
	client  config_client.IConfigClient
	dataId  string
	group   string
	format  string
	content string
	mu      sync.RWMutex
}

// NewNacosConfigSource creates a new Nacos config source
func NewNacosConfigSource(client config_client.IConfigClient, dataId, group, format string) (*NacosConfigSource, error) {
	if client == nil {
		return nil, fmt.Errorf("nacos config client is nil")
	}

	source := &NacosConfigSource{
		client: client,
		dataId: dataId,
		group:  group,
		format: format,
	}

	// Load initial config
	if err := source.load(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	return source, nil
}

// load loads configuration from Nacos
func (s *NacosConfigSource) load() error {
	param := vo.ConfigParam{
		DataId: s.dataId,
		Group:  s.group,
	}

	content, err := s.client.GetConfig(param)
	if err != nil {
		return WrapOperationError(err, "get config")
	}

	s.mu.Lock()
	s.content = content
	s.mu.Unlock()

	return nil
}

// Load returns the configuration content
func (s *NacosConfigSource) Load() ([]*config.KeyValue, error) {
	s.mu.RLock()
	content := s.content
	s.mu.RUnlock()

	if content == "" {
		return nil, fmt.Errorf("config content is empty for dataId: %s, group: %s", s.dataId, s.group)
	}

	kv := &config.KeyValue{
		Key:    s.dataId,
		Value:  []byte(content),
		Format: s.format,
	}

	return []*config.KeyValue{kv}, nil
}

// Watch watches configuration changes
func (s *NacosConfigSource) Watch() (config.Watcher, error) {
	return NewNacosConfigWatcher(s.client, s.dataId, s.group, s.format), nil
}

// NacosConfigWatcher implements config.Watcher interface
type NacosConfigWatcher struct {
	client      config_client.IConfigClient
	dataId      string
	group       string
	format      string
	stopCh      chan struct{}
	eventCh     chan []*config.KeyValue
	mu          sync.RWMutex
	running     bool
	stopOnce    sync.Once
	closed      int32 // Use atomic for checking if channels are closed
}

// NewNacosConfigWatcher creates a new Nacos config watcher
func NewNacosConfigWatcher(client config_client.IConfigClient, dataId, group, format string) *NacosConfigWatcher {
	return &NacosConfigWatcher{
		client:  client,
		dataId:  dataId,
		group:   group,
		format:  format,
		stopCh:  make(chan struct{}),
		eventCh: make(chan []*config.KeyValue, 10),
	}
}

// Start starts watching configuration changes
func (w *NacosConfigWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher is already running")
	}
	w.running = true
	w.mu.Unlock()

	// Listen to config changes
	param := vo.ConfigParam{
		DataId:  w.dataId,
		Group:   w.group,
		OnChange: w.handleConfigChange,
	}

	err := w.client.ListenConfig(param)
	if err != nil {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		return fmt.Errorf("failed to listen config: %w", err)
	}

	// Start background goroutine to handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			w.Stop()
		case <-w.stopCh:
		}
	}()

	return nil
}

// handleConfigChange handles configuration change events
func (w *NacosConfigWatcher) handleConfigChange(namespace, group, dataId, data string) {
	if dataId != w.dataId || group != w.group {
		return
	}

	// Check if watcher is still running before sending to channel
	if atomic.LoadInt32(&w.closed) == 1 {
		return
	}

	kv := &config.KeyValue{
		Key:    dataId,
		Value:  []byte(data),
		Format: w.format,
	}

	// Send event (non-blocking, with closed channel check)
	select {
	case w.eventCh <- []*config.KeyValue{kv}:
	case <-w.stopCh:
		// Channel is closed, watcher is stopping
		return
	default:
		log.Warnf("Config watcher event channel full, dropping event for dataId: %s", dataId)
	}
}

// Next returns the next configuration change event
func (w *NacosConfigWatcher) Next() ([]*config.KeyValue, error) {
	select {
	case kvs := <-w.eventCh:
		return kvs, nil
	case <-w.stopCh:
		return nil, fmt.Errorf("watcher stopped")
	}
}

// Stop stops the watcher
func (w *NacosConfigWatcher) Stop() error {
	var wasRunning bool
	w.mu.Lock()
	wasRunning = w.running
	w.running = false
	w.mu.Unlock()

	if !wasRunning {
		return nil
	}

	// Use sync.Once to ensure channels are closed only once
	w.stopOnce.Do(func() {
		// Mark as closed atomically
		atomic.StoreInt32(&w.closed, 1)

		// Cancel listening
		param := vo.ConfigParam{
			DataId: w.dataId,
			Group:  w.group,
		}
		_ = w.client.CancelListenConfig(param)

		// Close channels safely
		close(w.stopCh)
		close(w.eventCh)
	})

	return nil
}

// ConfigWatcher wraps NacosConfigWatcher with additional functionality
type ConfigWatcher struct {
	watcher *NacosConfigWatcher
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(watcher *NacosConfigWatcher) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		watcher: watcher,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the watcher
func (cw *ConfigWatcher) Start() error {
	return cw.watcher.Start(cw.ctx)
}

// Stop stops the watcher
func (cw *ConfigWatcher) Stop() {
	cw.cancel()
	_ = cw.watcher.Stop()
}

// GetConfig gets configuration from Nacos
func (p *PlugNacos) GetConfig(dataId string, group string) (config.Source, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	if !p.conf.EnableConfig {
		return nil, fmt.Errorf("configuration management is disabled")
	}

	if p.configClient == nil {
		return nil, fmt.Errorf("nacos config client is nil")
	}

	if group == "" {
		group = conf.DefaultGroup
	}

	// Determine format from dataId extension
	format := "yaml"
	if dataId != "" {
		ext := getFileExtension(dataId)
		if ext != "" {
			format = ext
		}
	}

	return NewNacosConfigSource(p.configClient, dataId, group, format)
}

// GetConfigSources gets all configuration sources for multi-config loading
func (p *PlugNacos) GetConfigSources() ([]config.Source, error) {
	if err := p.checkInitialized(); err != nil {
		return nil, err
	}

	var sources []config.Source

	// Get main configuration source
	mainSource, err := p.getMainConfigSource()
	if err != nil {
		return nil, fmt.Errorf("failed to get main config source: %w", err)
	}
	if mainSource != nil {
		sources = append(sources, mainSource)
	}

	// Get additional configuration sources
	additionalSources, err := p.getAdditionalConfigSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get additional config sources: %w", err)
	}
	sources = append(sources, additionalSources...)

	return sources, nil
}

// getMainConfigSource gets the main configuration source
func (p *PlugNacos) getMainConfigSource() (config.Source, error) {
	appName := app.GetName()
	if appName == "" {
		appName = "application"
	}

	dataId := fmt.Sprintf("%s.yaml", appName)
	group := conf.DefaultGroup

	// Use service config if available
	if p.conf.ServiceConfig != nil && p.conf.ServiceConfig.ServiceName != "" {
		dataId = fmt.Sprintf("%s.yaml", p.conf.ServiceConfig.ServiceName)
		if p.conf.ServiceConfig.Group != "" {
			group = p.conf.ServiceConfig.Group
		}
	}

	log.Infof("Loading main configuration from Nacos - DataId: [%s] Group: [%s] Namespace: [%s]",
		dataId, group, p.getNamespace())

	return p.GetConfig(dataId, group)
}

// getAdditionalConfigSources gets additional configuration sources
func (p *PlugNacos) getAdditionalConfigSources() ([]config.Source, error) {
	if len(p.conf.AdditionalConfigs) == 0 {
		return nil, nil
	}

	var sources []config.Source
	for _, cfg := range p.conf.AdditionalConfigs {
		format := cfg.Format
		if format == "" {
			format = getFileExtension(cfg.DataId)
			if format == "" {
				format = "yaml"
			}
		}

		group := cfg.Group
		if group == "" {
			group = conf.DefaultGroup
		}

		source, err := p.GetConfig(cfg.DataId, group)
		if err != nil {
			log.Warnf("Failed to load additional config - DataId: %s, Group: %s, Error: %v",
				cfg.DataId, group, err)
			continue
		}

		// Set format if needed
		if nacosSource, ok := source.(*NacosConfigSource); ok {
			nacosSource.mu.Lock()
			nacosSource.format = format
			nacosSource.mu.Unlock()
		}

		sources = append(sources, source)
	}

	return sources, nil
}

// getFileExtension extracts file extension from filename
func getFileExtension(filename string) string {
	// Simple extension extraction
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}

	ext := strings.ToLower(parts[len(parts)-1])
	switch ext {
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "properties", "props":
		return "properties"
	case "xml":
		return "xml"
	default:
		return ext
	}
}

// WatchConfig watches configuration changes
func (p *PlugNacos) WatchConfig(dataId, group string, callback func(string)) error {
	if err := p.checkInitialized(); err != nil {
		return err
	}

	if !p.conf.EnableConfig {
		return fmt.Errorf("configuration management is disabled")
	}

	if p.configClient == nil {
		return fmt.Errorf("nacos config client is nil")
	}

	if group == "" {
		group = conf.DefaultGroup
	}

	// Create watcher
	watcher := NewNacosConfigWatcher(p.configClient, dataId, group, "yaml")
	configWatcher := NewConfigWatcher(watcher)

	// Start watcher
	if err := configWatcher.Start(); err != nil {
		return fmt.Errorf("failed to start config watcher: %w", err)
	}

	// Store watcher
	watcherKey := fmt.Sprintf("%s:%s", dataId, group)
	p.watcherMutex.Lock()
	p.configWatchers[watcherKey] = configWatcher
	p.watcherMutex.Unlock()

	// Start goroutine to handle events
	go func() {
		for {
			kvs, err := watcher.Next()
			if err != nil {
				log.Errorf("Config watcher error for %s:%s: %v", dataId, group, err)
				break
			}

			if len(kvs) > 0 && callback != nil {
				callback(string(kvs[0].Value))
			}
		}
	}()

	return nil
}

