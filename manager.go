// Package lynx provides the core application framework for building microservices.
//
// This file (manager.go) contains the plugin manager interfaces and implementation.
// The plugin manager handles plugin registration, lookup, and lifecycle coordination.
package lynx

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/pkg/factory"
	"github.com/go-lynx/lynx/plugins"
)

// Compile-time check: ensure DefaultPluginManager implements TypedPluginManager.
var _ TypedPluginManager = (*DefaultPluginManager[plugins.Plugin])(nil)

// PluginManager defines plugin management interfaces.
// Note: TypedPluginManager is an alias for PluginManager and should be used in all public APIs.
type PluginManager interface {
	// Basic plugin management
	LoadPlugins(config.Config) error
	UnloadPlugins()
	LoadPluginsByName(config.Config, []string) error
	UnloadPluginsByName([]string)
	GetPlugin(name string) plugins.Plugin
	GetPluginByID(id string) plugins.Plugin
	GetPluginCapabilities(name string) (plugins.PluginCapabilities, error)
	PreparePlug(config config.Config) ([]plugins.Plugin, error)

	// Runtime and config
	GetRuntime() plugins.Runtime
	SetConfig(config.Config)

	// Resource operations
	StopPlugin(pluginName string) error
	GetResourceStats() map[string]any
	ListResources() []*plugins.ResourceInfo

	// Monitoring operations
	GetUnloadFailures() []UnloadFailureRecord
	ClearUnloadFailures()
	GetLastPrepareReport() PrepareReport
}

// TypedPluginManager is an alias for PluginManager.
type TypedPluginManager = PluginManager

// UnloadFailureRecord tracks plugin unload failures for monitoring
type UnloadFailureRecord struct {
	PluginName   string
	PluginID     string
	FailureTime  time.Time
	StopError    error
	CleanupError error
	RetryCount   int
}

type PrepareFailure struct {
	PluginName string
	Reason     string
}

type PrepareReport struct {
	Prepared       []string
	Skipped        []string
	Failures       []PrepareFailure
	PartialAllowed bool
}

// DefaultPluginManager is the generic plugin manager implementation.
type DefaultPluginManager[T plugins.Plugin] struct {
	pluginInstances   sync.Map // Name() -> Plugin instance
	pluginIDs         sync.Map // ID() -> Plugin instance
	pluginList        []plugins.Plugin
	factory           *factory.TypedFactory
	mu                sync.RWMutex
	runtime           plugins.Runtime
	config            config.Config
	lastPrepareReport PrepareReport

	// Unload failure tracking for monitoring
	unloadFailures   []UnloadFailureRecord
	unloadFailuresMu sync.RWMutex
}

// NewPluginManager creates a generic plugin manager.
func NewPluginManager[T plugins.Plugin](pluginList ...T) *DefaultPluginManager[T] {
	manager := &DefaultPluginManager[T]{
		pluginList: make([]plugins.Plugin, 0),
		factory:    factory.GlobalTypedFactory(),
		runtime:    plugins.NewUnifiedRuntime(),
	}

	// register initial plugins
	for _, plugin := range pluginList {
		var p plugins.Plugin = plugin
		if p != nil {
			if err := manager.registerPreparedPlugin(p); err != nil {
				panic(err)
			}
		}
	}

	return manager
}

// NewTypedPluginManager creates a plugin manager with plugins.Plugin as T.
func NewTypedPluginManager(pluginList ...plugins.Plugin) TypedPluginManager {
	return NewPluginManager[plugins.Plugin](pluginList...)
}

// SetConfig sets global config.
func (m *DefaultPluginManager[T]) SetConfig(conf config.Config) {
	m.config = conf
	if m.runtime != nil {
		m.runtime.SetConfig(conf)
	}
}

// GetRuntime returns the shared runtime.
func (m *DefaultPluginManager[T]) GetRuntime() plugins.Runtime {
	return m.runtime
}

// GetPlugin gets a plugin by Name().
func (m *DefaultPluginManager[T]) GetPlugin(name string) plugins.Plugin {
	if value, ok := m.pluginInstances.Load(name); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	return nil
}

// GetPluginByID gets a plugin by ID().
func (m *DefaultPluginManager[T]) GetPluginByID(id string) plugins.Plugin {
	if value, ok := m.pluginIDs.Load(id); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	return nil
}

func (m *DefaultPluginManager[T]) GetPluginCapabilities(name string) (plugins.PluginCapabilities, error) {
	p := m.GetPlugin(name)
	if p == nil {
		return plugins.PluginCapabilities{}, fmt.Errorf("plugin %s not found", name)
	}
	return plugins.DescribePluginCapabilities(p), nil
}

// containsName checks if a name exists in the slice.
func containsName(slice []string, name string) bool {
	for _, item := range slice {
		if item == name {
			return true
		}
	}
	return false
}

