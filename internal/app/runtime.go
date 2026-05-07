// Package app provides the core plugin orchestration framework used by Lynx applications.
package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// GetTypedResourceFromRuntime retrieves a type-safe resource from an explicit runtime.
func GetTypedResourceFromRuntime[T any](runtime plugins.Runtime, name string) (T, error) {
	var zero T
	if runtime == nil {
		return zero, fmt.Errorf("runtime is nil")
	}
	return plugins.GetTypedResource[T](runtime, name)
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
