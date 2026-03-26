package plugins

import (
	"sync"

	"github.com/go-kratos/kratos/v2/log"
)

// WithPluginContext creates a Runtime bound with plugin context.
func (r *UnifiedRuntime) WithPluginContext(pluginName string) Runtime {
	curOwner := r.getCurrentOwner()
	cur := r.getCurrentPluginContext()

	if pluginName == "" || pluginName == cur {
		return r
	}

	if (cur == "" || r.isSystemOwner(curOwner)) && pluginName != "" {
		owner := r.resolveOwner(pluginName)
		return &UnifiedRuntime{
			resources:            r.resources,
			resourceInfo:         r.resourceInfo,
			resourceOpMu:         r.resourceOpMu,
			config:               r.GetConfig(),
			logger:               r.GetLogger(),
			shared:               r.sharedState(),
			currentPluginContext: pluginName,
			contextMu:            sync.RWMutex{},
			owner:                owner,
			ownerHandles:         r.ownerHandles,
			ownerSeq:             r.ownerSeq,
			eventManager:         r.eventManager,
			eventAdapter:         nil,
			closed:               false,
			mu:                   sync.RWMutex{},
			shutdownCtx:          r.shutdownCtx,
			shutdownCancel:       nil,
		}
	}

	if logger := r.GetLogger(); logger != nil {
		logger.Log(log.LevelWarn, "msg", "denied WithPluginContext switch", "from", cur, "to", pluginName)
	}
	return r
}

// GetCurrentPluginContext returns current plugin context.
func (r *UnifiedRuntime) GetCurrentPluginContext() string {
	return r.getCurrentPluginContext()
}

func (r *UnifiedRuntime) getCurrentPluginContext() string {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	return r.currentPluginContext
}

func (r *UnifiedRuntime) getCurrentOwner() *runtimeOwner {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	if r.owner != nil {
		return r.owner
	}
	if r.currentPluginContext == "" {
		return &runtimeOwner{pluginID: "system"}
	}
	return &runtimeOwner{pluginID: r.currentPluginContext}
}

func (r *UnifiedRuntime) resolveOwner(pluginName string) *runtimeOwner {
	if pluginName == "" {
		return &runtimeOwner{pluginID: "system"}
	}
	if r.ownerHandles != nil {
		if value, ok := r.ownerHandles.Load(pluginName); ok {
			if owner, ok := value.(*runtimeOwner); ok {
				return owner
			}
		}
	}

	owner := &runtimeOwner{pluginID: pluginName}
	if r.ownerSeq != nil {
		owner.handleID = r.ownerSeq.Add(1)
	}
	if r.ownerHandles == nil {
		r.ownerHandles = &sync.Map{}
	}
	if actual, loaded := r.ownerHandles.LoadOrStore(pluginName, owner); loaded {
		if existing, ok := actual.(*runtimeOwner); ok {
			return existing
		}
	}
	return owner
}

func (r *UnifiedRuntime) ownerPluginID(owner *runtimeOwner) string {
	if owner == nil || owner.pluginID == "" {
		return "system"
	}
	return owner.pluginID
}

func (r *UnifiedRuntime) ownerHandleID(owner *runtimeOwner) uint64 {
	if owner == nil {
		return 0
	}
	return owner.handleID
}

func (r *UnifiedRuntime) isSystemOwner(owner *runtimeOwner) bool {
	return owner == nil || owner.pluginID == "" || owner.pluginID == "system"
}

func (r *UnifiedRuntime) canManageResource(caller *runtimeOwner, info *ResourceInfo) bool {
	if info == nil {
		return true
	}
	if r.isSystemOwner(caller) {
		return true
	}
	if caller == nil {
		return false
	}
	if info.OwnerHandleID != 0 && caller.handleID != 0 {
		return info.OwnerHandleID == caller.handleID
	}
	return info.PluginID == caller.pluginID
}
