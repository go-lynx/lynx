package plugins

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// SetEventBusAdapter injects the event adapter used by this runtime instance.
func (r *UnifiedRuntime) SetEventBusAdapter(adapter EventBusAdapter) {
	state := r.sharedState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.eventAdapter = adapter
}

// GetConfig returns the config.
func (r *UnifiedRuntime) GetConfig() config.Config {
	state := r.sharedState()
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.config
}

// SetConfig sets the config.
func (r *UnifiedRuntime) SetConfig(conf config.Config) {
	state := r.sharedState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.config = conf
}

// GetLogger returns the logger.
func (r *UnifiedRuntime) GetLogger() log.Logger {
	state := r.sharedState()
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.logger == nil {
		return log.DefaultLogger
	}
	return state.logger
}

// SetLogger sets the logger.
func (r *UnifiedRuntime) SetLogger(logger log.Logger) {
	state := r.sharedState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.logger = logger
}

// Shutdown closes the Runtime.
func (r *UnifiedRuntime) Shutdown() {
	state := r.sharedState()
	state.mu.Lock()
	if state.closed {
		state.mu.Unlock()
		return
	}
	adapter := state.eventAdapter
	logger := state.logger
	state.closed = true
	state.mu.Unlock()

	if r.shutdownCancel != nil {
		defer r.shutdownCancel()
	}

	if err := r.cleanupAllResources(); err != nil && logger != nil {
		logger.Log(log.LevelWarn, "msg", "failed to cleanup runtime resources during shutdown", "error", err)
	}

	if adapter != nil {
		if shutdownable, ok := adapter.(interface{ Shutdown() error }); ok {
			if err := shutdownable.Shutdown(); err != nil && logger != nil {
				logger.Log(log.LevelWarn, "msg", "failed to shutdown event bus", "error", err)
			}
		}
	}
}

// Close closes the Runtime (compatibility API).
func (r *UnifiedRuntime) Close() {
	r.Shutdown()
}

func (r *UnifiedRuntime) isClosed() bool {
	state := r.sharedState()
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.closed
}

func (r *UnifiedRuntime) getEventAdapter() EventBusAdapter {
	state := r.sharedState()
	state.mu.RLock()
	adapter := state.eventAdapter
	state.mu.RUnlock()
	return adapter
}

func (r *UnifiedRuntime) sharedState() *runtimeSharedState {
	if r.shared != nil {
		return r.shared
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shared == nil {
		r.shared = &runtimeSharedState{
			config:       r.config,
			logger:       r.logger,
			eventAdapter: r.eventAdapter,
			closed:       r.closed,
		}
		if r.shared.logger == nil {
			r.shared.logger = log.DefaultLogger
		}
	}
	return r.shared
}
