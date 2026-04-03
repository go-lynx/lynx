package plugins

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

type runtimeOwner struct {
	pluginID string
	handleID uint64
}

type runtimeSharedState struct {
	mu           sync.RWMutex
	config       config.Config
	logger       log.Logger
	eventAdapter EventBusAdapter
	closed       bool
}

const (
	sharedResourceKeyPrefix   = "__lynx_shared__:"
	privateResourceKeyPrefix  = "__lynx_private__:"
	privateResourceNamePrefix = "private:"
)

func sharedResourceStorageKey(name string) string {
	return sharedResourceKeyPrefix + name
}

func privateResourceStorageKey(pluginID, name string) string {
	return privateResourceKeyPrefix + pluginID + ":" + name
}

func privateResourceDisplayName(pluginID, name string) string {
	return privateResourceNamePrefix + pluginID + ":" + name
}

func parsePrivateResourceDisplayName(name string) (string, string, bool) {
	if !strings.HasPrefix(name, privateResourceNamePrefix) {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(name, privateResourceNamePrefix)
	idx := strings.Index(trimmed, ":")
	if idx <= 0 || idx == len(trimmed)-1 {
		return "", "", false
	}
	return trimmed[:idx], trimmed[idx+1:], true
}

func parseLegacyPrivateResourceDisplayName(name string) (string, string, bool) {
	idx := strings.Index(name, ":")
	if idx <= 0 || idx == len(name)-1 {
		return "", "", false
	}
	return name[:idx], name[idx+1:], true
}

// UnifiedRuntime is a unified Runtime implementation that consolidates all existing capabilities
type UnifiedRuntime struct {
	// Resource management - use sync.Map for better concurrent performance
	resources *sync.Map // map[string]any - stores all resources

	// Resource info tracking
	resourceInfo *sync.Map // map[string]*ResourceInfo
	resourceOpMu *sync.Mutex

	// Configuration and logging
	config config.Config
	logger log.Logger
	shared *runtimeSharedState

	// Plugin context management
	currentPluginContext string
	contextMu            sync.RWMutex
	owner                *runtimeOwner
	ownerHandles         *sync.Map
	ownerSeq             *atomic.Uint64

	// Event system - uses a unified event bus
	eventManager any // avoid circular dependency; set at runtime
	eventAdapter EventBusAdapter

	// Runtime state
	closed bool
	mu     sync.RWMutex

	// Context for graceful shutdown of background goroutines
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

// NewUnifiedRuntime creates a new unified Runtime instance
func NewUnifiedRuntime() *UnifiedRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	ownerSeq := &atomic.Uint64{}
	systemOwner := &runtimeOwner{pluginID: "system", handleID: ownerSeq.Add(1)}
	return &UnifiedRuntime{
		resources:    &sync.Map{},
		resourceInfo: &sync.Map{},
		resourceOpMu: &sync.Mutex{},
		logger:       log.DefaultLogger,
		shared: &runtimeSharedState{
			logger: log.DefaultLogger,
		},
		owner:          systemOwner,
		ownerHandles:   &sync.Map{},
		ownerSeq:       ownerSeq,
		closed:         false,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}
}

// ============================================================================
// Backward-compatible constructors
// ============================================================================

// NewSimpleRuntime and NewTypedRuntime are defined in plugin.go and delegate to NewUnifiedRuntime.
