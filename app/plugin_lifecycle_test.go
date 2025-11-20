package app

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

// TestSafeInitPlugin_GoroutineLeak 测试初始化时的 goroutine 泄漏
func TestSafeInitPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	before := runtime.NumGoroutine()

	// 创建一个会超时的非 context-aware 插件
	plugin := &SlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// 触发初始化（会超时）
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// 等待 goroutine 退出
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	// 允许一些系统 goroutine，但应该接近初始值
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_GoroutineDoneSignal 测试 goroutine 完成信号
func TestSafeInitPlugin_GoroutineDoneSignal(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个快速完成的插件
	plugin := &FastPlugin{}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	// 触发初始化
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// 等待 goroutine 退出
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStartPlugin_GoroutineLeak 测试启动时的 goroutine 泄漏
func TestSafeStartPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	before := runtime.NumGoroutine()

	// 创建一个会超时的插件
	plugin := &SlowPlugin{
		startDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// 触发启动（会超时）
	err := manager.safeStartPlugin(plugin, timeout)

	// 等待 goroutine 退出
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStopPlugin_GoroutineLeak 测试停止时的 goroutine 泄漏
func TestSafeStopPlugin_GoroutineLeak(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	before := runtime.NumGoroutine()

	// 创建一个会超时的插件
	plugin := &SlowPlugin{
		stopDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	// 触发停止（会超时）
	err := manager.safeStopPlugin(plugin, timeout)

	// 等待 goroutine 退出
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_Timeout 测试初始化超时
func TestSafeInitPlugin_Timeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个会超时的插件
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

	// 验证超时时间合理（应该在 timeout 附近，但允许一些误差）
	if duration < timeout || duration > timeout+200*time.Millisecond {
		t.Errorf("Timeout duration unexpected: %v (expected ~%v)", duration, timeout)
	}
}

// TestSafeInitPlugin_ContextAwareTimeout 测试 context-aware 插件超时
func TestSafeInitPlugin_ContextAwareTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个 context-aware 的插件
	plugin := &ContextAwareSlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	before := runtime.NumGoroutine()

	// 触发初始化（会超时）
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_NonContextAwareTimeout 测试非 context-aware 插件超时
func TestSafeInitPlugin_NonContextAwareTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个非 context-aware 的插件
	plugin := &SlowPlugin{
		initDuration: 10 * time.Second,
	}

	timeout := 100 * time.Millisecond

	before := runtime.NumGoroutine()

	// 触发初始化（会超时）
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// 等待清理（包括延迟结果检测）
	time.Sleep(600 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected timeout error")
	}

	// 验证延迟结果检测逻辑
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeInitPlugin_ConcurrentInit 测试并发初始化
func TestSafeInitPlugin_ConcurrentInit(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个快速完成的插件
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

	// 等待所有 goroutine 退出
	time.Sleep(200 * time.Millisecond)

	// 验证所有初始化都成功（或至少没有 panic）
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)
}

// TestSafeStartPlugin_ConcurrentStart 测试并发启动
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

// TestSafeStopPlugin_ConcurrentStop 测试并发停止
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

// TestSafeInitPlugin_PanicRecovery 测试 panic 恢复
func TestSafeInitPlugin_PanicRecovery(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 创建一个会在初始化时 panic 的插件
	plugin := &PanicPlugin{
		panicInInit: true,
	}

	timeout := 5 * time.Second

	before := runtime.NumGoroutine()

	// 触发初始化（应该捕获 panic）
	err := manager.safeInitPlugin(plugin, rt, timeout)

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if err == nil {
		t.Error("Expected error from panic")
	}

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestSafeStartPlugin_PanicRecovery 测试启动时的 panic 恢复
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

// TestSafeStopPlugin_PanicRecovery 测试停止时的 panic 恢复
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

// TestSafeInitPlugin_NilPlugin 测试 nil 插件
func TestSafeInitPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	// 传入 nil 插件
	err := manager.safeInitPlugin(nil, rt, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeStartPlugin_NilPlugin 测试 nil 插件启动
func TestSafeStartPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.safeStartPlugin(nil, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeStopPlugin_NilPlugin 测试 nil 插件停止
func TestSafeStopPlugin_NilPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.safeStopPlugin(nil, 5*time.Second)

	if err != nil {
		t.Errorf("Expected no error for nil plugin, got %v", err)
	}
}

// TestSafeInitPlugin_ZeroTimeout 测试零超时
func TestSafeInitPlugin_ZeroTimeout(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	plugin := &FastPlugin{}

	// 使用零超时（应该使用默认值 5s）
	err := manager.safeInitPlugin(plugin, rt, 0)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// 测试用的插件实现

// SlowPlugin 是一个会延迟的插件
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

func (p *SlowPlugin) GetDependencies() []string {
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

// FastPlugin 是一个快速完成的插件
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

func (p *FastPlugin) GetDependencies() []string {
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

// ContextAwareSlowPlugin 是一个 context-aware 的慢插件
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

func (p *ContextAwareSlowPlugin) GetDependencies() []string {
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

// PanicPlugin 是一个会在生命周期方法中 panic 的插件
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

func (p *PanicPlugin) GetDependencies() []string {
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
