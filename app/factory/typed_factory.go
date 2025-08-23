package factory

import (
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// TypedFactory type-safe plugin factory
type TypedFactory struct {
	// creators stores plugin creation functions
	creators map[string]func() plugins.Plugin
	// configMapping configuration prefix mapping
	configMapping map[string][]string
	// mu read-write lock for concurrent access protection
	mu sync.RWMutex
}

// RegisterPlugin registers a plugin (backward-compatible signature for TypedFactory).
// Recommended usage in plugins: factory.GlobalTypedFactory().RegisterPlugin(...)
func (f *TypedFactory) RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin) {
    f.mu.Lock()
    defer f.mu.Unlock()

    // Check if already registered
    if _, exists := f.creators[name]; exists {
        panic(fmt.Errorf("plugin already registered: %s", name))
    }

    // Cross-prefix duplicate name detection (defensive)
    for prefix, names := range f.configMapping {
        if prefix == configPrefix {
            continue
        }
        for _, n := range names {
            if n == name {
                log.Warnf("plugin name '%s' registered under multiple prefixes: existing='%s', new='%s'", name, prefix, configPrefix)
            }
        }
    }

    // Store creation function
    f.creators[name] = creator

    // Configuration mapping
    if f.configMapping[configPrefix] == nil {
        f.configMapping[configPrefix] = make([]string, 0)
    }
    // Deduplicate within prefix
    exists := false
    for _, n := range f.configMapping[configPrefix] {
        if n == name {
            exists = true
            break
        }
    }
    if !exists {
        f.configMapping[configPrefix] = append(f.configMapping[configPrefix], name)
    }
}

// NewTypedFactory creates a type-safe plugin factory
func NewTypedFactory() *TypedFactory {
	return &TypedFactory{
		creators:      make(map[string]func() plugins.Plugin),
		configMapping: make(map[string][]string),
	}
}

// RegisterTypedPlugin registers a type-safe plugin
func RegisterTypedPlugin[T plugins.Plugin](
	factory *TypedFactory,
	name string,
	configPrefix string,
	creator func() T,
) {
	// Delegate to non-generic RegisterPlugin for centralized logic
	factory.RegisterPlugin(name, configPrefix, func() plugins.Plugin { return creator() })
}

// GetTypedPlugin gets a type-safe plugin instance
func GetTypedPlugin[T plugins.Plugin](factory *TypedFactory, name string) (T, error) {
	var zero T

	factory.mu.RLock()
	creator, exists := factory.creators[name]
	factory.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("plugin %s not found", name)
	}

	plugin := creator()
	typed, ok := plugin.(T)
	if !ok {
		return zero, fmt.Errorf("plugin %s failed type assertion", name)
	}

	return typed, nil
}

// CreateTypedPlugin creates a type-safe plugin instance
func CreateTypedPlugin[T plugins.Plugin](factory *TypedFactory, name string) (T, error) {
	return GetTypedPlugin[T](factory, name)
}

// HasPlugin checks if plugin exists
func (f *TypedFactory) HasPlugin(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.creators[name]
	return exists
}

// GetConfigMapping gets configuration mapping
func (f *TypedFactory) GetConfigMapping() map[string][]string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a copy to avoid concurrent modification
	result := make(map[string][]string)
	for k, v := range f.configMapping {
		result[k] = make([]string, len(v))
		copy(result[k], v)
	}
	return result
}

// CreatePlugin creates plugin instance (compatible with old interface)
func (f *TypedFactory) CreatePlugin(name string) (plugins.Plugin, error) {
	f.mu.RLock()
	creator, exists := f.creators[name]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	return creator(), nil
}

// GetPluginRegistry returns the plugin registry (backward-compatible API).
func (f *TypedFactory) GetPluginRegistry() map[string][]string {
	return f.GetConfigMapping()
}

// UnregisterPlugin unregisters a plugin.
func (f *TypedFactory) UnregisterPlugin(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Remove from creator mapping
	delete(f.creators, name)

	// Remove from configuration mapping
	for prefix, pluginList := range f.configMapping {
		for i, plugin := range pluginList {
			if plugin == name {
				// Remove the plugin
				f.configMapping[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// If no plugins remain under this prefix, delete the prefix entry
				if len(f.configMapping[prefix]) == 0 {
					delete(f.configMapping, prefix)
				}
				break
			}
		}
	}
}

// Global typed factory instance
var (
	globalTypedFactory *TypedFactory
	typedFactoryOnce   sync.Once
)

// GlobalTypedFactory returns the global type-safe plugin factory.
func GlobalTypedFactory() *TypedFactory {
	typedFactoryOnce.Do(func() {
		globalTypedFactory = NewTypedFactory()
	})
	return globalTypedFactory
}
