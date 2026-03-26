// Package lynx provides the core plugin orchestration framework used by Lynx applications.
package lynx

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// GetTypedResource retrieves a type-safe resource (standalone helper)
// Deprecated: prefer GetTypedResourceFromRuntime or plugins.GetTypedResource with an explicit plugins.Runtime.
func GetTypedResource[T any](r *TypedRuntimePlugin, name string) (T, error) {
	return GetTypedResourceFromRuntime[T](r.UnderlyingRuntime(), name)
}

// GetTypedResourceFromRuntime retrieves a type-safe resource from an explicit runtime.
func GetTypedResourceFromRuntime[T any](runtime plugins.Runtime, name string) (T, error) {
	var zero T
	if runtime == nil {
		return zero, fmt.Errorf("runtime is nil")
	}
	return plugins.GetTypedResource[T](runtime, name)
}

// RegisterTypedResource registers a type-safe resource (standalone helper)
// Deprecated: prefer RegisterTypedResourceOnRuntime or plugins.RegisterTypedResource with an explicit plugins.Runtime.
func RegisterTypedResource[T any](r *TypedRuntimePlugin, name string, resource T) error {
	return RegisterTypedResourceOnRuntime[T](r.UnderlyingRuntime(), name, resource)
}

// RegisterTypedResourceOnRuntime registers a typed resource on an explicit runtime.
func RegisterTypedResourceOnRuntime[T any](runtime plugins.Runtime, name string, resource T) error {
	if runtime == nil {
		return fmt.Errorf("runtime is nil")
	}
	return plugins.RegisterTypedResource[T](runtime, name, resource)
}

// NewDefaultRuntime creates a unified runtime configured with the default logger.
func NewDefaultRuntime() plugins.Runtime {
	runtime := plugins.NewUnifiedRuntime()
	runtime.SetLogger(log.DefaultLogger)
	return runtime
}
