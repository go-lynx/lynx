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

type rollbackResult struct {
	pluginName string
	stopErr    error
	cleanupErr error
	success    bool
}

type lifecycleStepInfo struct {
	caps     plugins.PluginCapabilities
	ctxAware bool
}

type lifecycleExecOptions struct {
	action          string
	plugin          plugins.Plugin
	logStackTrace   bool
	contextRunner   func(context.Context) error
	plainRunner     func() error
	cancelledErr    func(error) error
	timeoutErr      func() error
	cleanupLeakWarn func(time.Duration)
	lateResultWarn  func(error)
}

// loadSortedPluginsByLevel starts plugins in parallel by dependency level and rolls back on failure.
// allSorted is kept intact across batches so that rollback can unregister every plugin
// that was prepared (not just those in the failing batch).
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

	// started accumulates every successfully started plugin across ALL batches.
	// On rollback we need to stop every plugin we have already started, not just
	// the ones in the current batch.
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
				if err := m.loadSinglePlugin(p, initTimeout, startTimeout); err != nil {
					errCh <- err
					return
				}
				startedMu.Lock()
				started = append(started, p)
				startedMu.Unlock()
			}()
		}

		wg.Wait()
		close(errCh)
		if len(errCh) > 0 {
			firstErr, allErrors := collectLifecycleErrors(errCh)
			// Pass the full `sorted` list so rollback unregisters every prepared
			// plugin, not only those in the failing batch.
			results, successCount := m.rollbackStartedPlugins(started, sorted, allErrors)
			if successCount < len(results) {
				return fmt.Errorf("plugin startup failed with %d errors, rollback had %d failures: %w",
					len(allErrors), len(results)-successCount, firstErr)
			}
			return firstErr
		}
	}

	return nil
}

func (m *DefaultPluginManager[T]) loadSinglePlugin(p plugins.Plugin, initTimeout, startTimeout time.Duration) error {
	if p == nil {
		return nil
	}

	rt := m.runtime.WithPluginContext(p.ID())
	if err := m.initializeLoadedPlugin(p, rt, initTimeout); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", p.ID(), err)
	}
	if err := m.startLoadedPlugin(p, startTimeout); err != nil {
		return fmt.Errorf("failed to start plugin %s: %w", p.ID(), err)
	}
	if err := m.registerManagedPlugin(p); err != nil {
		return fmt.Errorf("failed to register managed plugin %s: %w", p.ID(), err)
	}
	m.registerPluginMetricsGatherer(p)
	return nil
}

func (m *DefaultPluginManager[T]) initializeLoadedPlugin(p plugins.Plugin, rt plugins.Runtime, timeout time.Duration) error {
	initInfo := describeLifecycleStep(p, "initialize")
	m.emitPluginEvent(p.ID(), events.EventPluginInitializing, lifecycleStepMetadata(p, "initialize", timeout, initInfo.ctxAware))

	initStart := time.Now()
	if err := m.safeInitPlugin(p, rt, timeout); err != nil {
		setPluginStatusIfSupported(p, plugins.StatusFailed)
		if cleanupErr := m.runtime.CleanupResources(p.ID()); cleanupErr != nil {
			log.Warnf("Failed to cleanup resources for plugin %s after init failure: %v", p.ID(), cleanupErr)
		}
		m.emitLifecycleFailure(p, "initialize", initStart, initInfo.ctxAware, err)
		return err
	}

	setPluginStatusIfSupported(p, plugins.StatusInactive)
	m.emitLifecycleSuccess(p, events.EventPluginInitialized, initStart, initInfo.ctxAware)
	return nil
}