// listPluginNamesInternal returns current plugin names (sorted).
// Optimized to use pluginList when available to avoid duplicate traversal
func (m *DefaultPluginManager[T]) listPluginNamesInternal() []string {
	if m == nil {
		return nil
	}

	// Use pluginList if available to avoid traversing sync.Map
	m.mu.RLock()
	if len(m.pluginList) > 0 {
		names := make([]string, 0, len(m.pluginList))
		for _, p := range m.pluginList {
			if p != nil {
				names = append(names, p.Name())
			}
		}
		m.mu.RUnlock()
		sort.Strings(names)
		return names
	}
	m.mu.RUnlock()

	// Fallback to sync.Map traversal if pluginList is empty
	names := make([]string, 0)
	m.pluginInstances.Range(func(key, value any) bool {
		if name, ok := key.(string); ok {
			names = append(names, name)
		}
		return true
	})
	sort.Strings(names)
	return names
}

// listPluginsInternal returns a copy of current plugins (read-locked).
func (m *DefaultPluginManager[T]) listPluginsInternal() []plugins.Plugin {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]plugins.Plugin, 0, len(m.pluginList))
	for _, p := range m.pluginList {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// GetTypedPluginFromManager gets a typed plugin from any TypedPluginManager.
func GetTypedPluginFromManager[T plugins.Plugin](m TypedPluginManager, name string) (T, error) {
	var zero T
	if m == nil {
		return zero, fmt.Errorf("plugin manager is nil")
	}
	p := m.GetPlugin(name)
	if p == nil {
		return zero, fmt.Errorf("plugin %s not found", name)
	}
	if typed, ok := p.(T); ok {
		return typed, nil
	}
	return zero, fmt.Errorf("plugin %s is not of expected type", name)
}

// MustGetTypedPluginFromManager gets typed plugin or panics.
func MustGetTypedPluginFromManager[T plugins.Plugin](m TypedPluginManager, name string) T {
	p, err := GetTypedPluginFromManager[T](m, name)
	if err != nil {
		panic(err)
	}
	return p
}

func (m *DefaultPluginManager[T]) GetLastPrepareReport() PrepareReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := PrepareReport{
		Prepared:       append([]string(nil), m.lastPrepareReport.Prepared...),
		Skipped:        append([]string(nil), m.lastPrepareReport.Skipped...),
		Failures:       append([]PrepareFailure(nil), m.lastPrepareReport.Failures...),
		PartialAllowed: m.lastPrepareReport.PartialAllowed,
	}
	return report
}

func (m *DefaultPluginManager[T]) setLastPrepareReport(report PrepareReport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastPrepareReport = report
}

func validatePluginIdentity(p plugins.Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin is nil")
	}
	if p.Name() == "" {
		return fmt.Errorf("plugin %T has empty Name()", p)
	}
	if p.ID() == "" {
		return fmt.Errorf("plugin %s has empty ID()", p.Name())
	}
	caps := plugins.DescribePluginCapabilities(p)
	if !caps.ProtocolExplicit {
		return fmt.Errorf("plugin %s must declare PluginProtocol()", p.Name())
	}
	if caps.IsTrulyContextAware && !caps.HasLifecycleWithCtx {
		return fmt.Errorf("plugin %s declares IsContextAware=true but does not implement LifecycleWithContext", p.Name())
	}
	if caps.Protocol.ContextLifecycle && !caps.HasLifecycleWithCtx {
		return fmt.Errorf("plugin %s declares context lifecycle protocol but does not implement LifecycleWithContext", p.Name())
	}
	if caps.Protocol.ContextLifecycle && !caps.IsTrulyContextAware {
		return fmt.Errorf("plugin %s declares context lifecycle protocol but IsContextAware() is false", p.Name())
	}
	if caps.Protocol.HealthAware && !caps.HasHealthCheck {
		return fmt.Errorf("plugin %s declares health-aware protocol but does not implement CheckHealth", p.Name())
	}
	return nil
}

func (m *DefaultPluginManager[T]) registerPreparedPlugin(p plugins.Plugin) error {
	if err := validatePluginIdentity(p); err != nil {
		return err
	}
	if existing, ok := m.pluginInstances.Load(p.Name()); ok {
		if existingPlugin, ok := existing.(plugins.Plugin); ok && existingPlugin.ID() != p.ID() {
			return fmt.Errorf("plugin name %s already registered with different ID %s", p.Name(), existingPlugin.ID())
		}
		return fmt.Errorf("plugin %s is already loaded", p.Name())
	}
	if existing, ok := m.pluginIDs.Load(p.ID()); ok {
		if existingPlugin, ok := existing.(plugins.Plugin); ok && existingPlugin.Name() != p.Name() {
			return fmt.Errorf("plugin ID %s already registered with different name %s", p.ID(), existingPlugin.Name())
		}
		return fmt.Errorf("plugin ID %s is already loaded", p.ID())
	}

	m.mu.Lock()
	m.pluginList = append(m.pluginList, p)
	m.mu.Unlock()
	m.pluginInstances.Store(p.Name(), p)
	m.pluginIDs.Store(p.ID(), p)
	return nil
}
