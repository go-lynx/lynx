// Package app: plugin lifecycle operations (init/start/stop) with safety.
package app

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/events"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/observability/metrics"
	"github.com/go-lynx/lynx/plugins"
	"github.com/prometheus/client_golang/prometheus"
)

// loadSortedPluginsByLevel starts plugins in parallel by dependency level and rolls back on failure.
func (m *DefaultPluginManager[T]) loadSortedPluginsByLevel(sorted []PluginWithLevel) error {
	groups := make(map[int][]plugins.Plugin)
	levels := make([]int, 0)
	seen := make(map[int]bool)
	for _, pwl := range sorted {
		if pwl.Plugin == nil {
			continue
		}
		groups[pwl.level] = append(groups[pwl.level], pwl.Plugin)
		if !seen[pwl.level] {
			levels = append(levels, pwl.level)
			seen[pwl.level] = true
		}
	}
	sort.Ints(levels)

	started := make([]plugins.Plugin, 0)
	var startedMu sync.Mutex
	par := m.getStartParallelism()
	initTimeout := m.getInitTimeout()
	startTimeout := m.getStartTimeout()

	for _, lv := range levels {
		batch := groups[lv]
		if len(batch) == 0 {
			continue
		}

		sem := make(chan struct{}, par)
		var wg sync.WaitGroup
		errCh := make(chan error, len(batch))

		for _, p := range batch {
			p := p
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				rt := m.runtime.WithPluginContext(p.ID())
				
				// Emit plugin initializing event
				m.emitPluginEvent(p.ID(), events.EventPluginInitializing, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
				})
				
				if err := m.safeInitPlugin(p, rt, initTimeout); err != nil {
					// Emit error event
					m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
						"error":       err.Error(),
						"plugin_name": p.Name(),
						"operation":   "initialize",
					})
					errCh <- fmt.Errorf("failed to initialize plugin %s: %w", p.ID(), err)
					return
				}
				
				// Emit plugin initialized event
				m.emitPluginEvent(p.ID(), events.EventPluginInitialized, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
				})
				
				// Emit plugin starting event
				m.emitPluginEvent(p.ID(), events.EventPluginStarting, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
				})
				
				if err := m.safeStartPlugin(p, startTimeout); err != nil {
					_ = m.runtime.CleanupResources(p.ID())
					// Emit error event
					m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
						"error":       err.Error(),
						"plugin_name": p.Name(),
						"operation":   "start",
					})
					errCh <- fmt.Errorf("failed to start plugin %s: %w", p.ID(), err)
					return
				}
				
				// Emit plugin started event
				m.emitPluginEvent(p.ID(), events.EventPluginStarted, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
				})

				m.pluginInstances.Store(p.Name(), p)

				type metricsGathererProvider interface{ MetricsGatherer() prometheus.Gatherer }
				if prov, ok := p.(metricsGathererProvider); ok {
					if g := prov.MetricsGatherer(); g != nil {
						metrics.RegisterGatherer(g)
						log.Infof("registered private metrics gatherer for plugin %s", p.ID())
					}
				}

				startedMu.Lock()
				started = append(started, p)
				startedMu.Unlock()
			}()
		}

		wg.Wait()
		close(errCh)
		if len(errCh) > 0 {
			var firstErr error
			for e := range errCh {
				if firstErr == nil {
					firstErr = e
				}
				log.Errorf("plugin start error: %v", e)
			}
			timeout := m.getStopTimeout()
			for i := len(started) - 1; i >= 0; i-- {
				p := started[i]
				if p == nil {
					continue
				}
				if err := m.safeStopPlugin(p, timeout); err != nil {
					log.Warnf("rollback stop failed for plugin %s: %v", p.Name(), err)
				}
				if err := m.runtime.CleanupResources(p.ID()); err != nil {
					log.Warnf("rollback cleanup failed for plugin %s: %v", p.Name(), err)
				}
				m.pluginInstances.Delete(p.Name())
			}
			return firstErr
		}
	}

	return nil
}

// getStartParallelism returns start/init parallelism, default 8.
func (m *DefaultPluginManager[T]) getStartParallelism() int {
	d := 8
	if m == nil || m.config == nil {
		return d
	}
	var v int
	if err := m.config.Value("lynx.plugins.start_parallelism").Scan(&v); err == nil {
		if v > 0 {
			return v
		}
	}
	return d
}

