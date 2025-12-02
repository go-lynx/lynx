// Package lynx provides the core application framework for building microservices.
//
// This file (lifecycle.go) contains plugin lifecycle operations including:
//   - Plugin initialization with timeout and error recovery
//   - Plugin startup with parallel execution and rollback
//   - Plugin shutdown with graceful termination
//   - Context-aware lifecycle support for cancellation
package lynx

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/observability/metrics"
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
					// Set plugin status to failed before cleanup
					setPluginStatusIfSupported(p, plugins.StatusFailed)
					// Cleanup any partially registered resources to avoid leaks on init failure
					if cleanupErr := m.runtime.CleanupResources(p.ID()); cleanupErr != nil {
						log.Warnf("Failed to cleanup resources for plugin %s after init failure: %v", p.ID(), cleanupErr)
					}
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

				// Set plugin status to inactive after successful initialization
				setPluginStatusIfSupported(p, plugins.StatusInactive)
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
					// Set plugin status to failed before cleanup
					setPluginStatusIfSupported(p, plugins.StatusFailed)
					if cleanupErr := m.runtime.CleanupResources(p.ID()); cleanupErr != nil {
						log.Warnf("Failed to cleanup resources for plugin %s after start failure: %v", p.ID(), cleanupErr)
					}
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

				// Set plugin status to active after successful start
				setPluginStatusIfSupported(p, plugins.StatusActive)
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
			var allErrors []error
			for e := range errCh {
				if firstErr == nil {
					firstErr = e
				}
				allErrors = append(allErrors, e)
				log.Errorf("plugin start error: %v", e)
			}

			// Enhanced rollback with detailed tracking
			log.Errorf("Starting rollback for %d started plugins due to %d errors", len(started), len(allErrors))
			rollbackStart := time.Now()
			timeout := m.getStopTimeout()

			type rollbackResult struct {
				pluginName string
				stopErr    error
				cleanupErr error
				success    bool
			}
			var rollbackResults []rollbackResult
			var rollbackMu sync.Mutex

			// Rollback in reverse order (LIFO) to respect dependencies
			for i := len(started) - 1; i >= 0; i-- {
				p := started[i]
				if p == nil {
					continue
				}

				result := rollbackResult{
					pluginName: p.Name(),
					success:    true,
				}

				// Stop plugin with timeout
				stopStart := time.Now()
				if err := m.safeStopPlugin(p, timeout); err != nil {
					result.stopErr = err
					result.success = false
					log.Errorf("rollback stop failed for plugin %s (%s): %v (took %v)",
						p.Name(), p.ID(), err, time.Since(stopStart))
				} else {
					log.Infof("rollback stop succeeded for plugin %s (%s) (took %v)",
						p.Name(), p.ID(), time.Since(stopStart))
				}

				// Cleanup resources with timeout
				cleanupStart := time.Now()
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), timeout)
				defer cleanupCancel() // Ensure cleanup
				cleanupDone := make(chan error, 1)
				go func() {
					cleanupDone <- m.runtime.CleanupResources(p.ID())
				}()
				select {
				case err := <-cleanupDone:
					if err != nil {
						result.cleanupErr = err
						result.success = false
						log.Errorf("rollback cleanup failed for plugin %s (%s): %v (took %v)",
							p.Name(), p.ID(), err, time.Since(cleanupStart))
					} else {
						log.Infof("rollback cleanup succeeded for plugin %s (%s) (took %v)",
							p.Name(), p.ID(), time.Since(cleanupStart))
					}
				case <-cleanupCtx.Done():
					result.cleanupErr = cleanupCtx.Err()
					result.success = false
					log.Errorf("rollback cleanup timeout for plugin %s (%s) after %v",
						p.Name(), p.ID(), timeout)
				}

				// Remove from plugin instances
				m.pluginInstances.Delete(p.Name())

				rollbackMu.Lock()
				rollbackResults = append(rollbackResults, result)
				rollbackMu.Unlock()
			}

			// Log rollback summary
			rollbackDuration := time.Since(rollbackStart)
			successCount := 0
			for _, r := range rollbackResults {
				if r.success && r.stopErr == nil && r.cleanupErr == nil {
					successCount++
				}
			}

			log.Errorf("Rollback completed: total=%d, successful=%d, failed=%d, duration=%v",
				len(rollbackResults), successCount, len(rollbackResults)-successCount, rollbackDuration)

			// Emit rollback event with detailed statistics
			if rt := m.runtime; rt != nil {
				rt.EmitPluginEvent("plugin-manager", "rollback.completed", map[string]any{
					"total_plugins":    len(rollbackResults),
					"successful":       successCount,
					"failed":           len(rollbackResults) - successCount,
					"duration_ms":      rollbackDuration.Milliseconds(),
					"initial_errors":   len(allErrors),
					"rollback_results": rollbackResults,
				})
			}

			// If rollback had failures, return combined error
			if successCount < len(rollbackResults) {
				return fmt.Errorf("plugin startup failed with %d errors, rollback had %d failures: %w",
					len(allErrors), len(rollbackResults)-successCount, firstErr)
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

// getUnloadTotalTimeout returns total timeout for unloading all plugins, default 60s.
// This prevents the entire shutdown process from hanging indefinitely.
func (m *DefaultPluginManager[T]) getUnloadTotalTimeout() time.Duration {
	d := 60 * time.Second
	if m == nil || m.config == nil {
		return d
	}
	var confStr string
	if err := m.config.Value("lynx.plugins.unload_total_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			// Add timeout range validation
			if parsed < 10*time.Second {
				log.Warnf("unload_total_timeout too short (%v), using minimum 10s", parsed)
				return 10 * time.Second
			}
			if parsed > 300*time.Second {
				log.Warnf("unload_total_timeout too long (%v), using maximum 300s", parsed)
				return 300 * time.Second
			}
			return parsed
		}
	}
	return d
}