func (m *DefaultPluginManager[T]) startLoadedPlugin(p plugins.Plugin, timeout time.Duration) error {
	startInfo := describeLifecycleStep(p, "start")
	m.emitPluginEvent(p.ID(), events.EventPluginStarting, lifecycleStepMetadata(p, "start", timeout, startInfo.ctxAware))

	startStart := time.Now()
	if err := m.safeStartPlugin(p, timeout); err != nil {
		setPluginStatusIfSupported(p, plugins.StatusFailed)
		if cleanupErr := m.runtime.CleanupResources(p.ID()); cleanupErr != nil {
			log.Warnf("Failed to cleanup resources for plugin %s after start failure: %v", p.ID(), cleanupErr)
		}
		m.emitLifecycleFailure(p, "start", startStart, startInfo.ctxAware, err)
		return err
	}

	setPluginStatusIfSupported(p, plugins.StatusActive)
	m.emitLifecycleSuccess(p, events.EventPluginStarted, startStart, startInfo.ctxAware)
	return nil
}

func (m *DefaultPluginManager[T]) registerPluginMetricsGatherer(p plugins.Plugin) {
	type metricsGathererProvider interface{ MetricsGatherer() prometheus.Gatherer }
	if prov, ok := p.(metricsGathererProvider); ok {
		if g := prov.MetricsGatherer(); g != nil {
			metrics.RegisterGatherer(g)
			log.Infof("registered private metrics gatherer for plugin %s", p.ID())
		}
	}
}

// getStartParallelism returns start/init parallelism, default 8.
func (m *DefaultPluginManager[T]) getStartParallelism() int {
	return m.getBoundedIntConfig("lynx.plugins.start_parallelism", 8, 1, 0)
}

// getInitTimeout returns Initialize timeout, default 5s.
func (m *DefaultPluginManager[T]) getInitTimeout() time.Duration {
	return m.getBoundedDurationConfig("lynx.plugins.init_timeout", 5*time.Second, 1*time.Second, 60*time.Second)
}

// getStartTimeout returns Start timeout, default 5s.
func (m *DefaultPluginManager[T]) getStartTimeout() time.Duration {
	return m.getBoundedDurationConfig("lynx.plugins.start_timeout", 5*time.Second, 1*time.Second, 60*time.Second)
}

// getStopTimeout returns Stop timeout, default 5s.
func (m *DefaultPluginManager[T]) getStopTimeout() time.Duration {
	return m.getBoundedDurationConfig("lynx.plugins.stop_timeout", 5*time.Second, 1*time.Second, 120*time.Second)
}

// getUnloadTotalTimeout returns total timeout for unloading all plugins, default 60s.
// This prevents the entire shutdown process from hanging indefinitely.
func (m *DefaultPluginManager[T]) getUnloadTotalTimeout() time.Duration {
	return m.getBoundedDurationConfig("lynx.plugins.unload_total_timeout", 60*time.Second, 10*time.Second, 300*time.Second)
}

// getUnloadParallelism returns parallelism for unloading plugins, default 4.
// Lower than start parallelism to avoid overwhelming the system during shutdown.
func (m *DefaultPluginManager[T]) getUnloadParallelism() int {
	return m.getBoundedIntConfig("lynx.plugins.unload_parallelism", 4, 1, 16)
}

func collectLifecycleErrors(errCh <-chan error) (error, []error) {
	var firstErr error
	var allErrors []error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
		allErrors = append(allErrors, err)
		log.Errorf("plugin start error: %v", err)
	}
	return firstErr, allErrors
}

func (m *DefaultPluginManager[T]) rollbackStartedPlugins(started []plugins.Plugin, sorted []PluginWithLevel, allErrors []error) ([]rollbackResult, int) {
	log.Errorf("Starting rollback for %d started plugins due to %d errors", len(started), len(allErrors))
	rollbackStart := time.Now()
	timeout := m.getStopTimeout()
	results := make([]rollbackResult, 0, len(started))

	// Rollback in reverse order (LIFO) to respect dependencies.
	for i := len(started) - 1; i >= 0; i-- {
		p := started[i]
		if p == nil {
			continue
		}
		results = append(results, m.rollbackSinglePlugin(p, timeout))
	}

	// Ensure plugins that never started, or the one that failed before rollback,
	// do not remain registered after a failed load attempt.
	for _, pwl := range sorted {
		if pwl.Plugin == nil {
			continue
		}
		m.unregisterPlugin(pwl.Plugin)
	}

	rollbackDuration := time.Since(rollbackStart)
	successCount := 0
	for _, r := range results {
		if r.success && r.stopErr == nil && r.cleanupErr == nil {
			successCount++
		}
	}

	log.Errorf("Rollback completed: total=%d, successful=%d, failed=%d, duration=%v",
		len(results), successCount, len(results)-successCount, rollbackDuration)

	if rt := m.runtime; rt != nil {
		rt.EmitPluginEvent("plugin-manager", "rollback.completed", map[string]any{
			"total_plugins":    len(results),
			"successful":       successCount,
			"failed":           len(results) - successCount,
			"duration_ms":      rollbackDuration.Milliseconds(),
			"initial_errors":   len(allErrors),
			"rollback_results": results,
		})
	}

	return results, successCount
}