// getInitTimeout returns Initialize timeout, default 5s.
func (m *DefaultPluginManager[T]) getInitTimeout() time.Duration {
	d := 5 * time.Second
	if m == nil || m.config == nil {
		return d
	}
	var confStr string
	if err := m.config.Value("lynx.plugins.init_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			return parsed
		}
	}
	return d
}

// getStartTimeout returns Start timeout, default 5s.
func (m *DefaultPluginManager[T]) getStartTimeout() time.Duration {
	d := 5 * time.Second
	if m == nil || m.config == nil {
		return d
	}
	var confStr string
	if err := m.config.Value("lynx.plugins.start_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			return parsed
		}
	}
	return d
}

// getStopTimeout returns Stop timeout, default 5s.
func (m *DefaultPluginManager[T]) getStopTimeout() time.Duration {
	d := 5 * time.Second
	if m == nil || m.config == nil {
		return d
	}
	var confStr string
	if err := m.config.Value("lynx.plugins.stop_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			return parsed
		}
	}
	return d
}

// safeInitPlugin safely calls Initialize with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeInitPlugin(p plugins.Plugin, rt plugins.Runtime, timeout time.Duration) error {
	if p == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Initialize of %s: %v", p.ID(), r)
			}
		}()
		done <- p.Initialize(p, rt)
	}()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("initialize timeout after %s for plugin %s", timeout.String(), p.Name())
	}
}

// emitPluginEvent emits a plugin event to the unified event system
func (m *DefaultPluginManager[T]) emitPluginEvent(pluginID string, eventType events.EventType, metadata map[string]any) {
	if m.runtime == nil {
		return
	}
	
	// Convert events.EventType to plugins.EventType
	var pluginEventType plugins.EventType
	switch eventType {
	case events.EventPluginInitializing:
		pluginEventType = plugins.EventPluginInitializing
	case events.EventPluginInitialized:
		pluginEventType = plugins.EventPluginInitialized
	case events.EventPluginStarting:
		pluginEventType = plugins.EventPluginStarting
	case events.EventPluginStarted:
		pluginEventType = plugins.EventPluginStarted
	case events.EventPluginStopping:
		pluginEventType = plugins.EventPluginStopping
	case events.EventPluginStopped:
		pluginEventType = plugins.EventPluginStopped
	case events.EventErrorOccurred:
		pluginEventType = plugins.EventErrorOccurred
	default:
		pluginEventType = plugins.EventPluginInitializing
	}
	
	// Create plugin event
	pluginEvent := plugins.PluginEvent{
		Type:      pluginEventType,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "lifecycle",
		PluginID:  pluginID,
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}
	
	// Emit through runtime
	m.runtime.EmitEvent(pluginEvent)
}

// safeStartPlugin safely calls Start with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeStartPlugin(p plugins.Plugin, timeout time.Duration) error {
	if p == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Start of %s: %v", p.ID(), r)
			}
		}()
		done <- p.Start(p)
	}()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("start timeout after %s for plugin %s", timeout.String(), p.Name())
	}
}

// safeStopPlugin safely calls Stop with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeStopPlugin(p plugins.Plugin, timeout time.Duration) error {
	if p == nil {
		return nil
	}
	
	// Emit plugin stopping event
	m.emitPluginEvent(p.ID(), events.EventPluginStopping, map[string]any{
		"plugin_name": p.Name(),
		"plugin_id":   p.ID(),
	})
	
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// convert panic to error
				done <- fmt.Errorf("panic in Stop of %s: %v", p.Name(), r)
			}
		}()
		done <- p.Stop(p)
	}()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case err := <-done:
		if err != nil {
			// Emit error event
			m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
				"error":       err.Error(),
				"plugin_name": p.Name(),
				"operation":   "stop",
			})
		} else {
			// Emit plugin stopped event
			m.emitPluginEvent(p.ID(), events.EventPluginStopped, map[string]any{
				"plugin_name": p.Name(),
				"plugin_id":   p.ID(),
			})
		}
		return err
	case <-time.After(timeout):
		timeoutErr := fmt.Errorf("stop timeout after %s for plugin %s", timeout.String(), p.Name())
		// Emit timeout error event
		m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
			"error":       timeoutErr.Error(),
			"plugin_name": p.Name(),
			"operation":   "stop",
		})
		return timeoutErr
	}
}
