// Package lynx provides the core application framework for building microservices.
//
// This file (ops.go) contains plugin manager operations including:
//   - LoadPlugins: Load and start all configured plugins
//   - UnloadPlugins: Gracefully stop and unload all plugins
//   - LoadPluginsByName: Load specific plugins by name
//   - UnloadPluginsByName: Unload specific plugins by name
//   - StopPlugin: Stop a single plugin
//   - Resource management and monitoring utilities
package lynx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/subscribe"
)

// LoadPlugins loads and starts all plugins.
func (m *DefaultPluginManager[T]) LoadPlugins(conf config.Config) error {
	m.SetConfig(conf)

	preparedPlugins, err := m.PreparePlug(conf)
	if err != nil {
		return fmt.Errorf("failed to prepare plugins: %w", err)
	}
	if len(preparedPlugins) == 0 {
		return fmt.Errorf("no plugins prepared")
	}

	sortedPlugins, err := m.TopologicalSort(preparedPlugins)
	if err != nil {
		// Provide detailed error information for debugging
		return fmt.Errorf("failed to sort plugins (dependency resolution failed): %w. "+
			"This usually indicates circular dependencies or missing required dependencies. "+
			"Please check plugin dependency declarations", err)
	}

	if err := m.loadSortedPluginsByLevel(sortedPlugins); err != nil {
		return err
	}

	// Build gRPC subscriptions after plugins are loaded (control plane plugin must be started)
	// This ensures service discovery is available
	if Lynx() != nil && Lynx().bootConfig != nil && Lynx().bootConfig.Lynx != nil && Lynx().bootConfig.Lynx.Subscriptions != nil {
		subs := Lynx().bootConfig.Lynx.Subscriptions
		hasGrpcSubs := subs != nil && len(subs.Grpc) > 0
		if hasGrpcSubs {
			controlPlane := Lynx().GetControlPlane()
			if controlPlane == nil {
				m.UnloadPlugins()
				return fmt.Errorf("grpc subscriptions configured but control plane is not available (install a control plane plugin)")
			}
			disc := controlPlane.NewServiceDiscovery()
			if disc == nil {
				m.UnloadPlugins()
				return fmt.Errorf("grpc subscriptions configured but service discovery is not available")
			}
			routerFactory := func(service string) selector.NodeFilter {
				return controlPlane.NewNodeRouter(service)
			}
			conns, err := subscribe.BuildGrpcSubscriptions(Lynx().bootConfig.Lynx.Subscriptions, disc, routerFactory)
			if err != nil {
				m.UnloadPlugins()
				return fmt.Errorf("build grpc subscriptions failed: %w", err)
			}
			app := Lynx()
			if app != nil {
				app.grpcSubsMu.Lock()
				app.grpcSubs = conns
				app.grpcSubsMu.Unlock()
			}
		}
	}

	return nil
}

