// Package app: plugin lifecycle operations (init/start/stop) with safety.
package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
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

				// Detect whether plugin supports context-aware lifecycle and truly honors ctx
				supportsLC := false
				if _, ok := p.(plugins.LifecycleWithContext); ok {
					supportsLC = true
				}
				ctxAware := false
				if supportsLC {
					if ca, ok := p.(plugins.ContextAwareness); ok && ca.IsContextAware() {
						ctxAware = true
					}
				}
				// Debug info for context awareness (only show in debug mode)
				if !supportsLC {
					log.Debugf("plugin %s (%s) does not implement LifecycleWithContext; step=initialize. Please implement StartContext/StopContext/InitializeContext", p.Name(), p.ID())
				} else if !ctxAware {
					log.Debugf("plugin %s (%s) implements LifecycleWithContext but not truly context-aware; step=initialize. Ensure methods observe ctx and implement ContextAwareness.IsContextAware()=true", p.Name(), p.ID())
				}

				// Emit plugin initializing event
				m.emitPluginEvent(p.ID(), events.EventPluginInitializing, map[string]any{
					"plugin_name":   p.Name(),
					"plugin_id":     p.ID(),
					"step":          "initialize",
					"timeout_ms":    initTimeout.Milliseconds(),
					"ctx_aware":     ctxAware,
					"deadline_unix": time.Now().Add(initTimeout).Unix(),
				})

				initStart := time.Now()
				if err := m.safeInitPlugin(p, rt, initTimeout); err != nil {
					// Cleanup any partially registered resources to avoid leaks on init failure
					_ = m.runtime.CleanupResources(p.ID())
					// Emit error event
					m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
						"error":       err.Error(),
						"plugin_name": p.Name(),
						"operation":   "initialize",
						"took_ms":     time.Since(initStart).Milliseconds(),
						"timeout":     errors.Is(err, context.DeadlineExceeded),
						"ctx_aware":   ctxAware,
					})
					errCh <- fmt.Errorf("failed to initialize plugin %s: %w", p.ID(), err)
					return
				}

				// Emit plugin initialized event
				m.emitPluginEvent(p.ID(), events.EventPluginInitialized, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
					"took_ms":     time.Since(initStart).Milliseconds(),
					"ctx_aware":   ctxAware,
				})

				// Debug info for context awareness (only show in debug mode)
				if !supportsLC {
					log.Debugf("plugin %s (%s) does not implement LifecycleWithContext; step=start. Please implement StartContext/StopContext/InitializeContext", p.Name(), p.ID())
				} else if !ctxAware {
					log.Debugf("plugin %s (%s) implements LifecycleWithContext but not truly context-aware; step=start. Ensure methods observe ctx and implement ContextAwareness.IsContextAware()=true", p.Name(), p.ID())
				}
				// Emit plugin starting event
				m.emitPluginEvent(p.ID(), events.EventPluginStarting, map[string]any{
					"plugin_name":   p.Name(),
					"plugin_id":     p.ID(),
					"step":          "start",
					"timeout_ms":    startTimeout.Milliseconds(),
					"ctx_aware":     ctxAware,
					"deadline_unix": time.Now().Add(startTimeout).Unix(),
				})

				startStart := time.Now()
				if err := m.safeStartPlugin(p, startTimeout); err != nil {
					_ = m.runtime.CleanupResources(p.ID())
					// Emit error event
					m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
						"error":       err.Error(),
						"plugin_name": p.Name(),
						"operation":   "start",
						"took_ms":     time.Since(startStart).Milliseconds(),
						"timeout":     errors.Is(err, context.DeadlineExceeded),
						"ctx_aware":   ctxAware,
					})
					errCh <- fmt.Errorf("failed to start plugin %s: %w", p.ID(), err)
					return
				}

				// Emit plugin started event
				m.emitPluginEvent(p.ID(), events.EventPluginStarted, map[string]any{
					"plugin_name": p.Name(),
					"plugin_id":   p.ID(),
					"took_ms":     time.Since(startStart).Milliseconds(),
					"ctx_aware":   ctxAware,
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
			// Add timeout range validation
			if parsed < 1*time.Second {
				log.Warnf("init_timeout too short (%v), using minimum 1s", parsed)
				return 1 * time.Second
			}
			if parsed > 60*time.Second {
				log.Warnf("init_timeout too long (%v), using maximum 60s", parsed)
				return 60 * time.Second
			}
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
			// Add timeout range validation
			if parsed < 1*time.Second {
				log.Warnf("start_timeout too short (%v), using minimum 1s", parsed)
				return 1 * time.Second
			}
			if parsed > 60*time.Second {
				log.Warnf("start_timeout too long (%v), using maximum 60s", parsed)
				return 60 * time.Second
			}
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
			// Add timeout range validation
			if parsed < 1*time.Second {
				log.Warnf("stop_timeout too short (%v), using minimum 1s", parsed)
				return 1 * time.Second
			}
			if parsed > 120*time.Second {
				log.Warnf("stop_timeout too long (%v), using maximum 120s", parsed)
				return 120 * time.Second
			}
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
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 增强panic信息
				stackTrace := make([]byte, 4096)
				stackLen := runtime.Stack(stackTrace, false)
				log.Errorf("Panic in Initialize of %s: %v\nStack trace:\n%s", p.ID(), r, stackTrace[:stackLen])
				done <- fmt.Errorf("panic in Initialize of %s: %v", p.ID(), r)
			}
		}()
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			done <- lc.InitializeContext(ctx, p, rt)
			return
		}
		done <- p.Initialize(p, rt)
	}()
	select {
	case err := <-done:
		// 记录执行时间
		duration := time.Since(t0)
		if duration > timeout/2 {
			log.Warnf("Plugin %s initialize took %v (50%% of timeout %v)", p.Name(), duration, timeout)
		}
		return err
	case <-ctx.Done():
		// Mark plugin as failed to avoid lingering in an initializing state
		setPluginStatusIfSupported(p, plugins.StatusFailed)
		// Watchdog: log if the underlying goroutine returns late or keeps running
		go func(start time.Time) {
			select {
			case err := <-done:
				log.Warnf("plugin %s (%s) initialize returned after deadline; delay_ms=%d, err=%v", p.Name(), p.ID(), time.Since(start).Milliseconds(), err)
			case <-time.After(30 * time.Second):
				log.Errorf("plugin %s (%s) initialize still running 30s after timeout; potential goroutine leak", p.Name(), p.ID())
			}
		}(t0)
		return fmt.Errorf("initialize timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
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
	// Derive an approximate status from event type for better observability
	var status plugins.PluginStatus
	switch eventType {
	case events.EventPluginInitializing:
		status = plugins.StatusInitializing
	case events.EventPluginInitialized:
		status = plugins.StatusInactive
	case events.EventPluginStarting:
		status = plugins.StatusInitializing
	case events.EventPluginStarted:
		status = plugins.StatusActive
	case events.EventPluginStopping:
		status = plugins.StatusStopping
	case events.EventPluginStopped:
		status = plugins.StatusTerminated
	case events.EventErrorOccurred:
		status = plugins.StatusFailed
	default:
		status = plugins.StatusInactive
	}
	pluginEvent := plugins.PluginEvent{
		Type:      pluginEventType,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "lifecycle",
		PluginID:  pluginID,
		Status:    status,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	// Emit through runtime
	m.runtime.EmitEvent(pluginEvent)
}

// setPluginStatusIfSupported attempts to set plugin status via optional interface
// without introducing a hard dependency on a setter in the core interfaces.
func setPluginStatusIfSupported(p plugins.Plugin, status plugins.PluginStatus) {
	type statusSetter interface{ SetStatus(plugins.PluginStatus) }
	if s, ok := p.(statusSetter); ok {
		s.SetStatus(status)
	}
}

// safeStartPlugin safely calls Start with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeStartPlugin(p plugins.Plugin, timeout time.Duration) error {
	if p == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in Start of %s: %v", p.ID(), r)
			}
		}()
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			done <- lc.StartContext(ctx, p)
			return
		}
		done <- p.Start(p)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Mark plugin as failed to avoid lingering in a starting state
		setPluginStatusIfSupported(p, plugins.StatusFailed)
		// Watchdog: log if the underlying goroutine returns late or keeps running
		go func(start time.Time) {
			select {
			case err := <-done:
				log.Warnf("plugin %s (%s) start returned after deadline; delay_ms=%d, err=%v", p.Name(), p.ID(), time.Since(start).Milliseconds(), err)
			case <-time.After(30 * time.Second):
				log.Errorf("plugin %s (%s) start still running 30s after timeout; potential goroutine leak", p.Name(), p.ID())
			}
		}(t0)
		return fmt.Errorf("start timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
	}
}