func (m *DefaultPluginManager[T]) rollbackSinglePlugin(p plugins.Plugin, timeout time.Duration) rollbackResult {
	result := rollbackResult{
		pluginName: p.Name(),
		success:    true,
	}

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

	cleanupStart := time.Now()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), timeout)
	defer cleanupCancel()
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

	m.unregisterPlugin(p)
	return result
}

func (m *DefaultPluginManager[T]) getBoundedDurationConfig(key string, defaultValue, minValue, maxValue time.Duration) time.Duration {
	if m == nil || m.config == nil {
		return defaultValue
	}
	var confStr string
	if err := m.config.Value(key).Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			if parsed < minValue {
				log.Warnf("%s too short (%v), using minimum %v", key, parsed, minValue)
				return minValue
			}
			if parsed > maxValue {
				log.Warnf("%s too long (%v), using maximum %v", key, parsed, maxValue)
				return maxValue
			}
			return parsed
		}
	}
	return defaultValue
}

func (m *DefaultPluginManager[T]) getBoundedIntConfig(key string, defaultValue, minValue, maxValue int) int {
	if m == nil || m.config == nil {
		return defaultValue
	}
	var v int
	if err := m.config.Value(key).Scan(&v); err == nil {
		if v < minValue {
			return defaultValue
		}
		if maxValue > 0 && v > maxValue {
			log.Warnf("%s too high (%d), using maximum %d", key, v, maxValue)
			return maxValue
		}
		return v
	}
	return defaultValue
}

func describeLifecycleStep(p plugins.Plugin, step string) lifecycleStepInfo {
	info := lifecycleStepInfo{
		caps:     plugins.DescribePluginCapabilities(p),
		ctxAware: false,
	}
	info.ctxAware = info.caps.IsTrulyContextAware
	if !info.caps.Protocol.ContextLifecycle {
		log.Debugf("plugin %s (%s) does not declare context lifecycle protocol; step=%s. PluginProtocol().ContextLifecycle must be true to enable context-aware lifecycle", p.Name(), p.ID(), step)
	} else if !info.ctxAware {
		log.Debugf("plugin %s (%s) declares context lifecycle protocol but is not truly context-aware; step=%s. Ensure methods observe ctx and implement ContextAwareness.IsContextAware()=true", p.Name(), p.ID(), step)
	}
	return info
}

func lifecycleStepMetadata(p plugins.Plugin, step string, timeout time.Duration, ctxAware bool) map[string]any {
	return map[string]any{
		"plugin_name":   p.Name(),
		"plugin_id":     p.ID(),
		"step":          step,
		"timeout_ms":    timeout.Milliseconds(),
		"ctx_aware":     ctxAware,
		"deadline_unix": time.Now().Add(timeout).Unix(),
	}
}