// UnloadPlugins stops and unloads all plugins with overall timeout protection.
// Optimized for stability: adds total timeout, parallel unloading, and better error handling.
func (m *DefaultPluginManager[T]) UnloadPlugins() {
	if m == nil || len(m.pluginList) == 0 {
		return
	}

	// Emit plugin manager shutdown event
	m.emitPluginManagerShutdownEvent()

	// Get timeouts and parallelism settings
	perPluginTimeout := m.getStopTimeout()
	totalTimeout := m.getUnloadTotalTimeout()
	parallelism := m.getUnloadParallelism()

	// Create overall context with total timeout to prevent indefinite blocking
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	// Track unload start time for monitoring
	unloadStart := time.Now()

	var ordered []plugins.Plugin
	sorted, err := m.TopologicalSort(m.pluginList)
	if err != nil {
		// Topological sort failed - use dependency-aware unload order (best effort)
		log.Errorf("topological sort failed during unload: %v", err)
		log.Errorf("Using best-effort unload order (dependents before dependencies)")
		ordered = m.UnloadOrder(m.pluginList)
		if len(ordered) == 0 {
			ordered = make([]plugins.Plugin, len(m.pluginList))
			copy(ordered, m.pluginList)
		}
	} else {
		tmp := make([]plugins.Plugin, 0, len(sorted))
		for _, w := range sorted {
			if w.Plugin != nil {
				tmp = append(tmp, w.Plugin)
			}
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			ordered = append(ordered, tmp[i])
		}
	}

	// Use semaphore to control parallelism
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Optimized: Pre-allocate slice capacity to avoid frequent reallocations
	unloadErrors := make([]string, 0, len(ordered))
	timeoutReached := false

	// Fix Bug 1: Track which plugins are being cleaned up to avoid race conditions
	// Use sync.Map to safely track plugins that have started cleanup
	cleaningUp := sync.Map{} // map[pluginID]bool

	// Unload plugins with controlled parallelism
	for _, plugin := range ordered {
		p := plugin
		if p == nil {
			continue
		}

		// Check if overall timeout has been reached
		select {
		case <-ctx.Done():
			log.Errorf("UnloadPlugins: overall timeout (%v) reached, forcing shutdown of remaining %d plugins",
				totalTimeout, len(ordered))
			timeoutReached = true
			// Fix Bug 1: Force cleanup of remaining plugins that haven't started cleanup yet
			// Only clean up plugins that are not already being cleaned up by their goroutines
			for _, remaining := range ordered {
				if remaining != nil {
					pluginID := remaining.ID()
					// Check if this plugin is already being cleaned up
					if _, alreadyCleaning := cleaningUp.LoadOrStore(pluginID, true); alreadyCleaning {
						// Plugin is already being cleaned up by its goroutine, skip forced cleanup
						log.Debugf("Skipping forced cleanup for plugin %s: already being cleaned up", remaining.Name())
						continue
					}
					// This plugin hasn't started cleanup yet, force cleanup it
					m.pluginInstances.Delete(remaining.Name())
					if cleanupErr := m.runtime.CleanupResources(pluginID); cleanupErr != nil {
						log.Warnf("Forced cleanup failed for plugin %s: %v", remaining.Name(), cleanupErr)
					}
				}
			}
			break
		default:
		}

		if timeoutReached {
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(plugin plugins.Plugin) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Fix Bug 1: Mark this plugin as being cleaned up to prevent double cleanup
			pluginID := plugin.ID()
			if _, alreadyCleaning := cleaningUp.LoadOrStore(pluginID, true); alreadyCleaning {
				// Another goroutine (forced cleanup) is already cleaning this up, skip
				log.Debugf("Skipping cleanup for plugin %s: already being cleaned up by forced cleanup", plugin.Name())
				return
			}

			// Create per-plugin context that respects overall timeout
			pluginCtx, pluginCancel := context.WithTimeout(ctx, perPluginTimeout)
			defer pluginCancel()

			// Emit plugin unloading event
			m.emitPluginUnloadEvent(pluginID, plugin.Name())

			var stopErr, cleanupErr error

			// Stop plugin with timeout protection
			stopDone := make(chan error, 1)
			go func() {
				stopDone <- m.safeStopPlugin(plugin, perPluginTimeout)
			}()

			select {
			case err := <-stopDone:
				if err != nil {
					stopErr = err
					log.Errorf("Failed to unload plugin %s: %v", plugin.Name(), err)
					m.emitPluginErrorEvent(pluginID, plugin.Name(), "unload", err)
				}
			case <-pluginCtx.Done():
				stopErr = fmt.Errorf("plugin stop timeout after %v", perPluginTimeout)
				log.Errorf("Plugin %s stop timed out, forcing cleanup", plugin.Name())
				m.emitPluginErrorEvent(pluginID, plugin.Name(), "unload", stopErr)
			}

			// Cleanup resources with timeout protection
			// Fix Bug 1: Only cleanup if we successfully marked this plugin as being cleaned up
			cleanupDone := make(chan error, 1)
			go func() {
				cleanupDone <- m.runtime.CleanupResources(pluginID)
			}()

			select {
			case err := <-cleanupDone:
				if err != nil {
					cleanupErr = err
					log.Errorf("Failed to cleanup resources for plugin %s: %v", plugin.Name(), err)
					m.emitResourceCleanupErrorEvent(pluginID, plugin.Name(), err)
				}
			case <-pluginCtx.Done():
				cleanupErr = fmt.Errorf("resource cleanup timeout after %v", perPluginTimeout)
				log.Errorf("Plugin %s resource cleanup timed out", plugin.Name())
				m.emitResourceCleanupErrorEvent(pluginID, plugin.Name(), cleanupErr)
			}

			// Record unload failure if either stop or cleanup failed
			if stopErr != nil || cleanupErr != nil {
				m.recordUnloadFailure(plugin, stopErr, cleanupErr)
				mu.Lock()
				unloadErrors = append(unloadErrors, fmt.Sprintf("%s: stop=%v, cleanup=%v",
					plugin.Name(), stopErr, cleanupErr))
				mu.Unlock()
			}

			m.pluginInstances.Delete(plugin.Name())
			// Remove from cleaningUp map when done
			cleaningUp.Delete(pluginID)
		}(p)
	}

	// Wait for all unload operations to complete or timeout
	if !timeoutReached {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All plugins unloaded successfully
			unloadDuration := time.Since(unloadStart)
			if len(unloadErrors) > 0 {
				log.Warnf("UnloadPlugins completed in %v with %d errors: %v",
					unloadDuration, len(unloadErrors), unloadErrors)
			} else {
				log.Infof("UnloadPlugins completed successfully in %v", unloadDuration)
			}
		case <-ctx.Done():
			// Overall timeout reached
			log.Errorf("UnloadPlugins: overall timeout (%v) reached, some plugins may not have been properly unloaded",
				totalTimeout)
		}
	}

	// Cleanup plugin list
	m.mu.Lock()
	m.pluginList = nil
	m.mu.Unlock()
}

