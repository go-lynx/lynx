package plugins

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
)

// GetResource gets a resource (backward compatible API).
func (r *UnifiedRuntime) GetResource(name string) (any, error) {
	return r.GetSharedResource(name)
}

// RegisterResource registers a resource (backward compatible API).
func (r *UnifiedRuntime) RegisterResource(name string, resource any) error {
	return r.RegisterSharedResource(name, resource)
}

// GetSharedResource retrieves a shared resource.
func (r *UnifiedRuntime) GetSharedResource(name string) (any, error) {
	if err := r.ensureResourceAccessAllowed(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	key := sharedResourceStorageKey(name)
	value, ok := r.resources.Load(key)
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	r.updateAccessStats(key)
	return value, nil
}

// RegisterSharedResource registers a shared resource.
func (r *UnifiedRuntime) RegisterSharedResource(name string, resource any) error {
	if err := r.ensureResourceRegistrationAllowed(name, resource); err != nil {
		return err
	}

	callerOwner := r.getCurrentOwner()
	caller := r.ownerPluginID(callerOwner)
	key := sharedResourceStorageKey(name)

	var oldResource any
	r.resourceOpMu.Lock()
	if oldInfoValue, loaded := r.resourceInfo.Load(key); loaded {
		if oldInfo, ok := oldInfoValue.(*ResourceInfo); ok {
			owner := oldInfo.PluginID
			if owner == "" {
				owner = "system"
			}
			if !r.canManageResource(callerOwner, oldInfo) {
				r.resourceOpMu.Unlock()
				return fmt.Errorf("permission denied: plugin %q cannot overwrite shared resource %q owned by %q", caller, name, owner)
			}
		}
		if existing, ok := r.resources.Load(key); ok {
			oldResource = existing
		}
	}

	now := time.Now()
	info := &ResourceInfo{
		Name:          name,
		Type:          reflect.TypeOf(resource).String(),
		PluginID:      caller,
		OwnerHandleID: r.ownerHandleID(callerOwner),
		IsPrivate:     false,
		CreatedAt:     now,
		LastUsedAt:    now,
		AccessCount:   0,
		Size:          0,
		Metadata:      make(map[string]any),
	}

	r.resources.Store(key, resource)
	r.resourceInfo.Store(key, info)
	r.resourceOpMu.Unlock()

	if oldResource != nil {
		_ = r.cleanupResourceGracefully(name, oldResource)
	}
	r.scheduleResourceSizeEstimate(key, resource)
	return nil
}

// GetPrivateResource gets a private (plugin-scoped) resource.
func (r *UnifiedRuntime) GetPrivateResource(name string) (any, error) {
	pluginID := r.getCurrentPluginContext()
	if pluginID == "" {
		return nil, fmt.Errorf("no plugin context set")
	}

	key := privateResourceStorageKey(pluginID, name)
	value, ok := r.resources.Load(key)
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	r.updateAccessStats(key)
	return value, nil
}

// RegisterPrivateResource registers a private (plugin-scoped) resource.
func (r *UnifiedRuntime) RegisterPrivateResource(name string, resource any) error {
	if err := r.ensureResourceRegistrationAllowed(name, resource); err != nil {
		return err
	}

	callerOwner := r.getCurrentOwner()
	pluginID := r.ownerPluginID(callerOwner)
	if pluginID == "" || r.isSystemOwner(callerOwner) {
		return fmt.Errorf("no plugin context set")
	}

	key := privateResourceStorageKey(pluginID, name)
	displayName := privateResourceDisplayName(pluginID, name)

	var oldResource any
	r.resourceOpMu.Lock()
	if existing, loaded := r.resources.Load(key); loaded {
		oldResource = existing
	}

	now := time.Now()
	info := &ResourceInfo{
		Name:          displayName,
		Type:          reflect.TypeOf(resource).String(),
		PluginID:      pluginID,
		OwnerHandleID: r.ownerHandleID(callerOwner),
		IsPrivate:     true,
		CreatedAt:     now,
		LastUsedAt:    now,
		AccessCount:   0,
		Size:          0,
		Metadata:      make(map[string]any),
	}

	r.resources.Store(key, resource)
	r.resourceInfo.Store(key, info)
	r.resourceOpMu.Unlock()

	if oldResource != nil {
		_ = r.cleanupResourceGracefully(displayName, oldResource)
	}
	r.scheduleResourceSizeEstimate(key, resource)
	return nil
}

func (r *UnifiedRuntime) ensureResourceAccessAllowed() error {
	if r.isClosed() {
		return fmt.Errorf("runtime is closed")
	}
	select {
	case <-r.shutdownCtx.Done():
		return fmt.Errorf("runtime is closed")
	default:
		return nil
	}
}

func (r *UnifiedRuntime) ensureResourceRegistrationAllowed(name string, resource any) error {
	if err := r.ensureResourceAccessAllowed(); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	return nil
}

func (r *UnifiedRuntime) scheduleResourceSizeEstimate(key string, resource any) {
	go func() {
		select {
		case <-r.shutdownCtx.Done():
			return
		default:
			size := r.estimateResourceSize(resource)
			select {
			case <-r.shutdownCtx.Done():
				return
			default:
				if value, ok := r.resourceInfo.Load(key); ok {
					if existingInfo, ok := value.(*ResourceInfo); ok {
						atomic.StoreInt64(&existingInfo.Size, size)
					}
				}
			}
		}
	}()
}
