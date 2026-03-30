package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// GetResourceInfo returns a copy of resource info so callers cannot mutate internal state.
func (r *UnifiedRuntime) GetResourceInfo(name string) (*ResourceInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	key := r.resolveResourceInfoLookupKey(name)
	value, ok := r.resourceInfo.Load(key)
	if !ok {
		return nil, fmt.Errorf("resource info not found: %s", name)
	}

	info, ok := value.(*ResourceInfo)
	if !ok {
		return nil, fmt.Errorf("invalid resource info type for: %s", name)
	}

	return copyResourceInfo(info), nil
}

func (r *UnifiedRuntime) resolveResourceInfoLookupKey(name string) string {
	if _, ok := r.resourceInfo.Load(name); ok {
		return name
	}

	sharedKey := sharedResourceStorageKey(name)
	if _, ok := r.resourceInfo.Load(sharedKey); ok {
		return sharedKey
	}

	if pluginID, privateName, ok := parsePrivateResourceDisplayName(name); ok {
		privateKey := privateResourceStorageKey(pluginID, privateName)
		if _, exists := r.resourceInfo.Load(privateKey); exists {
			return privateKey
		}
	}

	if pluginID, privateName, ok := parseLegacyPrivateResourceDisplayName(name); ok {
		privateKey := privateResourceStorageKey(pluginID, privateName)
		if _, exists := r.resourceInfo.Load(privateKey); exists {
			return privateKey
		}
	}

	return name
}

// ListResources returns copies of all resource infos so callers cannot mutate internal state.
func (r *UnifiedRuntime) ListResources() []*ResourceInfo {
	var resources []*ResourceInfo

	r.resourceInfo.Range(func(key, value any) bool {
		if info, ok := value.(*ResourceInfo); ok {
			resources = append(resources, copyResourceInfo(info))
		}
		return true
	})

	return resources
}

// CleanupResources cleans up resources for a plugin.
func (r *UnifiedRuntime) CleanupResources(pluginID string) error {
	if pluginID == "" {
		return fmt.Errorf("plugin ID cannot be empty")
	}

	callerOwner := r.getCurrentOwner()
	caller := r.ownerPluginID(callerOwner)
	if caller != "" && !r.isSystemOwner(callerOwner) {
		if caller != pluginID {
			return fmt.Errorf("permission denied: plugin %q cannot cleanup resources of plugin %q (only owner or system can cleanup)", caller, pluginID)
		}
	} else {
		caller = "system-shutdown"
	}

	r.resourceOpMu.Lock()
	type resItem struct {
		key         string
		displayName string
		res         any
	}
	var toDelete []resItem

	r.resourceInfo.Range(func(key, value any) bool {
		if info, ok := value.(*ResourceInfo); ok && info.PluginID == pluginID {
			storageKey := key.(string)
			if resource, exists := r.resources.Load(storageKey); exists {
				toDelete = append(toDelete, resItem{key: storageKey, displayName: info.Name, res: resource})
			}
		}
		return true
	})

	for _, item := range toDelete {
		r.resources.Delete(item.key)
		r.resourceInfo.Delete(item.key)
	}
	r.resourceOpMu.Unlock()

	if len(toDelete) > 0 {
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelInfo, "msg", "resource cleanup initiated", "plugin_id", pluginID, "caller", caller)
		}
	}

	var errors []error
	var cleanedCount int
	seenCleanupTargets := make(map[string]struct{}, len(toDelete))
	for _, item := range toDelete {
		if identity, ok := resourceCleanupIdentity(item.res); ok {
			if _, exists := seenCleanupTargets[identity]; exists {
				cleanedCount++
				continue
			}
			seenCleanupTargets[identity] = struct{}{}
		}

		if err := r.cleanupResourceGracefully(item.displayName, item.res); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup resource %s: %w", item.displayName, err))
		} else {
			cleanedCount++
		}
	}

	if len(toDelete) > 0 {
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelInfo, "msg", "cleaned up resources for plugin",
				"plugin_id", pluginID,
				"total", len(toDelete),
				"cleaned", cleanedCount,
				"errors", len(errors))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("resource cleanup had %d errors: %v", len(errors), errors[0])
	}
	return nil
}

