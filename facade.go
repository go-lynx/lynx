// Package lynx provides the core application framework for building Go microservices.
//
// The implementation lives in internal/app; this file re-exports the public API
// so that callers continue to use import paths of the form github.com/go-lynx/lynx.
package lynx

import (
	iapp "github.com/go-lynx/lynx/internal/app"
	"github.com/go-lynx/lynx/plugins"

	"github.com/go-kratos/kratos/v2/log"
)

// ── Core app type ──────────────────────────────────────────────────────────────

// LynxApp is the main application type. See internal/app for the implementation.
type LynxApp = iapp.LynxApp

// ── Plugin manager ─────────────────────────────────────────────────────────────

type PluginManager = iapp.PluginManager
type DefaultPluginManager[T plugins.Plugin] = iapp.DefaultPluginManager[T]
type UnloadFailureRecord = iapp.UnloadFailureRecord
type PrepareFailure = iapp.PrepareFailure
type PrepareReport = iapp.PrepareReport
type ConfigReloadEntry = iapp.ConfigReloadEntry
type RestartRequirementReport = iapp.RestartRequirementReport

func NewPluginManager[T plugins.Plugin](pluginList ...T) *iapp.DefaultPluginManager[T] {
	return iapp.NewPluginManager(pluginList...)
}

func NewPluginManagerWithError[T plugins.Plugin](pluginList ...T) (*iapp.DefaultPluginManager[T], error) {
	return iapp.NewPluginManagerWithError(pluginList...)
}

var NewTypedPluginManager = iapp.NewTypedPluginManager

func GetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) (T, error) {
	return iapp.GetTypedPluginFromManager[T](m, name)
}

func MustGetTypedPluginFromManager[T plugins.Plugin](m PluginManager, name string) T {
	return iapp.MustGetTypedPluginFromManager[T](m, name)
}

func ListPluginNames(m PluginManager) []string {
	return iapp.ListPluginNames(m)
}

func Plugins(m PluginManager) []plugins.Plugin {
	return iapp.Plugins(m)
}

// ── Control plane ──────────────────────────────────────────────────────────────

type ControlPlane = iapp.ControlPlane
type SystemCore = iapp.SystemCore
type RateLimiter = iapp.RateLimiter
type ServiceRegistry = iapp.ServiceRegistry
type RouteManager = iapp.RouteManager
type ConfigManager = iapp.ConfigManager
type DefaultControlPlane = iapp.DefaultControlPlane
type MultiConfigControlPlane = iapp.MultiConfigControlPlane
type ControlPlaneCapability = iapp.ControlPlaneCapability
type ControlPlaneCapabilityReporter = iapp.ControlPlaneCapabilityReporter
type ControlPlaneConfigTarget = iapp.ControlPlaneConfigTarget
type ControlPlaneConfigWatcherProvider = iapp.ControlPlaneConfigWatcherProvider

const (
	ControlPlaneCapabilityConfig            = iapp.ControlPlaneCapabilityConfig
	ControlPlaneCapabilityRegistry          = iapp.ControlPlaneCapabilityRegistry
	ControlPlaneCapabilityDiscovery         = iapp.ControlPlaneCapabilityDiscovery
	ControlPlaneCapabilityRouter            = iapp.ControlPlaneCapabilityRouter
	ControlPlaneCapabilityRateLimit         = iapp.ControlPlaneCapabilityRateLimit
	ControlPlaneCapabilityTrafficProtection = iapp.ControlPlaneCapabilityTrafficProtection
	ControlPlaneCapabilityWatcher           = iapp.ControlPlaneCapabilityWatcher
)

var ControlPlaneCapabilitiesOf = iapp.ControlPlaneCapabilitiesOf
var ControlPlaneCapabilityResourceName = iapp.ControlPlaneCapabilityResourceName
var RegisterControlPlaneCapabilityResources = iapp.RegisterControlPlaneCapabilityResources
var StartControlPlaneWatcher = iapp.StartControlPlaneWatcher

// ── Certificate ────────────────────────────────────────────────────────────────

type CertificateProvider = iapp.CertificateProvider

// ── Circuit breaker ────────────────────────────────────────────────────────────

type CircuitBreaker = iapp.CircuitBreaker
type CircuitState = iapp.CircuitState

const (
	CircuitStateClosed   = iapp.CircuitStateClosed
	CircuitStateOpen     = iapp.CircuitStateOpen
	CircuitStateHalfOpen = iapp.CircuitStateHalfOpen
)

var NewCircuitBreaker = iapp.NewCircuitBreaker

// ── Recovery ───────────────────────────────────────────────────────────────────

type ErrorRecoveryManager = iapp.ErrorRecoveryManager
type ErrorRecord = iapp.ErrorRecord
type RecoveryRecord = iapp.RecoveryRecord
type ErrorSeverity = iapp.ErrorSeverity
type ErrorCategory = iapp.ErrorCategory
type RecoveryStrategy = iapp.RecoveryStrategy
type DefaultRecoveryStrategy = iapp.DefaultRecoveryStrategy

const (
	ErrorSeverityLow      = iapp.ErrorSeverityLow
	ErrorSeverityMedium   = iapp.ErrorSeverityMedium
	ErrorSeverityHigh     = iapp.ErrorSeverityHigh
	ErrorSeverityCritical = iapp.ErrorSeverityCritical

	ErrorCategoryNetwork    = iapp.ErrorCategoryNetwork
	ErrorCategoryDatabase   = iapp.ErrorCategoryDatabase
	ErrorCategoryConfig     = iapp.ErrorCategoryConfig
	ErrorCategoryPlugin     = iapp.ErrorCategoryPlugin
	ErrorCategoryResource   = iapp.ErrorCategoryResource
	ErrorCategorySecurity   = iapp.ErrorCategorySecurity
	ErrorCategoryTimeout    = iapp.ErrorCategoryTimeout
	ErrorCategoryValidation = iapp.ErrorCategoryValidation
	ErrorCategorySystem     = iapp.ErrorCategorySystem
)

var NewErrorRecoveryManager = iapp.NewErrorRecoveryManager
var NewDefaultRecoveryStrategy = iapp.NewDefaultRecoveryStrategy

// ── Topology ──────────────────────────────────────────────────────────────────

type PluginWithLevel = iapp.PluginWithLevel

// ── Reports ────────────────────────────────────────────────────────────────────

type PluginRuntimeReport = iapp.PluginRuntimeReport
type CoreRuntimeReport = iapp.CoreRuntimeReport

// ── Constructors and helpers ───────────────────────────────────────────────────

var NewStandaloneApp = iapp.NewStandaloneApp
var NewDefaultRuntime = iapp.NewDefaultRuntime

func GetTypedResourceFromRuntime[T any](runtime plugins.Runtime, name string) (T, error) {
	return iapp.GetTypedResourceFromRuntime[T](runtime, name)
}

func RegisterTypedResourceOnRuntime[T any](runtime plugins.Runtime, name string, resource T) error {
	return iapp.RegisterTypedResourceOnRuntime[T](runtime, name, resource)
}

var GetEventManagerFromApp = iapp.GetEventManagerFromApp
var GetEventListenerManagerFromApp = iapp.GetEventListenerManagerFromApp
var GetEventAdapterFromApp = iapp.GetEventAdapterFromApp
var GetPluginManagerFromApp = iapp.GetPluginManagerFromApp
var GetGlobalConfigFromApp = iapp.GetGlobalConfigFromApp

// ── Logger (re-export for doc completeness) ────────────────────────────────────

// DefaultLogger re-exports kratos log.DefaultLogger for callers that import only lynx.
var DefaultLogger = log.DefaultLogger