// safeStopPlugin safely calls Stop with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeStopPlugin(p plugins.Plugin, timeout time.Duration) error {
	if p == nil {
		return nil
	}

	// Detect whether plugin supports context-aware lifecycle and truly honors ctx
	supportsLC := false
	if _, ok := p.(plugins.LifecycleWithContext); ok {
		supportsLC = true
	}
	ctxAware := false
	if supportsLC {
		if ca, ok := p.(plugins.ContextAwareness); ok && ca.IsContextAware() {
			ctxAware = true
		}
	}

	// Debug info for context awareness (only show in debug mode)
	if !supportsLC {
		log.Debugf("plugin %s (%s) does not implement LifecycleWithContext; step=stop. Please implement StartContext/StopContext/InitializeContext", p.Name(), p.ID())
	} else if !ctxAware {
		log.Debugf("plugin %s (%s) implements LifecycleWithContext but not truly context-aware; step=stop. Ensure methods observe ctx and implement ContextAwareness.IsContextAware()=true", p.Name(), p.ID())
	}

	// Emit plugin stopping event
	m.emitPluginEvent(p.ID(), events.EventPluginStopping, map[string]any{
		"plugin_name":   p.Name(),
		"plugin_id":     p.ID(),
		"step":          "stop",
		"timeout_ms":    timeout.Milliseconds(),
		"ctx_aware":     ctxAware,
		"deadline_unix": time.Now().Add(timeout).Unix(),
	})

	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// convert panic to error
				done <- fmt.Errorf("panic in Stop of %s: %v", p.Name(), r)
			}
		}()
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			done <- lc.StopContext(ctx, p)
			return
		}
		done <- p.Stop(p)
	}()
	select {
	case err := <-done:
		if err != nil {
			// Emit error event
			m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
				"error":       err.Error(),
				"plugin_name": p.Name(),
				"operation":   "stop",
				"took_ms":     time.Since(t0).Milliseconds(),
				"ctx_aware":   ctxAware,
			})
		} else {
			// Emit plugin stopped event
			m.emitPluginEvent(p.ID(), events.EventPluginStopped, map[string]any{
				"plugin_name": p.Name(),
				"plugin_id":   p.ID(),
				"took_ms":     time.Since(t0).Milliseconds(),
				"ctx_aware":   ctxAware,
			})
		}
		return err
	case <-ctx.Done():
		timeoutErr := fmt.Errorf("stop timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
		// Mark plugin as failed to reflect abnormal termination state
		setPluginStatusIfSupported(p, plugins.StatusFailed)
		// Emit timeout error event
		m.emitPluginEvent(p.ID(), events.EventErrorOccurred, map[string]any{
			"error":       timeoutErr.Error(),
			"plugin_name": p.Name(),
			"operation":   "stop",
			"took_ms":     time.Since(t0).Milliseconds(),
			"timeout":     true,
			"ctx_aware":   ctxAware,
		})
		// Watchdog: log if the underlying goroutine returns late or keeps running
		go func(start time.Time) {
			select {
			case err := <-done:
				log.Warnf("plugin %s (%s) stop returned after deadline; delay_ms=%d, err=%v", p.Name(), p.ID(), time.Since(start).Milliseconds(), err)
			case <-time.After(30 * time.Second):
				log.Errorf("plugin %s (%s) stop still running 30s after timeout; potential goroutine leak", p.Name(), p.ID())
			}
		}(t0)
		return timeoutErr
	}
}