// LoadPluginsByName loads a subset of plugins by Name().
func (m *DefaultPluginManager[T]) LoadPluginsByName(conf config.Config, pluginNames []string) error {
	m.SetConfig(conf)

	preparedPlugins, err := m.PreparePlug(conf)
	if err != nil {
		return err
	}

	var targetPlugins []plugins.Plugin
	pluginMap := make(map[string]plugins.Plugin)
	for _, plugin := range preparedPlugins {
		pluginMap[plugin.Name()] = plugin
	}

	for _, name := range pluginNames {
		if plugin, exists := pluginMap[name]; exists {
			targetPlugins = append(targetPlugins, plugin)
		} else {
			return fmt.Errorf("plugin %s not found", name)
		}
	}

	sortedPlugins, err := m.TopologicalSort(targetPlugins)
	if err != nil {
		// Provide detailed error information for debugging
		return fmt.Errorf("failed to sort plugins (dependency resolution failed): %w. "+
			"This usually indicates circular dependencies or missing required dependencies. "+
			"Please check plugin dependency declarations", err)
	}

	if err := m.loadSortedPluginsByLevel(sortedPlugins); err != nil {
		return err
	}
	return nil
}

// UnloadPluginsByName unloads a subset of plugins by Name().
func (m *DefaultPluginManager[T]) UnloadPluginsByName(names []string) {
	if m == nil || len(names) == 0 {
		return
	}

	timeout := m.getStopTimeout()

	var subset []plugins.Plugin
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	m.pluginInstances.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		if _, wanted := nameSet[name]; !wanted {
			return true
		}
		if p, ok2 := value.(plugins.Plugin); ok2 && p != nil {
			subset = append(subset, p)
		}
		return true
	})

	if len(subset) == 0 {
		for _, n := range names {
			log.Infof("plugin %s not found, skip unload", n)
		}
		return
	}

	var ordered []plugins.Plugin
	sorted, err := m.TopologicalSort(subset)
	if err != nil {
		log.Errorf("topological sort failed for subset unload: %v", err)
		log.Errorf("Using best-effort unload order for subset")
		ordered = m.UnloadOrder(subset)
		if len(ordered) == 0 {
			ordered = subset
		}
	} else {
		tmp := make([]plugins.Plugin, 0, len(sorted))
		for _, w := range sorted {
			if w.Plugin != nil {
				tmp = append(tmp, w.Plugin)
			}
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			ordered = append(ordered, tmp[i])
		}
	}

	for _, p := range ordered {
		if p == nil {
			continue
		}

		// Emit plugin unloading event
		m.emitPluginUnloadEvent(p.ID(), p.Name())

		var stopErr, cleanupErr error
		if err := m.safeStopPlugin(p, timeout); err != nil {
			stopErr = err
			log.Errorf("Failed to unload plugin %s: %v", p.Name(), err)
			// Emit error event
			m.emitPluginErrorEvent(p.ID(), p.Name(), "unload", err)
		}
		if err := m.runtime.CleanupResources(p.ID()); err != nil {
			cleanupErr = err
			log.Errorf("Failed to cleanup resources for plugin %s: %v", p.Name(), err)
			// Emit resource cleanup error event
			m.emitResourceCleanupErrorEvent(p.ID(), p.Name(), err)
		}

		// Record unload failure if either stop or cleanup failed
		if stopErr != nil || cleanupErr != nil {
			m.recordUnloadFailure(p, stopErr, cleanupErr)
		}

		m.pluginInstances.Delete(p.Name())
	}

	m.mu.Lock()
	var newList []plugins.Plugin
	for _, item := range m.pluginList {
		if item != nil {
			if _, removed := nameSet[item.Name()]; !removed {
				newList = append(newList, item)
			}
		}
	}
	m.pluginList = newList
	m.mu.Unlock()
}