func lifecycleResultMetadata(p plugins.Plugin, operation string, took time.Duration, ctxAware bool, timeout bool, err error) map[string]any {
	metadata := map[string]any{
		"plugin_name": p.Name(),
		"operation":   operation,
		"took_ms":     took.Milliseconds(),
		"ctx_aware":   ctxAware,
	}
	if timeout {
		metadata["timeout"] = true
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	return metadata
}

func lifecycleSuccessMetadata(p plugins.Plugin, took time.Duration, ctxAware bool) map[string]any {
	return map[string]any{
		"plugin_name": p.Name(),
		"plugin_id":   p.ID(),
		"took_ms":     took.Milliseconds(),
		"ctx_aware":   ctxAware,
	}
}

func (m *DefaultPluginManager[T]) emitLifecycleFailure(p plugins.Plugin, operation string, startedAt time.Time, ctxAware bool, err error) {
	if p == nil || err == nil {
		return
	}
	m.emitPluginEvent(p.ID(), events.EventErrorOccurred, lifecycleResultMetadata(p, operation, time.Since(startedAt), ctxAware, errors.Is(err, context.DeadlineExceeded), err))
}

func (m *DefaultPluginManager[T]) emitLifecycleSuccess(p plugins.Plugin, eventType events.EventType, startedAt time.Time, ctxAware bool) {
	if p == nil {
		return
	}
	m.emitPluginEvent(p.ID(), eventType, lifecycleSuccessMetadata(p, time.Since(startedAt), ctxAware))
}

func (m *DefaultPluginManager[T]) recoverLifecyclePanic(action string, p plugins.Plugin, r any, errCh chan<- error, ctx context.Context, includeStack bool) {
	msg := fmt.Sprintf("panic in %s of %s: %v", action, p.ID(), r)
	if includeStack {
		stackTrace := make([]byte, 4096)
		stackLen := runtime.Stack(stackTrace, false)
		log.Errorf("Panic in %s of %s: %v\nStack trace:\n%s", action, p.ID(), r, stackTrace[:stackLen])
	}
	select {
	case errCh <- fmt.Errorf("%s", msg):
	case <-ctx.Done():
	}
}

// goroutineCompletionWait is the time allowed for a non-context-aware goroutine
// to signal completion after returning its result. This prevents a very brief
// goroutine from being considered a leak, while not blocking the caller for long.
// It can be overridden in tests via the LYNX_GOROUTINE_DONE_TIMEOUT_MS environment variable.
const goroutineCompletionWait = 100 * time.Millisecond

// goroutineCleanupWait is the extra time allowed after a context cancellation for
// a non-context-aware goroutine to exit cleanly before we log a potential leak.
// It can be overridden in tests via the LYNX_GOROUTINE_CLEANUP_TIMEOUT_MS environment variable.
const goroutineCleanupWait = 200 * time.Millisecond

func (m *DefaultPluginManager[T]) runLifecycleNonContext(ctx context.Context, opts lifecycleExecOptions) error {
	errCh := make(chan error, 1)
	goroutineDone := make(chan struct{}, 1)

	go func() {
		defer func() {
			select {
			case goroutineDone <- struct{}{}:
			default:
			}
			if r := recover(); r != nil {
				m.recoverLifecyclePanic(opts.action, opts.plugin, r, errCh, ctx, opts.logStackTrace)
			}
		}()
		errCh <- opts.plainRunner()
	}()

	select {
	case err := <-errCh:
		select {
		case <-goroutineDone:
		case <-time.After(goroutineCompletionWait):
		}
		return err
	case <-ctx.Done():
		cleanupTimeout := goroutineCleanupWait
		select {
		case <-goroutineDone:
			select {
			case lateErr := <-errCh:
				if opts.lateResultWarn != nil {
					opts.lateResultWarn(lateErr)
				}
			default:
			}
		case <-time.After(cleanupTimeout):
			if opts.cleanupLeakWarn != nil {
				opts.cleanupLeakWarn(cleanupTimeout)
			}
			select {
			case <-errCh:
			default:
			}
		}
		return opts.timeoutErr()
	}
}

func (m *DefaultPluginManager[T]) runLifecycleStep(ctx context.Context, opts lifecycleExecOptions) error {
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.recoverLifecyclePanic(opts.action, opts.plugin, r, done, ctx, opts.logStackTrace)
			}
		}()

		if ctx.Err() != nil {
			select {
			case done <- opts.cancelledErr(ctx.Err()):
			case <-ctx.Done():
			}
			return
		}

		var err error
		if opts.contextRunner != nil {
			err = opts.contextRunner(ctx)
		} else {
			err = m.runLifecycleNonContext(ctx, opts)
		}

		select {
		case done <- err:
		case <-ctx.Done():
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return opts.timeoutErr()
	}
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

	var contextRunner func(context.Context) error
	if lc, ok := plugins.GetTrueContextLifecycle(p); ok {
		contextRunner = func(ctx context.Context) error {
			return lc.InitializeContext(ctx, p, rt)
		}
	}

	err := m.runLifecycleStep(ctx, lifecycleExecOptions{
		action:        "Initialize",
		plugin:        p,
		logStackTrace: true,
		contextRunner: contextRunner,
		plainRunner: func() error {
			return p.Initialize(p, rt)
		},
		cancelledErr: func(err error) error {
			return fmt.Errorf("initialize cancelled before start for plugin %s: %w", p.Name(), err)
		},
		timeoutErr: func() error {
			return fmt.Errorf("initialize timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
		},
		cleanupLeakWarn: func(cleanupTimeout time.Duration) {
			log.Warnf("plugin %s (%s) initialize goroutine did not complete within cleanup timeout (%v); plugin may not be respecting cancellation signals. This may cause goroutine leak.",
				p.Name(), p.ID(), cleanupTimeout)
		},
		lateResultWarn: func(lateErr error) {
			log.Warnf("plugin %s (%s) initialize completed after timeout with error: %v", p.Name(), p.ID(), lateErr)
		},
	})
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warnf("plugin %s (%s) initialize timed out; plugin may still be running", p.Name(), p.ID())
		setPluginStatusIfSupported(p, plugins.StatusFailed)
	}
	duration := time.Since(t0)
	if duration > timeout/2 {
		log.Warnf("Plugin %s initialize took %v (50%% of timeout %v)", p.Name(), duration, timeout)
	}
	return err
}

