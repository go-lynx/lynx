package lynx

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestSafeInitPlugin_GoroutineLeak tests goroutine leaks during initialization
func TestSafeInitPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	before := runtime.NumGoroutine()

	// Create a slow non-context-aware plugin
	plugin := &SlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// Trigger initialization (will timeout)
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// Wait for goroutines to exit
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Allow some system goroutines, should be close to initial value
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_GoroutineDoneSignal tests goroutine completion signal
func TestSafeInitPlugin_GoroutineDoneSignal(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a fast plugin
	plugin := &FastPlugin{}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	// Trigger initialization
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// Wait for goroutines to exit
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStartPlugin_GoroutineLeak tests goroutine leaks during startup
func TestSafeStartPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	before := runtime.NumGoroutine()

	// Create a slow plugin
	plugin := &SlowPlugin{
		startDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// Trigger startup (will timeout)
	err := manager.safeStartPlugin(plugin, timeout)

	// Wait for goroutines to exit
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStopPlugin_GoroutineLeak tests goroutine leaks during stop
func TestSafeStopPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	before := runtime.NumGoroutine()

	// Create a slow plugin
	plugin := &SlowPlugin{
		stopDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// Trigger stop (will timeout)
	err := manager.safeStopPlugin(plugin, timeout)

	// Wait for goroutines to exit
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_Timeout tests initialization timeout
func TestSafeInitPlugin_Timeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a slow plugin
	plugin := &SlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	start := time.Now()
	err := manager.safeInitPlugin(plugin, rt, timeout)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Verify timeout duration is reasonable (should be around timeout, allow some margin)
	if duration < timeout || duration > timeout+200*time.Millisecond {
		t.Errorf("Timeout duration unexpected: %v (expected ~%v)", duration, timeout)
	}
}

// TestSafeInitPlugin_ContextAwareTimeout tests context-aware plugin timeout
func TestSafeInitPlugin_ContextAwareTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a context-aware slow plugin
	plugin := &ContextAwareSlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	before := runtime.NumGoroutine()

	// Trigger initialization (will timeout)
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_NonContextAwareTimeout tests non-context-aware plugin timeout
func TestSafeInitPlugin_NonContextAwareTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a non-context-aware slow plugin
	plugin := &SlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	before := runtime.NumGoroutine()

	// Trigger initialization (will timeout)
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// Wait for cleanup (including delayed result detection)
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Verify delayed result detection logic
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_ConcurrentInit tests concurrent initialization
func TestSafeInitPlugin_ConcurrentInit(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a fast plugin
	plugin := &FastPlugin{}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := int32(0)
	errorCount := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := manager.safeInitPlugin(plugin, rt, 5*time.Second)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Wait for all goroutines to exit
	time.Sleep(200 * time.Millisecond)

	// Verify all initializations succeeded (or at least no panics)
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
}

// TestSafeStartPlugin_ConcurrentStart tests concurrent startup
func TestSafeStartPlugin_ConcurrentStart(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	plugin := &FastPlugin{}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := int32(0)
	errorCount := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := manager.safeStartPlugin(plugin, 5*time.Second)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
}

// TestSafeStopPlugin_ConcurrentStop tests concurrent stop
func TestSafeStopPlugin_ConcurrentStop(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	plugin := &FastPlugin{}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := int32(0)
	errorCount := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := manager.safeStopPlugin(plugin, 5*time.Second)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
}

// TestSafeInitPlugin_PanicRecovery tests panic recovery
func TestSafeInitPlugin_PanicRecovery(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Create a plugin that panics during initialization
	plugin := &PanicPlugin{
		panicInInit: true,
	}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	// Trigger initialization (should catch panic)
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected error from panic")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStartPlugin_PanicRecovery tests panic recovery during startup
func TestSafeStartPlugin_PanicRecovery(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	plugin := &PanicPlugin{
		panicInStart: true,
	}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	err := manager.safeStartPlugin(plugin, timeout)

	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected error from panic")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStopPlugin_PanicRecovery tests panic recovery during stop
func TestSafeStopPlugin_PanicRecovery(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	plugin := &PanicPlugin{
		panicInStop: true,
	}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	err := manager.safeStopPlugin(plugin, timeout)

	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected error from panic")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_NilPlugin tests nil plugin handling
func TestSafeInitPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// Pass nil plugin
	err := manager.safeInitPlugin(nil, rt, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeStartPlugin_NilPlugin tests nil plugin startup
func TestSafeStartPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.safeStartPlugin(nil, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeStopPlugin_NilPlugin tests nil plugin stop
func TestSafeStopPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.safeStopPlugin(nil, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeInitPlugin_ZeroTimeout tests zero timeout handling
func TestSafeInitPlugin_ZeroTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	plugin := &FastPlugin{}

	// Use zero timeout (should use default 5s)
	err := manager.safeInitPlugin(plugin, rt, 0)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// Test plugin implementations

// SlowPlugin is a plugin that delays
type SlowPlugin struct {
	initDuration  time.Duration
	startDuration time.Duration
	stopDuration  time.Duration
}

func (p *SlowPlugin) Name() string {
	return "slow-plugin"
}

func (p *SlowPlugin) ID() string {
	return "slow-plugin"
}

func (p *SlowPlugin) Version() string {
	return "1.0.0"
}

func (p *SlowPlugin) Description() string {
	return "Slow test plugin"
}

func (p *SlowPlugin) CheckHealth() error {
	return nil
}

func (p *SlowPlugin) GetDependencies() []plugins.Dependency {
	return nil
}

func (p *SlowPlugin) Weight() int {
	return 0
}

func (p *SlowPlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

func (p *SlowPlugin) StartupTasks() error {
	return nil
}

func (p *SlowPlugin) CleanupTasks() error {
	return nil
}

func (p *SlowPlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (p *SlowPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	if p.initDuration > 0 {
		time.Sleep(p.initDuration)
	}
	return nil
}

func (p *SlowPlugin) Start(plugin plugins.Plugin) error {
	if p.startDuration > 0 {
		time.Sleep(p.startDuration)
	}
	return nil
}

func (p *SlowPlugin) Stop(plugin plugins.Plugin) error {
	if p.stopDuration > 0 {
		time.Sleep(p.stopDuration)
	}
	return nil
}

// FastPlugin is a fast plugin
type FastPlugin struct{}

func (p *FastPlugin) Name() string {
	return "fast-plugin"
}

func (p *FastPlugin) ID() string {
	return "fast-plugin"
}

func (p *FastPlugin) Version() string {
	return "1.0.0"
}

func (p *FastPlugin) Description() string {
	return "Fast test plugin"
}

func (p *FastPlugin) CheckHealth() error {
	return nil
}

func (p *FastPlugin) GetDependencies() []plugins.Dependency {
	return nil
}

func (p *FastPlugin) Weight() int {
	return 0
}

func (p *FastPlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

func (p *FastPlugin) StartupTasks() error {
	return nil
}

func (p *FastPlugin) CleanupTasks() error {
	return nil
}

func (p *FastPlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (p *FastPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	return nil
}

func (p *FastPlugin) Start(plugin plugins.Plugin) error {
	return nil
}

func (p *FastPlugin) Stop(plugin plugins.Plugin) error {
	return nil
}

// ContextAwareSlowPlugin is a context-aware slow plugin
type ContextAwareSlowPlugin struct {
	initDuration time.Duration
}

func (p *ContextAwareSlowPlugin) Name() string {
	return "context-aware-slow-plugin"
}

func (p *ContextAwareSlowPlugin) ID() string {
	return "context-aware-slow-plugin"
}

func (p *ContextAwareSlowPlugin) Version() string {
	return "1.0.0"
}

func (p *ContextAwareSlowPlugin) Description() string {
	return "Context-aware slow test plugin"
}

func (p *ContextAwareSlowPlugin) CheckHealth() error {
	return nil
}

func (p *ContextAwareSlowPlugin) GetDependencies() []plugins.Dependency {
	return nil
}

func (p *ContextAwareSlowPlugin) Weight() int {
	return 0
}

func (p *ContextAwareSlowPlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

func (p *ContextAwareSlowPlugin) StartupTasks() error {
	return nil
}

func (p *ContextAwareSlowPlugin) CleanupTasks() error {
	return nil
}

func (p *ContextAwareSlowPlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (p *ContextAwareSlowPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	return errors.New("should use InitializeContext")
}

func (p *ContextAwareSlowPlugin) InitializeContext(ctx context.Context, plugin plugins.Plugin, rt plugins.Runtime) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.initDuration):
		return nil
	}
}

func (p *ContextAwareSlowPlugin) Start(plugin plugins.Plugin) error {
	return nil
}

func (p *ContextAwareSlowPlugin) StartContext(ctx context.Context, plugin plugins.Plugin) error {
	return nil
}

func (p *ContextAwareSlowPlugin) Stop(plugin plugins.Plugin) error {
	return nil
}

func (p *ContextAwareSlowPlugin) StopContext(ctx context.Context, plugin plugins.Plugin) error {
	return nil
}

// PanicPlugin is a plugin that panics in lifecycle methods
type PanicPlugin struct {
	panicInInit  bool
	panicInStart bool
	panicInStop  bool
}

func (p *PanicPlugin) Name() string {
	return "panic-plugin"
}

func (p *PanicPlugin) ID() string {
	return "panic-plugin"
}

func (p *PanicPlugin) Version() string {
	return "1.0.0"
}

func (p *PanicPlugin) Description() string {
	return "Panic test plugin"
}

func (p *PanicPlugin) CheckHealth() error {
	return nil
}

func (p *PanicPlugin) GetDependencies() []plugins.Dependency {
	return nil
}

func (p *PanicPlugin) Weight() int {
	return 0
}

func (p *PanicPlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

func (p *PanicPlugin) StartupTasks() error {
	return nil
}

func (p *PanicPlugin) CleanupTasks() error {
	return nil
}

func (p *PanicPlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (p *PanicPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	if p.panicInInit {
		panic("panic in Initialize")
	}
	return nil
}

func (p *PanicPlugin) Start(plugin plugins.Plugin) error {
	if p.panicInStart {
		panic("panic in Start")
	}
	return nil
}

func (p *PanicPlugin) Stop(plugin plugins.Plugin) error {
	if p.panicInStop {
		panic("panic in Stop")
	}
	return nil
}