// StopPlugin stops a single plugin by Name().
func (m *DefaultPluginManager[T]) StopPlugin(pluginName string) error {
	plugin, exists := m.pluginInstances.Load(pluginName)
	if !exists {
		log.Infof("plugin %s not found, skip stop", pluginName)
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	p, ok := plugin.(plugins.Plugin)
	if !ok {
		return fmt.Errorf("invalid plugin instance for %s", pluginName)
	}
	if p == nil {
		// Instance is nil; cleanup by plugin name. Plugins should use consistent Name/ID so resources are findable.
		_ = m.runtime.CleanupResources(pluginName)
		m.pluginInstances.Delete(pluginName)
		return nil
	}

	// Emit plugin stopping event
	m.emitPluginUnloadEvent(p.ID(), p.Name())

	timeout := m.getStopTimeout()
	var stopErr, cleanupErr error
	if err := m.safeStopPlugin(p, timeout); err != nil {
		stopErr = err
		// Emit error event
		m.emitPluginErrorEvent(p.ID(), p.Name(), "stop", err)
	}
	if err := m.runtime.CleanupResources(p.ID()); err != nil {
		cleanupErr = err
		// Emit resource cleanup error event
		m.emitResourceCleanupErrorEvent(p.ID(), p.Name(), err)
	}

	// Record unload failure if either stop or cleanup failed
	if stopErr != nil || cleanupErr != nil {
		m.recordUnloadFailure(p, stopErr, cleanupErr)
		if stopErr != nil {
			return fmt.Errorf("failed to stop plugin %s: %w", pluginName, stopErr)
		}
		return fmt.Errorf("failed to cleanup resources for plugin %s: %w", pluginName, cleanupErr)
	}

	m.pluginInstances.Delete(pluginName)
	return nil
}

// GetResourceStats Resource helpers.
func (m *DefaultPluginManager[T]) GetResourceStats() map[string]any {
	return m.runtime.GetResourceStats()
}

func (m *DefaultPluginManager[T]) ListResources() []*plugins.ResourceInfo {
	return m.runtime.ListResources()
}

// ListPluginNames Public helpers for any TypedPluginManager.
func ListPluginNames(m TypedPluginManager) []string {
	if m == nil {
		return nil
	}
	type nameLister interface{ listPluginNamesInternal() []string }
	if l, ok := m.(nameLister); ok {
		return l.listPluginNamesInternal()
	}
	return nil
}

func Plugins(m TypedPluginManager) []plugins.Plugin {
	if m == nil {
		return nil
	}
	type pluginsLister interface{ listPluginsInternal() []plugins.Plugin }
	if l, ok := m.(pluginsLister); ok {
		return l.listPluginsInternal()
	}
	return nil
}

// emitPluginUnloadEvent emits a plugin unload event
func (m *DefaultPluginManager[T]) emitPluginUnloadEvent(pluginID, pluginName string) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventPluginStopping,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "lifecycle",
		PluginID:  pluginID,
		Status:    plugins.StatusInactive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   "unload",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitPluginErrorEvent emits a plugin error event
func (m *DefaultPluginManager[T]) emitPluginErrorEvent(pluginID, pluginName, operation string, err error) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityHigh,
		Source:    "plugin-manager",
		Category:  "error",
		PluginID:  pluginID,
		Status:    plugins.StatusFailed,
		Timestamp: time.Now().Unix(),
		Error:     err,
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   operation,
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitResourceCleanupErrorEvent emits a resource cleanup error event
func (m *DefaultPluginManager[T]) emitResourceCleanupErrorEvent(pluginID, pluginName string, err error) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventErrorOccurred,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "error",
		PluginID:  pluginID,
		Status:    plugins.StatusFailed,
		Timestamp: time.Now().Unix(),
		Error:     err,
		Metadata: map[string]any{
			"plugin_name": pluginName,
			"operation":   "resource_cleanup",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// emitPluginManagerShutdownEvent emits a plugin manager shutdown event
func (m *DefaultPluginManager[T]) emitPluginManagerShutdownEvent() {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventType("system.plugin_manager_shutdown"),
		Priority:  plugins.PriorityHigh,
		Source:    "plugin-manager",
		Category:  "system",
		PluginID:  "system",
		Status:    plugins.StatusInactive,
		Timestamp: time.Now().Unix(),
		Metadata: map[string]any{
			"operation": "shutdown",
			"reason":    "application_close",
		},
	}

	m.runtime.EmitEvent(pluginEvent)
}

// recordUnloadFailure records a plugin unload failure for monitoring
func (m *DefaultPluginManager[T]) recordUnloadFailure(p plugins.Plugin, stopErr, cleanupErr error) {
	if m == nil || p == nil {
		return
	}

	m.unloadFailuresMu.Lock()
	defer m.unloadFailuresMu.Unlock()

	// Limit the number of stored failures to prevent unbounded growth
	const maxFailures = 100
	if len(m.unloadFailures) >= maxFailures {
		// Remove oldest failure (FIFO)
		m.unloadFailures = m.unloadFailures[1:]
	}

	record := UnloadFailureRecord{
		PluginName:   p.Name(),
		PluginID:     p.ID(),
		FailureTime:  time.Now(),
		StopError:    stopErr,
		CleanupError: cleanupErr,
		RetryCount:   0, // Can be incremented if retry logic is added
	}

	m.unloadFailures = append(m.unloadFailures, record)
}

// GetUnloadFailures returns all recorded plugin unload failures
func (m *DefaultPluginManager[T]) GetUnloadFailures() []UnloadFailureRecord {
	if m == nil {
		return nil
	}

	m.unloadFailuresMu.RLock()
	defer m.unloadFailuresMu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]UnloadFailureRecord, len(m.unloadFailures))
	copy(result, m.unloadFailures)
	return result
}

// ClearUnloadFailures clears all recorded unload failures
func (m *DefaultPluginManager[T]) ClearUnloadFailures() {
	if m == nil {
		return
	}

	m.unloadFailuresMu.Lock()
	defer m.unloadFailuresMu.Unlock()
	m.unloadFailures = nil
}
