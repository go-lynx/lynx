package apollo

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// ConfigWatcher watches Apollo configuration changes
type ConfigWatcher struct {
	namespace string
	onChange  func(namespace string, key string, value string)
	onError   func(err error)
	stopCh    chan struct{}
	mu        sync.RWMutex
	metrics   *Metrics
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(namespace string) *ConfigWatcher {
	return &ConfigWatcher{
		namespace: namespace,
		stopCh:    make(chan struct{}),
	}
}

// SetOnConfigChanged sets the callback for configuration changes
func (w *ConfigWatcher) SetOnConfigChanged(callback func(namespace string, key string, value string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = callback
}

// SetOnError sets the error callback
func (w *ConfigWatcher) SetOnError(callback func(err error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onError = callback
}

// Start starts watching configuration changes
// Note: This is a legacy method for backward compatibility.
// The actual watching is handled by ApolloConfigWatcher which implements config.Watcher.
func (w *ConfigWatcher) Start() {
	// This method is kept for backward compatibility
	// The actual watching is done through ApolloConfigWatcher
	log.Infof("Config watcher started for namespace: %s (using ApolloConfigWatcher)", w.namespace)
}

// Stop stops watching configuration changes
func (w *ConfigWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	select {
	case <-w.stopCh:
		// Already stopped
		return
	default:
		close(w.stopCh)
		log.Infof("Stopped config watcher for namespace: %s", w.namespace)
	}
}

// WatchConfig watches configuration changes
func (p *PlugApollo) WatchConfig(namespace string) (*ConfigWatcher, error) {
	if !p.IsInitialized() {
		return nil, NewInitError("Apollo plugin not initialized")
	}

	// Record configuration watch operation metrics
	if p.metrics != nil {
		p.metrics.RecordConfigOperation(namespace, "watch", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordConfigOperation(namespace, "watch", "success")
			}
		}()
	}

	log.Infof("Watching config - Namespace: %s", namespace)

	// Check if the configuration is already being watched
	p.watcherMutex.Lock()
	if existingWatcher, exists := p.configWatchers[namespace]; exists {
		p.watcherMutex.Unlock()
		log.Infof("Config %s is already being watched", namespace)
		return existingWatcher, nil
	}
	p.watcherMutex.Unlock()

	// Create configuration watcher
	watcher := NewConfigWatcher(namespace)
	watcher.metrics = p.metrics // Pass metrics reference

	// Set event handling callbacks
	watcher.SetOnConfigChanged(func(ns string, key string, value string) {
		p.handleConfigChanged(ns, key, value)
	})

	watcher.SetOnError(func(err error) {
		p.handleConfigWatchError(namespace, err)
	})

	// Register watcher
	p.watcherMutex.Lock()
	p.configWatchers[namespace] = watcher
	p.watcherMutex.Unlock()

	// Start watching
	watcher.Start()

	return watcher, nil
}

// handleConfigChanged handles configuration change events
func (p *PlugApollo) handleConfigChanged(namespace, key, value string) {
	log.Infof("Config changed - Namespace: [%s], Key: [%s]", namespace, key)

	// Record configuration change metrics
	if p.metrics != nil {
		p.metrics.RecordConfigChange(namespace)
	}

	// Clear cache for this namespace
	p.cacheMutex.Lock()
	delete(p.configCache, fmt.Sprintf("%s:%s", namespace, key))
	p.cacheMutex.Unlock()
}

// handleConfigWatchError handles configuration watch errors
func (p *PlugApollo) handleConfigWatchError(namespace string, err error) {
	log.Errorf("Config watch error - Namespace: [%s], Error: %v", namespace, err)

	// Record error metrics
	if p.metrics != nil {
		p.metrics.RecordConfigOperation(namespace, "watch", "error")
	}

	// Retry watching if needed
	if p.conf.EnableRetry {
		go p.retryConfigWatch(namespace)
	}
}

// retryConfigWatch retries configuration watching
func (p *PlugApollo) retryConfigWatch(namespace string) {
	log.Infof("Retrying config watch for %s", namespace)

	// Wait for a period before retrying, but allow cancellation on plugin stop
	if p.healthCheckCh != nil {
		select {
		case <-p.healthCheckCh:
			log.Infof("Config watch retry canceled due to plugin shutdown: %s", namespace)
			return
		case <-time.After(5 * time.Second):
		}
	} else {
		// Fallback when channel is not available
		if p.IsDestroyed() {
			return
		}
		time.Sleep(5 * time.Second)
	}

	if p.IsDestroyed() {
		return
	}

	// Recreate watcher
	if _, err := p.WatchConfig(namespace); err == nil {
		log.Infof("Successfully recreated config watcher for %s", namespace)
	} else {
		log.Errorf("Failed to recreate config watcher for %s: %v", namespace, err)
	}
}