// cleanupResourceGracefully attempts to gracefully cleanup a resource.
func (r *UnifiedRuntime) cleanupResourceGracefully(name string, resource any) error {
	if resource == nil {
		return nil
	}

	defer func() {
		if rec := recover(); rec != nil {
			if logger := r.GetLogger(); logger != nil {
				logger.Log(log.LevelWarn, "msg", "panic during resource cleanup", "resource", name, "panic", rec)
			}
		}
	}()

	startTime := time.Now()
	defer func() {
		if duration := time.Since(startTime); duration > 5*time.Second {
			if logger := r.GetLogger(); logger != nil {
				logger.Log(log.LevelWarn, "msg", "slow resource cleanup", "resource", name, "duration", duration)
			}
		}
	}()

	cleanupTimeout := r.getResourceCleanupTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	switch v := resource.(type) {
	case interface{ ShutdownContext(context.Context) error }:
		if err := v.ShutdownContext(ctx); err != nil {
			return fmt.Errorf("shutdown (ctx) failed: %w", err)
		}
		return nil
	case interface{ StopContext(context.Context) error }:
		if err := v.StopContext(ctx); err != nil {
			return fmt.Errorf("stop (ctx) failed: %w", err)
		}
		return nil
	case interface{ CloseContext(context.Context) error }:
		if err := v.CloseContext(ctx); err != nil {
			return fmt.Errorf("close (ctx) failed: %w", err)
		}
		return nil
	case interface{ CleanupContext(context.Context) error }:
		if err := v.CleanupContext(ctx); err != nil {
			return fmt.Errorf("cleanup (ctx) failed: %w", err)
		}
		return nil
	case interface{ DestroyContext(context.Context) error }:
		if err := v.DestroyContext(ctx); err != nil {
			return fmt.Errorf("destroy (ctx) failed: %w", err)
		}
		return nil
	}

	switch v := resource.(type) {
	case interface{ Shutdown() error }:
		return normalizeCleanupError(v.Shutdown())
	case interface{ Stop() error }:
		return normalizeCleanupError(v.Stop())
	case interface{ Cleanup() error }:
		return normalizeCleanupError(v.Cleanup())
	case interface{ Destroy() error }:
		return normalizeCleanupError(v.Destroy())
	case interface{ Release() error }:
		return normalizeCleanupError(v.Release())
	case interface{ Close() error }:
		return normalizeCleanupError(v.Close())
	case context.CancelFunc:
		v()
		return nil
	case func():
		v()
		return nil
	}

	if val := reflect.ValueOf(resource); val.Kind() == reflect.Chan && val.Type().ChanDir() != reflect.RecvDir {
		defer func() {
			if r := recover(); r != nil {
			}
		}()
		val.Close()
		return nil
	}

	return nil
}

func resourceCleanupIdentity(resource any) (string, bool) {
	if resource == nil {
		return "", false
	}

	val := reflect.ValueOf(resource)
	if !val.IsValid() {
		return "", false
	}

	switch val.Kind() {
	case reflect.Interface:
		if val.IsNil() {
			return "", false
		}
		return resourceCleanupIdentity(val.Elem().Interface())
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		if val.Kind() != reflect.UnsafePointer && val.IsNil() {
			return "", false
		}
		return fmt.Sprintf("%T:%x", resource, val.Pointer()), true
	default:
		return "", false
	}
}

func normalizeCleanupError(err error) error {
	if isBenignCleanupError(err) {
		return nil
	}
	return err
}

func isBenignCleanupError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"already closed",
		"client is closed",
		"database is closed",
		"read/write on closed pipe",
		"use of closed network connection",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}

	return false
}

// GetResourceStats returns resource statistics including size and plugin information.
func (r *UnifiedRuntime) GetResourceStats() map[string]any {
	var totalResources, privateResources, sharedResources int
	var totalSize int64
	pluginSet := make(map[string]bool)

	r.resourceInfo.Range(func(key, value any) bool {
		if info, ok := value.(*ResourceInfo); ok {
			totalResources++
			totalSize += atomic.LoadInt64(&info.Size)
			pluginSet[info.PluginID] = true
			if info.IsPrivate {
				privateResources++
			} else {
				sharedResources++
			}
		}
		return true
	})

	return map[string]any{
		"total_resources":        totalResources,
		"private_resources":      privateResources,
		"shared_resources":       sharedResources,
		"total_size_bytes":       totalSize,
		"plugins_with_resources": len(pluginSet),
		"runtime_closed":         r.isClosed(),
	}
}

// getResourceCleanupTimeout returns the timeout for resource cleanup, default 3s.
func (r *UnifiedRuntime) getResourceCleanupTimeout() time.Duration {
	defaultTimeout := 3 * time.Second
	cfg := r.GetConfig()
	if cfg == nil {
		return defaultTimeout
	}

	var confStr string
	if err := cfg.Value("lynx.runtime.resource_cleanup_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			if parsed < 1*time.Second {
				if logger := r.GetLogger(); logger != nil {
					logger.Log(log.LevelWarn, "msg", "resource_cleanup_timeout too short, using minimum 1s", "configured", parsed)
				}
				return 1 * time.Second
			}
			if parsed > 30*time.Second {
				if logger := r.GetLogger(); logger != nil {
					logger.Log(log.LevelWarn, "msg", "resource_cleanup_timeout too long, using maximum 30s", "configured", parsed)
				}
				return 30 * time.Second
			}
			return parsed
		}
	}
	return defaultTimeout
}

