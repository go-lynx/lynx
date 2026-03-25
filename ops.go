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
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
)

type unloadPluginResult struct {
	pluginName   string
	stopErr      error
	cleanupErr   error
	shouldRecord bool
}

type unloadExecutionOptions struct {
	operation         string
	unregisterOnError bool
}

func formatUnloadErrorSummary(result unloadPluginResult) string {
	return fmt.Sprintf("%s: stop=%v, cleanup=%v", result.pluginName, result.stopErr, result.cleanupErr)
}

func (m *DefaultPluginManager[T]) recordUnloadResult(plugin plugins.Plugin, result unloadPluginResult) {
	if m == nil || plugin == nil || !result.shouldRecord {
		return
	}
	m.recordUnloadFailure(plugin, result.stopErr, result.cleanupErr)
}

func unloadResultError(pluginName string, result unloadPluginResult) error {
	if !result.shouldRecord {
		return nil
	}
	if result.stopErr != nil {
		return fmt.Errorf("failed to stop plugin %s: %w", pluginName, result.stopErr)
	}
	if result.cleanupErr != nil {
		return fmt.Errorf("failed to cleanup resources for plugin %s: %w", pluginName, result.cleanupErr)
	}
	return nil
}

func shouldStopPlugin(p plugins.Plugin) bool {
	if p == nil {
		return false
	}

	switch p.Status(p) {
	case plugins.StatusActive, plugins.StatusStopping, plugins.StatusInitializing, plugins.StatusSuspended:
		return true
	default:
		return false
	}
}

func (m *DefaultPluginManager[T]) resetPluginManagerStateAfterUnload() {
	m.mu.Lock()
	m.pluginList = nil
	m.managedPluginList = nil
	m.mu.Unlock()

	m.pluginIDs.Range(func(key, value any) bool {
		m.pluginIDs.Delete(key)
		return true
	})
	m.managedIDs.Range(func(key, value any) bool {
		m.managedIDs.Delete(key)
		return true
	})
	m.pluginInstances.Range(func(key, value any) bool {
		m.pluginInstances.Delete(key)
		return true
	})
	m.managedInstances.Range(func(key, value any) bool {
		m.managedInstances.Delete(key)
		return true
	})
}

func (m *DefaultPluginManager[T]) resolveOrderedUnloadTargets(targets []plugins.Plugin, description string) []plugins.Plugin {
	if len(targets) == 0 {
		return nil
	}

	var ordered []plugins.Plugin
	sorted, err := m.TopologicalSort(targets)
	if err != nil {
		log.Errorf("topological sort failed for %s: %v", description, err)
		log.Errorf("Using best-effort unload order for %s", description)
		ordered = m.UnloadOrder(targets)
		if len(ordered) == 0 {
			ordered = targets
		}
		return ordered
	}

	tmp := make([]plugins.Plugin, 0, len(sorted))
	for _, w := range sorted {
		if w.Plugin != nil {
			tmp = append(tmp, w.Plugin)
		}
	}
	for i := len(tmp) - 1; i >= 0; i-- {
		ordered = append(ordered, tmp[i])
	}
	return ordered
}

func (m *DefaultPluginManager[T]) buildUnloadBatches(targets []plugins.Plugin) ([][]plugins.Plugin, []plugins.Plugin) {
	if len(targets) == 0 {
		return nil, nil
	}

	sorted, err := m.TopologicalSort(targets)
	if err != nil {
		log.Errorf("topological sort failed during unload: %v", err)
		log.Errorf("Using best-effort unload order (dependents before dependencies)")
		ordered := m.UnloadOrder(targets)
		if len(ordered) == 0 {
			ordered = make([]plugins.Plugin, len(targets))
			copy(ordered, targets)
		}
		if len(ordered) == 0 {
			return nil, nil
		}
		return [][]plugins.Plugin{ordered}, ordered
	}

	levelGroups := make(map[int][]plugins.Plugin)
	maxLevel := 0
	for _, w := range sorted {
		if w.Plugin == nil {
			continue
		}
		levelGroups[w.level] = append(levelGroups[w.level], w.Plugin)
		if w.level > maxLevel {
			maxLevel = w.level
		}
	}

	var batches [][]plugins.Plugin
	var ordered []plugins.Plugin
	for level := maxLevel; level >= 0; level-- {
		batch := levelGroups[level]
		if len(batch) == 0 {
			continue
		}
		batches = append(batches, batch)
		ordered = append(ordered, batch...)
	}
	return batches, ordered
}