// getUnloadParallelism returns parallelism for unloading plugins, default 4.
// Lower than start parallelism to avoid overwhelming the system during shutdown.
func (m *DefaultPluginManager[T]) getUnloadParallelism() int {
	d := 4
	if m == nil || m.config == nil {
		return d
	}
	var v int
	if err := m.config.Value("lynx.plugins.unload_parallelism").Scan(&v); err == nil {
		if v > 0 && v <= 16 {
			return v
		}
		if v > 16 {
			log.Warnf("unload_parallelism too high (%d), using maximum 16", v)
			return 16
		}
	}
	return d
}

// safeInitPlugin safely calls Initialize with timeout and panic protection.
// Fixed: Simplified goroutine nesting, using context for unified lifecycle management
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

	// Use buffered channel to prevent goroutine blocking
	done := make(chan error, 1)

	// Simplified: Single goroutine with proper context management
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Enhance panic details
				stackTrace := make([]byte, 4096)
				stackLen := runtime.Stack(stackTrace, false)
				log.Errorf("Panic in Initialize of %s: %v\nStack trace:\n%s", p.ID(), r, stackTrace[:stackLen])
				select {
				case done <- fmt.Errorf("panic in Initialize of %s: %v", p.ID(), r):
				case <-ctx.Done():
					// Context cancelled, ignore result
				}
			}
		}()

		// Check context cancellation before starting
		if ctx.Err() != nil {
			select {
			case done <- fmt.Errorf("initialize cancelled before start for plugin %s: %w", p.Name(), ctx.Err()):
			case <-ctx.Done():
				// Context cancelled, ignore result
			}
			return
		}

		var err error
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			// Context-aware plugin - directly call with context
			err = lc.InitializeContext(ctx, p, rt)
		} else {
			// For non-context-aware plugins, run in a separate goroutine with cancellation monitoring
			// Fixed: Add proper goroutine lifecycle management to prevent leaks
			errCh := make(chan error, 1)
			goroutineDone := make(chan struct{}, 1)

			// Start plugin initialization in goroutine
			go func() {
				defer func() {
					// Signal goroutine completion
					select {
					case goroutineDone <- struct{}{}:
					default:
					}
					// Recover from panic
					if r := recover(); r != nil {
						stackTrace := make([]byte, 4096)
						stackLen := runtime.Stack(stackTrace, false)
						log.Errorf("Panic in Initialize of %s: %v\nStack trace:\n%s", p.ID(), r, stackTrace[:stackLen])
						select {
						case errCh <- fmt.Errorf("panic in Initialize of %s: %v", p.ID(), r):
						default:
						}
					}
				}()
				// Execute initialization
				errCh <- p.Initialize(p, rt)
			}()

			// Wait for either completion or cancellation
			select {
			case err = <-errCh:
				// Initialization completed - wait briefly for goroutine to signal completion
				select {
				case <-goroutineDone:
					// Goroutine completed normally
				case <-time.After(100 * time.Millisecond):
					// Timeout waiting, but result is already received
				}
			case <-ctx.Done():
				// Timeout or cancellation - plugin may still be running
				err = fmt.Errorf("initialize timeout for plugin %s: %w", p.Name(), context.DeadlineExceeded)
				log.Warnf("plugin %s (%s) initialize timed out; plugin may still be running", p.Name(), p.ID())

				// Wait for goroutine to complete with a short timeout
				// This helps detect if the plugin respects cancellation signals
				// Use a shorter timeout to prevent blocking shutdown
				cleanupTimeout := 200 * time.Millisecond
				select {
				case <-goroutineDone:
					// Check if there's a result (plugin completed after timeout)
					select {
					case lateErr := <-errCh:
						log.Warnf("plugin %s (%s) initialize completed after timeout with error: %v",
							p.Name(), p.ID(), lateErr)
					default:
					}
				case <-time.After(cleanupTimeout):
					// Goroutine did not complete within timeout
					// This indicates the plugin may not be respecting cancellation
					log.Warnf("plugin %s (%s) initialize goroutine did not complete within cleanup timeout (%v); "+
						"plugin may not be respecting cancellation signals. This may cause goroutine leak.",
						p.Name(), p.ID(), cleanupTimeout)
					// Force cleanup: try to drain the channel to prevent blocking
					select {
					case <-errCh:
					default:
					}
				}
			}
		}

		// Send result (non-blocking)
		select {
		case done <- err:
		case <-ctx.Done():
			// Context cancelled, ignore result
		}
	}()

	// Wait for result or timeout
	select {
	case err := <-done:
		// Record execution duration
		duration := time.Since(t0)
		if duration > timeout/2 {
			log.Warnf("Plugin %s initialize took %v (50%% of timeout %v)", p.Name(), duration, timeout)
		}
		return err
	case <-ctx.Done():
		// Mark plugin as failed to avoid lingering in an initializing state
		setPluginStatusIfSupported(p, plugins.StatusFailed)
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
// Fixed: Simplified goroutine nesting, using context for unified lifecycle management
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

	// Use buffered channel to prevent goroutine blocking
	done := make(chan error, 1)

	// Simplified: Single goroutine with proper context management
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case done <- fmt.Errorf("panic in Start of %s: %v", p.ID(), r):
				case <-ctx.Done():
					// Context cancelled, ignore result
				}
			}
		}()

		// Check context cancellation before starting
		if ctx.Err() != nil {
			select {
			case done <- fmt.Errorf("start cancelled before execution for plugin %s: %w", p.Name(), ctx.Err()):
			case <-ctx.Done():
				// Context cancelled, ignore result
			}
			return
		}

		var err error
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			// Context-aware plugin - directly call with context
			err = lc.StartContext(ctx, p)
		} else {
			// For non-context-aware plugins, run in a separate goroutine with cancellation monitoring
			// Fixed: Add proper goroutine lifecycle management to prevent leaks
			errCh := make(chan error, 1)
			goroutineDone := make(chan struct{}, 1)

			// Start plugin start in goroutine
			go func() {
				defer func() {
					// Signal goroutine completion
					select {
					case goroutineDone <- struct{}{}:
					default:
					}
					// Recover from panic
					if r := recover(); r != nil {
						select {
						case errCh <- fmt.Errorf("panic in Start of %s: %v", p.ID(), r):
						default:
						}
					}
				}()
				// Execute start
				errCh <- p.Start(p)
			}()

			// Wait for either completion or cancellation
			select {
			case err = <-errCh:
				// Start completed - wait briefly for goroutine to signal completion
				select {
				case <-goroutineDone:
					// Goroutine completed normally
				case <-time.After(100 * time.Millisecond):
					// Timeout waiting, but result is already received
				}
			case <-ctx.Done():
				// Timeout or cancellation - plugin may still be running
				err = fmt.Errorf("start timeout for plugin %s: %w", p.Name(), context.DeadlineExceeded)
				log.Warnf("plugin %s (%s) start timed out; plugin may still be running", p.Name(), p.ID())

				// Wait for goroutine to complete with a short timeout
				// Use a shorter timeout to prevent blocking shutdown
				cleanupTimeout := 200 * time.Millisecond
				select {
				case <-goroutineDone:
					// Check if there's a result (plugin completed after timeout)
					select {
					case lateErr := <-errCh:
						log.Warnf("plugin %s (%s) start completed after timeout with error: %v",
							p.Name(), p.ID(), lateErr)
					default:
					}
				case <-time.After(cleanupTimeout):
					// Goroutine did not complete within timeout
					log.Warnf("plugin %s (%s) start goroutine did not complete within cleanup timeout (%v); "+
						"plugin may not be respecting cancellation signals. This may cause goroutine leak.",
						p.Name(), p.ID(), cleanupTimeout)
					// Force cleanup: try to drain the channel to prevent blocking
					select {
					case <-errCh:
					default:
					}
				}
			}
		}

		// Send result (non-blocking)
		select {
		case done <- err:
		case <-ctx.Done():
			// Context cancelled, ignore result
		}
	}()

	// Wait for result or timeout
	select {
	case err := <-done:
		// Record execution duration
		duration := time.Since(t0)
		if duration > timeout/2 {
			log.Warnf("Plugin %s start took %v (50%% of timeout %v)", p.Name(), duration, timeout)
		}
		return err
	case <-ctx.Done():
		// Mark plugin as failed to avoid lingering in a starting state
		setPluginStatusIfSupported(p, plugins.StatusFailed)
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

	// Use buffered channel to prevent goroutine blocking
	done := make(chan error, 1)

	// Simplified: Single goroutine with proper context management
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// convert panic to error
				select {
				case done <- fmt.Errorf("panic in Stop of %s: %v", p.Name(), r):
				case <-ctx.Done():
					// Context cancelled, ignore result
				}
			}
		}()

		// Check context cancellation before starting
		if ctx.Err() != nil {
			select {
			case done <- fmt.Errorf("stop cancelled before execution for plugin %s: %w", p.Name(), ctx.Err()):
			case <-ctx.Done():
				// Context cancelled, ignore result
			}
			return
		}

		var err error
		if lc, ok := p.(plugins.LifecycleWithContext); ok {
			// Context-aware plugin - directly call with context
			err = lc.StopContext(ctx, p)
		} else {
			// For non-context-aware plugins, run in a separate goroutine with cancellation monitoring
			// Fixed: Add proper goroutine lifecycle management to prevent leaks
			errCh := make(chan error, 1)
			goroutineDone := make(chan struct{}, 1)

			// Start plugin stop in goroutine
			go func() {
				defer func() {
					// Signal goroutine completion
					select {
					case goroutineDone <- struct{}{}:
					default:
					}
					// Recover from panic
					if r := recover(); r != nil {
						select {
						case errCh <- fmt.Errorf("panic in Stop of %s: %v", p.Name(), r):
						default:
						}
					}
				}()
				// Execute stop
				errCh <- p.Stop(p)
			}()

			// Wait for either completion or cancellation
			select {
			case err = <-errCh:
				// Stop completed - wait briefly for goroutine to signal completion
				select {
				case <-goroutineDone:
					// Goroutine completed normally
				case <-time.After(100 * time.Millisecond):
					// Timeout waiting, but result is already received
				}
			case <-ctx.Done():
				// Timeout or cancellation - plugin may still be running
				err = fmt.Errorf("stop timeout for plugin %s: %w", p.Name(), context.DeadlineExceeded)
				log.Warnf("plugin %s (%s) stop timed out; plugin may still be running", p.Name(), p.ID())

				// Wait for goroutine to complete with a short timeout
				// Use a shorter timeout to prevent blocking shutdown
				cleanupTimeout := 200 * time.Millisecond
				select {
				case <-goroutineDone:
					// Check if there's a result (plugin completed after timeout)
					select {
					case lateErr := <-errCh:
						log.Warnf("plugin %s (%s) stop completed after timeout with error: %v",
							p.Name(), p.ID(), lateErr)
					default:
					}
				case <-time.After(cleanupTimeout):
					// Goroutine did not complete within timeout
					log.Warnf("plugin %s (%s) stop goroutine did not complete within cleanup timeout (%v); "+
						"plugin may not be respecting cancellation signals. This may cause goroutine leak.",
						p.Name(), p.ID(), cleanupTimeout)
					// Force cleanup: try to drain the channel to prevent blocking
					select {
					case <-errCh:
					default:
					}
				}
			}
		}

		// Send result (non-blocking)
		select {
		case done <- err:
		case <-ctx.Done():
			// Context cancelled, ignore result
		}
	}()

	// Wait for result or timeout
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
		return timeoutErr
	}
}