// copyResourceInfo returns a shallow copy of ResourceInfo with Metadata and atomic fields snapshotted.
func copyResourceInfo(info *ResourceInfo) *ResourceInfo {
	if info == nil {
		return nil
	}
	c := &ResourceInfo{
		Name:          info.Name,
		Type:          info.Type,
		PluginID:      info.PluginID,
		OwnerHandleID: info.OwnerHandleID,
		IsPrivate:     info.IsPrivate,
		CreatedAt:     info.CreatedAt,
		LastUsedAt:    info.LastUsedAt,
		AccessCount:   atomic.LoadInt64(&info.AccessCount),
		Size:          atomic.LoadInt64(&info.Size),
	}
	if info.Metadata != nil {
		c.Metadata = make(map[string]any, len(info.Metadata))
		for k, v := range info.Metadata {
			c.Metadata[k] = v
		}
	}
	return c
}

// updateAccessStats updates access statistics for a resource.
func (r *UnifiedRuntime) updateAccessStats(name string) {
	if value, ok := r.resourceInfo.Load(name); ok {
		if info, ok := value.(*ResourceInfo); ok {
			atomic.AddInt64(&info.AccessCount, 1)
			info.LastUsedAt = time.Now()
		}
	}
}

// estimateResourceSize estimates the size of a resource.
func (r *UnifiedRuntime) estimateResourceSize(resource any) int64 {
	if resource == nil {
		return 0
	}
	val := reflect.ValueOf(resource)
	visited := make(map[uintptr]bool)
	return r.estimateValueSizeWithDepth(val, 0, 20, visited)
}

// estimateValueSizeWithDepth recursively estimates value size with protection.
func (r *UnifiedRuntime) estimateValueSizeWithDepth(val reflect.Value, depth, maxDepth int, visited map[uintptr]bool) int64 {
	if !val.IsValid() || depth > maxDepth {
		return 0
	}

	if val.Kind() == reflect.Ptr && !val.IsNil() {
		ptr := val.Pointer()
		if visited[ptr] {
			return 8
		}
		visited[ptr] = true
		defer func() { delete(visited, ptr) }()
	}

	switch val.Kind() {
	case reflect.String:
		return int64(val.Len())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return 8
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return 8
	case reflect.Float32, reflect.Float64:
		return 8
	case reflect.Bool:
		return 1
	case reflect.Slice, reflect.Array:
		size := int64(0)
		length := val.Len()
		maxElements := 1000
		if length > maxElements {
			sampleSize := int64(0)
			for i := 0; i < maxElements && i < length; i++ {
				sampleSize += r.estimateValueSizeWithDepth(val.Index(i), depth+1, maxDepth, visited)
			}
			return (sampleSize * int64(length)) / int64(maxElements)
		}
		for i := 0; i < length; i++ {
			size += r.estimateValueSizeWithDepth(val.Index(i), depth+1, maxDepth, visited)
		}
		return size
	case reflect.Map:
		size := int64(0)
		keys := val.MapKeys()
		maxKeys := 1000
		if len(keys) > maxKeys {
			sampleSize := int64(0)
			for i := 0; i < maxKeys; i++ {
				key := keys[i]
				sampleSize += r.estimateValueSizeWithDepth(key, depth+1, maxDepth, visited)
				sampleSize += r.estimateValueSizeWithDepth(val.MapIndex(key), depth+1, maxDepth, visited)
			}
			return (sampleSize * int64(len(keys))) / int64(maxKeys)
		}
		for _, key := range keys {
			size += r.estimateValueSizeWithDepth(key, depth+1, maxDepth, visited)
			size += r.estimateValueSizeWithDepth(val.MapIndex(key), depth+1, maxDepth, visited)
		}
		return size
	case reflect.Struct:
		size := int64(0)
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if field.CanInterface() {
				size += r.estimateValueSizeWithDepth(field, depth+1, maxDepth, visited)
			}
		}
		return size
	case reflect.Ptr:
		if val.IsNil() {
			return 8
		}
		return 8 + r.estimateValueSizeWithDepth(val.Elem(), depth+1, maxDepth, visited)
	case reflect.Interface:
		if val.IsNil() {
			return 8
		}
		return 8 + r.estimateValueSizeWithDepth(val.Elem(), depth+1, maxDepth, visited)
	default:
		return 8
	}
}