func (m *DefaultPluginManager[T]) forceCleanupRemainingPlugins(ordered []plugins.Plugin, cleaningUp *sync.Map) {
	for _, remaining := range ordered {
		if remaining == nil {
			continue
		}

		pluginID := remaining.ID()
		if _, alreadyCleaning := cleaningUp.LoadOrStore(pluginID, true); alreadyCleaning {
			log.Debugf("Skipping forced cleanup for plugin %s: already being cleaned up", remaining.Name())
			continue
		}

		m.unregisterPlugin(remaining)
		if cleanupErr := m.runtime.CleanupResources(pluginID); cleanupErr != nil {
			log.Warnf("Forced cleanup failed for plugin %s: %v", remaining.Name(), cleanupErr)
		}
	}
}

func (m *DefaultPluginManager[T]) appendUnloadError(unloadErrors *[]string, unloadErrorsMu *sync.Mutex, result unloadPluginResult) {
	if !result.shouldRecord {
		return
	}

	unloadErrorsMu.Lock()
	*unloadErrors = append(*unloadErrors, formatUnloadErrorSummary(result))
	unloadErrorsMu.Unlock()
}

func (m *DefaultPluginManager[T]) unloadBatch(
	ctx context.Context,
	batch []plugins.Plugin,
	perPluginTimeout time.Duration,
	parallelism int,
	cleaningUp *sync.Map,
	unloadErrors *[]string,
	unloadErrorsMu *sync.Mutex,
) bool {
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for _, plugin := range batch {
		p := plugin
		if p == nil {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(plugin plugins.Plugin) {
			defer wg.Done()
			defer func() { <-sem }()

			result, processed := m.unloadManagedPlugin(ctx, plugin, perPluginTimeout, cleaningUp)
			if !processed {
				return
			}
			if result.shouldRecord {
				m.recordUnloadResult(plugin, result)
				m.appendUnloadError(unloadErrors, unloadErrorsMu, result)
			}
		}(p)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return false
	case <-ctx.Done():
		return true
	}
}

// LoadPlugins loads and starts all plugins.
func (m *DefaultPluginManager[T]) LoadPlugins(conf config.Config) error {
	if m == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	m.SetConfig(conf)

	preparedPlugins, err := m.PreparePlug(conf)
	if err != nil {
		return fmt.Errorf("failed to prepare plugins: %w", err)
	}
	if len(preparedPlugins) == 0 {
		log.Infof("no unmanaged plugins prepared; skipping load")
		return nil
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

	return nil
}

// UnloadPlugins stops and unloads all plugins with overall timeout protection.
// Optimized for stability: adds total timeout, parallel unloading, and better error handling.
func (m *DefaultPluginManager[T]) UnloadPlugins() {
	if m == nil {
		return
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	if len(m.managedPluginList) == 0 {
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

	batches, ordered := m.buildUnloadBatches(m.managedPluginList)

	var mu sync.Mutex
	// Optimized: Pre-allocate slice capacity to avoid frequent reallocations
	unloadErrors := make([]string, 0, len(ordered))
	timeoutReached := false

	// Fix Bug 1: Track which plugins are being cleaned up to avoid race conditions
	// Use sync.Map to safely track plugins that have started cleanup
	cleaningUp := sync.Map{} // map[pluginID]bool

	for _, batch := range batches {
		// Check if overall timeout has been reached
		select {
		case <-ctx.Done():
			log.Errorf("UnloadPlugins: overall timeout (%v) reached, forcing shutdown of remaining %d plugins",
				totalTimeout, len(ordered))
			timeoutReached = true
			m.forceCleanupRemainingPlugins(ordered, &cleaningUp)
			break
		default:
		}

		if timeoutReached {
			break
		}

		if m.unloadBatch(ctx, batch, perPluginTimeout, parallelism, &cleaningUp, &unloadErrors, &mu) {
			// Overall timeout reached
			log.Errorf("UnloadPlugins: overall timeout (%v) reached, some plugins may not have been properly unloaded",
				totalTimeout)
			timeoutReached = true
		}
	}

	if !timeoutReached {
		unloadDuration := time.Since(unloadStart)
		if len(unloadErrors) > 0 {
			log.Warnf("UnloadPlugins completed in %v with %d errors: %v",
				unloadDuration, len(unloadErrors), unloadErrors)
		} else {
			log.Infof("UnloadPlugins completed successfully in %v", unloadDuration)
		}
	}

	m.resetPluginManagerStateAfterUnload()
}

func (m *DefaultPluginManager[T]) unloadManagedPlugin(ctx context.Context, plugin plugins.Plugin, perPluginTimeout time.Duration, cleaningUp *sync.Map) (unloadPluginResult, bool) {
	result := unloadPluginResult{}
	if plugin == nil {
		return result, false
	}

	pluginID := plugin.ID()
	result.pluginName = plugin.Name()

	// Fix Bug 1: Mark this plugin as being cleaned up to prevent double cleanup.
	if _, alreadyCleaning := cleaningUp.LoadOrStore(pluginID, true); alreadyCleaning {
		log.Debugf("Skipping cleanup for plugin %s: already being cleaned up by forced cleanup", plugin.Name())
		return result, false
	}
	defer cleaningUp.Delete(pluginID)

	pluginCtx, pluginCancel := context.WithTimeout(ctx, perPluginTimeout)
	defer pluginCancel()

	m.emitPluginUnloadEvent(pluginID, plugin.Name())

	result.stopErr = m.stopPluginForUnload(pluginCtx, plugin, perPluginTimeout, unloadExecutionOptions{operation: "unload"})
	result.cleanupErr = m.cleanupPluginForUnload(pluginCtx, plugin, pluginID, perPluginTimeout)
	result.shouldRecord = result.stopErr != nil || result.cleanupErr != nil

	m.unregisterPlugin(plugin)
	return result, true
}

func (m *DefaultPluginManager[T]) stopPluginForUnload(pluginCtx context.Context, plugin plugins.Plugin, perPluginTimeout time.Duration, opts unloadExecutionOptions) error {
	if !shouldStopPlugin(plugin) {
		return nil
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- m.safeStopPlugin(plugin, perPluginTimeout)
	}()

	select {
	case err := <-stopDone:
		if err != nil {
			log.Errorf("Failed to unload plugin %s: %v", plugin.Name(), err)
			m.emitPluginErrorEvent(plugin.ID(), plugin.Name(), opts.operation, err)
		}
		return err
	case <-pluginCtx.Done():
		err := fmt.Errorf("plugin stop timeout after %v", perPluginTimeout)
		log.Errorf("Plugin %s stop timed out, forcing cleanup", plugin.Name())
		m.emitPluginErrorEvent(plugin.ID(), plugin.Name(), opts.operation, err)
		return err
	}
}

func (m *DefaultPluginManager[T]) cleanupPluginForUnload(pluginCtx context.Context, plugin plugins.Plugin, pluginID string, perPluginTimeout time.Duration) error {
	cleanupDone := make(chan error, 1)
	go func() {
		cleanupDone <- m.runtime.CleanupResources(pluginID)
	}()

	select {
	case err := <-cleanupDone:
		if err != nil {
			log.Errorf("Failed to cleanup resources for plugin %s: %v", plugin.Name(), err)
			m.emitResourceCleanupErrorEvent(pluginID, plugin.Name(), err)
		}
		return err
	case <-pluginCtx.Done():
		err := fmt.Errorf("resource cleanup timeout after %v", perPluginTimeout)
		log.Errorf("Plugin %s resource cleanup timed out", plugin.Name())
		m.emitResourceCleanupErrorEvent(pluginID, plugin.Name(), err)
		return err
	}
}

func (m *DefaultPluginManager[T]) unloadPluginSynchronously(plugin plugins.Plugin, timeout time.Duration, opts unloadExecutionOptions) unloadPluginResult {
	result := unloadPluginResult{}
	if plugin == nil {
		return result
	}

	result.pluginName = plugin.Name()
	pluginCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	m.emitPluginUnloadEvent(plugin.ID(), plugin.Name())
	result.stopErr = m.stopPluginForUnload(pluginCtx, plugin, timeout, opts)
	result.cleanupErr = m.cleanupPluginForUnload(pluginCtx, plugin, plugin.ID(), timeout)
	result.shouldRecord = result.stopErr != nil || result.cleanupErr != nil
	if !result.shouldRecord || opts.unregisterOnError {
		m.unregisterPlugin(plugin)
	}
	return result
}

// LoadPluginsByName loads a subset of plugins by Name().
func (m *DefaultPluginManager[T]) LoadPluginsByName(conf config.Config, pluginNames []string) error {
	if m == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

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
	targetSet := make(map[string]struct{}, len(pluginNames))
	for _, name := range pluginNames {
		targetSet[name] = struct{}{}
	}
	var stagedButUnused []plugins.Plugin
	for _, plugin := range preparedPlugins {
		if plugin == nil {
			continue
		}
		if _, wanted := targetSet[plugin.Name()]; !wanted {
			stagedButUnused = append(stagedButUnused, plugin)
		}
	}
	defer func() {
		for _, plugin := range stagedButUnused {
			m.removePreparedPlugin(plugin)
		}
	}()

	for _, name := range pluginNames {
		if _, alreadyManaged := m.managedInstances.Load(name); alreadyManaged {
			log.Infof("plugin %s is already managed, skip load", name)
			continue
		}
		if plugin, exists := pluginMap[name]; exists {
			targetPlugins = append(targetPlugins, plugin)
		} else {
			return fmt.Errorf("plugin %s not found", name)
		}
	}
	if len(targetPlugins) == 0 {
		log.Infof("no unmanaged target plugins selected; skipping subset load")
		return nil
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
	if m == nil {
		return
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	if len(names) == 0 {
		return
	}

	timeout := m.getStopTimeout()

	var subset []plugins.Plugin
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	m.managedInstances.Range(func(key, value any) bool {
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

	ordered := m.resolveOrderedUnloadTargets(subset, "subset unload")

	for _, p := range ordered {
		if p == nil {
			continue
		}
		result := m.unloadPluginSynchronously(p, timeout, unloadExecutionOptions{operation: "unload", unregisterOnError: true})
		m.recordUnloadResult(p, result)
	}

}

// StopPlugin stops a single plugin by Name().
func (m *DefaultPluginManager[T]) StopPlugin(pluginName string) error {
	if m == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	plugin, exists := m.managedInstances.Load(pluginName)
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
		m.managedInstances.Delete(pluginName)
		return nil
	}

	timeout := m.getStopTimeout()
	result := m.unloadPluginSynchronously(p, timeout, unloadExecutionOptions{operation: "stop", unregisterOnError: false})
	m.recordUnloadResult(p, result)
	return unloadResultError(pluginName, result)
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

func (m *DefaultPluginManager[T]) emitManagerEvent(
	eventType plugins.EventType,
	priority int,
	category string,
	pluginID string,
	status plugins.PluginStatus,
	err error,
	metadata map[string]any,
) {
	if m.runtime == nil {
		return
	}

	pluginEvent := plugins.PluginEvent{
		Type:      eventType,
		Priority:  priority,
		Source:    "plugin-manager",
		Category:  category,
		PluginID:  pluginID,
		Status:    status,
		Timestamp: time.Now().Unix(),
		Error:     err,
		Metadata:  metadata,
	}

	m.runtime.EmitEvent(pluginEvent)
}

func managerOperationMetadata(pluginName, operation string, extra map[string]any) map[string]any {
	metadata := map[string]any{
		"operation": operation,
	}
	if pluginName != "" {
		metadata["plugin_name"] = pluginName
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

// emitPluginUnloadEvent emits a plugin unload event
func (m *DefaultPluginManager[T]) emitPluginUnloadEvent(pluginID, pluginName string) {
	m.emitManagerEvent(
		plugins.EventPluginStopping,
		plugins.PriorityNormal,
		"lifecycle",
		pluginID,
		plugins.StatusInactive,
		nil,
		managerOperationMetadata(pluginName, "unload", nil),
	)
}

// emitPluginErrorEvent emits a plugin error event
func (m *DefaultPluginManager[T]) emitPluginErrorEvent(pluginID, pluginName, operation string, err error) {
	m.emitManagerEvent(
		plugins.EventErrorOccurred,
		plugins.PriorityHigh,
		"error",
		pluginID,
		plugins.StatusFailed,
		err,
		managerOperationMetadata(pluginName, operation, nil),
	)
}

// emitResourceCleanupErrorEvent emits a resource cleanup error event
func (m *DefaultPluginManager[T]) emitResourceCleanupErrorEvent(pluginID, pluginName string, err error) {
	m.emitManagerEvent(
		plugins.EventErrorOccurred,
		plugins.PriorityNormal,
		"error",
		pluginID,
		plugins.StatusFailed,
		err,
		managerOperationMetadata(pluginName, "resource_cleanup", nil),
	)
}

// emitPluginManagerShutdownEvent emits a plugin manager shutdown event
func (m *DefaultPluginManager[T]) emitPluginManagerShutdownEvent() {
	m.emitManagerEvent(
		plugins.EventType("system.plugin_manager_shutdown"),
		plugins.PriorityHigh,
		"system",
		"system",
		plugins.StatusInactive,
		nil,
		managerOperationMetadata("", "shutdown", map[string]any{
			"reason": "application_close",
		}),
	)
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
