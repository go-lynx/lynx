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

// Compile-time check: ensure DefaultPluginManager implements PluginManager.
var _ PluginManager = (*DefaultPluginManager[plugins.Plugin])(nil)

// PluginManager defines plugin management interfaces.
type PluginManager interface {
	// Basic plugin management
	LoadPlugins(config.Config) error
	UnloadPlugins()
	LoadPluginsByName(config.Config, []string) error
	UnloadPluginsByName([]string)
	GetPlugin(name string) plugins.Plugin
	GetPluginByID(id string) plugins.Plugin
	GetPluginCapabilities(name string) (plugins.PluginCapabilities, error)
	GetRestartRequirementReport() RestartRequirementReport
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

type ConfigReloadEntry struct {
	PluginName string
	PluginID   string
	Reason     string
}

// RestartRequirementReport is the core-facing summary for configuration changes.
// Lynx applies configuration changes by restart instead of in-process reload.
type RestartRequirementReport struct {
	RestartRequired []ConfigReloadEntry
	Invalid         []ConfigReloadEntry
}

// DefaultPluginManager is the generic plugin manager implementation.
type DefaultPluginManager[T plugins.Plugin] struct {
	pluginInstances   sync.Map // Name() -> known/prepared plugin instance
	pluginIDs         sync.Map // ID() -> known/prepared plugin instance
	pluginList        []plugins.Plugin
	managedInstances  sync.Map // Name() -> lifecycle-managed plugin instance
	managedIDs        sync.Map // ID() -> lifecycle-managed plugin instance
	managedPluginList []plugins.Plugin
	factory           *factory.TypedFactory
	mu                sync.RWMutex
	operationMu       sync.Mutex
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
		pluginList:        make([]plugins.Plugin, 0),
		managedPluginList: make([]plugins.Plugin, 0),
		factory:           factory.GlobalTypedFactory(),
		runtime:           plugins.NewUnifiedRuntime(),
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
//
// Deprecated: use NewPluginManager[plugins.Plugin] or NewPluginManager directly.
// Retained for backward compatibility; will be removed in a future version.
func NewTypedPluginManager(pluginList ...plugins.Plugin) PluginManager {
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
	if value, ok := m.managedInstances.Load(name); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	if value, ok := m.pluginInstances.Load(name); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
	return nil
}

// GetPluginByID gets a plugin by ID().
func (m *DefaultPluginManager[T]) GetPluginByID(id string) plugins.Plugin {
	if value, ok := m.managedIDs.Load(id); ok {
		if plugin, ok := value.(plugins.Plugin); ok {
			return plugin
		}
	}
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

func (m *DefaultPluginManager[T]) GetRestartRequirementReport() RestartRequirementReport {
	report := RestartRequirementReport{
		RestartRequired: make([]ConfigReloadEntry, 0),
		Invalid:         make([]ConfigReloadEntry, 0),
	}
	for _, p := range m.listPluginsInternal() {
		if p == nil {
			continue
		}
		entry := ConfigReloadEntry{
			PluginName: p.Name(),
			PluginID:   p.ID(),
		}
		_, configurable := p.(plugins.Configurable)
		_, validator := p.(plugins.ConfigValidator)
		_, rollbacker := p.(plugins.ConfigRollbacker)

		// ConfigHotReload / ConfigValidation / ConfigRollback fields were removed
		// from PluginProtocol because Lynx core is restart-based.  The only relevant
		// distinction now is whether the plugin implements any of the legacy compat
		// interfaces (Configurable, ConfigValidator, ConfigRollbacker).
		if configurable || validator || rollbacker {
			entry.Reason = "plugin exposes configuration hooks, but lynx core requires restart to apply configuration changes"
		} else {
			entry.Reason = "lynx core applies configuration changes by restart"
		}
		report.RestartRequired = append(report.RestartRequired, entry)
	}
	return report
}

// filterOutPluginByIdentity compacts list in place: removes nil slots, entries matching
// name+id, and returns the shortened slice (same backing array as list).
func filterOutPluginByIdentity(list []plugins.Plugin, name, id string) []plugins.Plugin {
	if len(list) == 0 {
		return list
	}
	filtered := list[:0]
	for _, item := range list {
		if item == nil {
			continue
		}
		if item.Name() == name && item.ID() == id {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
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

// listPluginNamesInternal returns the names of lifecycle-managed plugins (sorted).
// Prepared-only plugins are intentionally excluded so runtime-facing queries only
// report plugins that actually entered manager-controlled lifecycle.
func (m *DefaultPluginManager[T]) listPluginNamesInternal() []string {
	if m == nil {
		return nil
	}

	// Use managedPluginList if available to avoid traversing sync.Map
	m.mu.RLock()
	if len(m.managedPluginList) > 0 {
		names := make([]string, 0, len(m.managedPluginList))
		for _, p := range m.managedPluginList {
			if p != nil {
				names = append(names, p.Name())
			}
		}
		m.mu.RUnlock()
		sort.Strings(names)
		return names
	}
	m.mu.RUnlock()

	// Fallback to sync.Map traversal if managedPluginList is empty
	names := make([]string, 0)
	m.managedInstances.Range(func(key, value any) bool {
		if name, ok := key.(string); ok {
			names = append(names, name)
		}
		return true
	})
	sort.Strings(names)
	return names
}

// listPluginsInternal returns a copy of lifecycle-managed plugins (read-locked).
func (m *DefaultPluginManager[T]) listPluginsInternal() []plugins.Plugin {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]plugins.Plugin, 0, len(m.managedPluginList))
	for _, p := range m.managedPluginList {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// GetTypedPluginFromManager gets a typed plugin from any PluginManager.
func GetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) (T, error) {
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
func MustGetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) T {
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

func (m *DefaultPluginManager[T]) registerManagedPlugin(p plugins.Plugin) error {
	if m == nil || p == nil {
		return fmt.Errorf("plugin is nil")
	}
	if existing, ok := m.managedInstances.Load(p.Name()); ok {
		if existingPlugin, ok := existing.(plugins.Plugin); ok && existingPlugin.ID() != p.ID() {
			return fmt.Errorf("managed plugin name %s already registered with different ID %s", p.Name(), existingPlugin.ID())
		}
		return nil
	}
	if existing, ok := m.managedIDs.Load(p.ID()); ok {
		if existingPlugin, ok := existing.(plugins.Plugin); ok && existingPlugin.Name() != p.Name() {
			return fmt.Errorf("managed plugin ID %s already registered with different name %s", p.ID(), existingPlugin.Name())
		}
		return nil
	}

	m.mu.Lock()
	m.managedPluginList = append(m.managedPluginList, p)
	m.mu.Unlock()
	m.managedInstances.Store(p.Name(), p)
	m.managedIDs.Store(p.ID(), p)
	m.removePreparedPlugin(p)
	return nil
}

func (m *DefaultPluginManager[T]) removePreparedPlugin(p plugins.Plugin) {
	if m == nil || p == nil {
		return
	}

	m.pluginInstances.Delete(p.Name())
	m.pluginIDs.Delete(p.ID())

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pluginList = filterOutPluginByIdentity(m.pluginList, p.Name(), p.ID())
}

func (m *DefaultPluginManager[T]) unregisterPlugin(p plugins.Plugin) {
	if m == nil || p == nil {
		return
	}

	name, id := p.Name(), p.ID()
	m.pluginInstances.Delete(name)
	m.pluginIDs.Delete(id)
	m.managedInstances.Delete(name)
	m.managedIDs.Delete(id)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pluginList = filterOutPluginByIdentity(m.pluginList, name, id)
	m.managedPluginList = filterOutPluginByIdentity(m.managedPluginList, name, id)
}