// lynxToPluginEventEntry maps an events.EventType to its corresponding
// plugins.EventType string and PluginStatus for observability.
type lynxToPluginEventEntry struct {
	pluginEventType plugins.EventType
	status          plugins.PluginStatus
}

// lynxToPluginEventTable is the single source-of-truth mapping between
// the unified events.EventType (uint32) and the plugin-level
// plugins.EventType (string) + PluginStatus pair.
// Adding a new event type only requires one entry here.
var lynxToPluginEventTable = map[events.EventType]lynxToPluginEventEntry{
	events.EventPluginInitializing: {plugins.EventPluginInitializing, plugins.StatusInitializing},
	events.EventPluginInitialized:  {plugins.EventPluginInitialized, plugins.StatusInactive},
	events.EventPluginStarting:     {plugins.EventPluginStarting, plugins.StatusInitializing},
	events.EventPluginStarted:      {plugins.EventPluginStarted, plugins.StatusActive},
	events.EventPluginStopping:     {plugins.EventPluginStopping, plugins.StatusStopping},
	events.EventPluginStopped:      {plugins.EventPluginStopped, plugins.StatusTerminated},
	events.EventErrorOccurred:      {plugins.EventErrorOccurred, plugins.StatusFailed},
}

// defaultLynxToPluginEventEntry is returned for any event type that is not
// explicitly listed in lynxToPluginEventTable.
var defaultLynxToPluginEventEntry = lynxToPluginEventEntry{
	pluginEventType: plugins.EventPluginInitializing,
	status:          plugins.StatusInactive,
}

// emitPluginEvent emits a plugin event to the unified event system.
// It converts events.EventType to plugins.EventType and derives the
// approximate PluginStatus using lynxToPluginEventTable.
func (m *DefaultPluginManager[T]) emitPluginEvent(pluginID string, eventType events.EventType, metadata map[string]any) {
	if m.runtime == nil {
		return
	}

	entry, ok := lynxToPluginEventTable[eventType]
	if !ok {
		entry = defaultLynxToPluginEventEntry
	}

	pluginEvent := plugins.PluginEvent{
		Type:      entry.pluginEventType,
		Priority:  plugins.PriorityNormal,
		Source:    "plugin-manager",
		Category:  "lifecycle",
		PluginID:  pluginID,
		Status:    entry.status,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

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

	var contextRunner func(context.Context) error
	if lc, ok := plugins.GetTrueContextLifecycle(p); ok {
		contextRunner = func(ctx context.Context) error {
			return lc.StartContext(ctx, p)
		}
	}

	err := m.runLifecycleStep(ctx, lifecycleExecOptions{
		action:        "Start",
		plugin:        p,
		logStackTrace: false,
		contextRunner: contextRunner,
		plainRunner: func() error {
			return p.Start(p)
		},
		cancelledErr: func(err error) error {
			return fmt.Errorf("start cancelled before execution for plugin %s: %w", p.Name(), err)
		},
		timeoutErr: func() error {
			return fmt.Errorf("start timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
		},
		cleanupLeakWarn: func(cleanupTimeout time.Duration) {
			log.Warnf("plugin %s (%s) start goroutine did not complete within cleanup timeout (%v); plugin may not be respecting cancellation signals. This may cause goroutine leak.",
				p.Name(), p.ID(), cleanupTimeout)
		},
		lateResultWarn: func(lateErr error) {
			log.Warnf("plugin %s (%s) start completed after timeout with error: %v", p.Name(), p.ID(), lateErr)
		},
	})
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warnf("plugin %s (%s) start timed out; plugin may still be running", p.Name(), p.ID())
		setPluginStatusIfSupported(p, plugins.StatusFailed)
	}
	duration := time.Since(t0)
	if duration > timeout/2 {
		log.Warnf("Plugin %s start took %v (50%% of timeout %v)", p.Name(), duration, timeout)
	}
	return err
}

// safeStopPlugin safely calls Stop with timeout and panic protection.
func (m *DefaultPluginManager[T]) safeStopPlugin(p plugins.Plugin, timeout time.Duration) error {
	if p == nil {
		return nil
	}

	stopInfo := describeLifecycleStep(p, "stop")

	// Emit plugin stopping event
	m.emitPluginEvent(p.ID(), events.EventPluginStopping, lifecycleStepMetadata(p, "stop", timeout, stopInfo.ctxAware))

	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var contextRunner func(context.Context) error
	if lc, ok := plugins.GetTrueContextLifecycle(p); ok {
		contextRunner = func(ctx context.Context) error {
			return lc.StopContext(ctx, p)
		}
	}

	err := m.runLifecycleStep(ctx, lifecycleExecOptions{
		action:        "Stop",
		plugin:        p,
		logStackTrace: false,
		contextRunner: contextRunner,
		plainRunner: func() error {
			return p.Stop(p)
		},
		cancelledErr: func(err error) error {
			return fmt.Errorf("stop cancelled before execution for plugin %s: %w", p.Name(), err)
		},
		timeoutErr: func() error {
			return fmt.Errorf("stop timeout after %s for plugin %s: %w", timeout.String(), p.Name(), context.DeadlineExceeded)
		},
		cleanupLeakWarn: func(cleanupTimeout time.Duration) {
			log.Warnf("plugin %s (%s) stop goroutine did not complete within cleanup timeout (%v); plugin may not be respecting cancellation signals. This may cause goroutine leak.",
				p.Name(), p.ID(), cleanupTimeout)
		},
		lateResultWarn: func(lateErr error) {
			log.Warnf("plugin %s (%s) stop completed after timeout with error: %v", p.Name(), p.ID(), lateErr)
		},
	})
	if err != nil {
		m.emitLifecycleFailure(p, "stop", t0, stopInfo.ctxAware, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warnf("plugin %s (%s) stop timed out; plugin may still be running", p.Name(), p.ID())
		setPluginStatusIfSupported(p, plugins.StatusFailed)
	}
	if err == nil {
		m.emitLifecycleSuccess(p, events.EventPluginStopped, t0, stopInfo.ctxAware)
	}
	return err
}
