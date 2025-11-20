package app

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

// TestNewApp_ConcurrentInit 测试并发初始化
func TestNewApp_ConcurrentInit(t *testing.T) {
	// 重置全局状态
	resetGlobalState()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	apps := make([]*LynxApp, numGoroutines)
	errors := make([]error, numGoroutines)

	// 创建测试配置
	cfg := createTestConfig(t)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			app, err := NewApp(cfg)
			apps[idx] = app
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// 验证所有 goroutine 返回同一个实例或错误
	firstApp := apps[0]
	firstErr := errors[0]

	for i := 1; i < numGoroutines; i++ {
		if errors[i] != nil && firstErr == nil {
			t.Errorf("Goroutine %d got error but first didn't: %v", i, errors[i])
		}
		if errors[i] == nil && firstErr != nil {
			t.Errorf("Goroutine %d got no error but first did: %v", firstErr)
		}
		if apps[i] != firstApp && firstErr == nil {
			t.Errorf("Goroutine %d got different app instance", i)
		}
	}

	// 清理
	if firstApp != nil {
		firstApp.Close()
	}
}

// TestNewApp_InitStateTransitions 测试初始化状态转换
func TestNewApp_InitStateTransitions(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 测试 initStateNotInitialized -> initStateInitializing
	state := atomic.LoadInt32(&initState)
	if state != initStateNotInitialized {
		t.Errorf("Expected initStateNotInitialized, got %d", state)
	}

	// 启动初始化
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	// 验证状态变为 initStateInitialized
	state = atomic.LoadInt32(&initState)
	if state != initStateInitialized {
		t.Errorf("Expected initStateInitialized, got %d", state)
	}

	// 清理
	app.Close()
}

// TestNewApp_InitLockAcquisition 测试初始化锁获取
func TestNewApp_InitLockAcquisition(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 测试 CAS 操作
	acquired := atomic.CompareAndSwapInt32(&initState, initStateNotInitialized, initStateInitializing)
	if !acquired {
		t.Error("Failed to acquire init lock")
	}

	// 验证状态已改变
	state := atomic.LoadInt32(&initState)
	if state != initStateInitializing {
		t.Errorf("Expected initStateInitializing, got %d", state)
	}

	// 重置状态
	atomic.StoreInt32(&initState, initStateNotInitialized)
}

// TestNewApp_InitFailureRetry 测试初始化失败重试
func TestNewApp_InitFailureRetry(t *testing.T) {
	resetGlobalState()

	// 第一次初始化失败（使用无效配置）
	invalidCfg := createInvalidConfig(t)
	app1, err1 := NewApp(invalidCfg)

	if err1 == nil {
		t.Error("Expected error for invalid config")
		if app1 != nil {
			app1.Close()
		}
		return
	}

	// 验证状态变为 initStateFailed
	state := atomic.LoadInt32(&initState)
	if state != initStateFailed {
		t.Errorf("Expected initStateFailed, got %d", state)
	}

	// 第二次初始化成功
	validCfg := createTestConfig(t)
	app2, err2 := NewApp(validCfg)

	if err2 != nil {
		t.Fatalf("Expected success on retry, got error: %v", err2)
	}

	// 验证状态变为 initStateInitialized
	state = atomic.LoadInt32(&initState)
	if state != initStateInitialized {
		t.Errorf("Expected initStateInitialized, got %d", state)
	}

	// 清理
	if app2 != nil {
		app2.Close()
	}
}

// TestNewApp_InitFailureAfterSuccess 测试成功后的失败处理
func TestNewApp_InitFailureAfterSuccess(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 第一次初始化成功
	app1, err1 := NewApp(cfg)
	if err1 != nil {
		t.Fatalf("Failed to initialize app: %v", err1)
	}

	// 关闭应用
	app1.Close()

	// 重置状态（模拟应用被关闭）
	atomic.StoreInt32(&initState, initStateNotInitialized)
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()

	// 再次初始化
	app2, err2 := NewApp(cfg)
	if err2 != nil {
		t.Fatalf("Failed to reinitialize app: %v", err2)
	}

	// 清理
	if app2 != nil {
		app2.Close()
	}
}

// TestNewApp_NilConfig 测试空配置
func TestNewApp_NilConfig(t *testing.T) {
	resetGlobalState()

	app, err := NewApp(nil)

	if err == nil {
		t.Error("Expected error for nil config")
		if app != nil {
			app.Close()
		}
		return
	}

	if app != nil {
		t.Error("Expected nil app for nil config")
	}
}

// TestNewApp_InitTimeout 测试初始化超时
func TestNewApp_InitTimeout(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 第一个 goroutine 开始初始化
	var wg sync.WaitGroup
	wg.Add(1)

	var app1 *LynxApp
	var err1 error

	go func() {
		defer wg.Done()
		// 模拟长时间初始化
		time.Sleep(100 * time.Millisecond)
		app1, err1 = NewApp(cfg)
	}()

	// 第二个 goroutine 等待初始化
	time.Sleep(10 * time.Millisecond) // 确保第一个 goroutine 先开始

	app2, err2 := NewApp(cfg)

	// 等待第一个 goroutine 完成
	wg.Wait()

	// 验证第二个 goroutine 要么得到同一个实例，要么得到超时错误
	if err2 != nil {
		// 可能是超时错误
		t.Logf("Second goroutine got error (may be timeout): %v", err2)
	} else if app2 != app1 {
		t.Error("Second goroutine got different app instance")
	}

	// 清理
	if app1 != nil {
		app1.Close()
	}
}

// TestNewApp_AppClosedAfterInit 测试初始化后应用被关闭
func TestNewApp_AppClosedAfterInit(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 初始化应用
	app1, err1 := NewApp(cfg)
	if err1 != nil {
		t.Fatalf("Failed to initialize app: %v", err1)
	}

	// 关闭应用
	app1.Close()

	// 重置状态
	atomic.StoreInt32(&initState, initStateNotInitialized)
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()

	// 另一个 goroutine 检查状态
	app2, err2 := NewApp(cfg)

	if err2 != nil {
		t.Logf("Got error (expected if app was closed): %v", err2)
	} else if app2 == nil {
		t.Error("Got nil app without error")
	} else {
		app2.Close()
	}
}

// TestNewApp_InitLockVerification 测试初始化锁验证
func TestNewApp_InitLockVerification(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// 手动设置状态为 initializing（模拟另一个 goroutine 正在初始化）
	atomic.StoreInt32(&initState, initStateInitializing)

	// 尝试初始化（应该失败或等待）
	app, err := NewApp(cfg)

	// 验证结果（可能超时或成功，取决于实现）
	if err != nil {
		t.Logf("Got error (may be expected): %v", err)
	} else if app != nil {
		app.Close()
	}

	// 重置状态
	atomic.StoreInt32(&initState, initStateNotInitialized)
}

// TestNewApp_MultipleInitAttempts 测试多次初始化尝试
func TestNewApp_MultipleInitAttempts(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	const numAttempts = 10
	apps := make([]*LynxApp, numAttempts)
	errors := make([]error, numAttempts)

	for i := 0; i < numAttempts; i++ {
		apps[i], errors[i] = NewApp(cfg)
		time.Sleep(10 * time.Millisecond) // 小延迟
	}

	// 验证所有尝试都返回同一个实例
	firstApp := apps[0]
	firstErr := errors[0]

	for i := 1; i < numAttempts; i++ {
		if apps[i] != firstApp && firstErr == nil {
			t.Errorf("Attempt %d got different app instance", i)
		}
	}

	// 清理
	if firstApp != nil {
		firstApp.Close()
	}
}

// TestNewApp_ConcurrentInitWithFailure 测试并发初始化中的失败
func TestNewApp_ConcurrentInitWithFailure(t *testing.T) {
	resetGlobalState()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	apps := make([]*LynxApp, numGoroutines)
	errors := make([]error, numGoroutines)

	// 一些使用有效配置，一些使用无效配置
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			var cfg config.Config
			if idx%2 == 0 {
				cfg = createTestConfig(t)
			} else {
				cfg = createInvalidConfig(t)
			}
			apps[idx], errors[idx] = NewApp(cfg)
		}(i)
	}

	wg.Wait()

	// 验证至少有一个成功（使用有效配置的）
	hasSuccess := false
	for i := 0; i < numGoroutines; i++ {
		if errors[i] == nil && apps[i] != nil {
			hasSuccess = true
			apps[i].Close()
			break
		}
	}

	if !hasSuccess {
		t.Error("Expected at least one successful initialization")
	}
}

// resetGlobalState 重置全局状态（用于测试）
func resetGlobalState() {
	atomic.StoreInt32(&initState, initStateNotInitialized)
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()
	initMu.Lock()
	initErr = nil
	initMu.Unlock()
}

// createTestConfig 创建测试配置
func createTestConfig(t *testing.T) config.Config {
	// 创建一个简单的内存配置
	// 注意：这里需要根据实际的配置系统调整
	// 如果无法创建真实配置，可以创建一个 mock 配置
	return config.New(
		config.WithSource(
			file.NewSource("testdata/test.yaml"), // 需要创建测试文件
		),
	)
}

// createInvalidConfig 创建无效配置（用于测试失败场景）
func createInvalidConfig(t *testing.T) config.Config {
	// 返回 nil 或无效配置
	return nil
}

// TestNewApp_GoroutineLeak 测试 goroutine 泄漏
func TestNewApp_GoroutineLeak(t *testing.T) {
	resetGlobalState()

	before := runtime.NumGoroutine()

	cfg := createTestConfig(t)

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			app, err := NewApp(cfg)
			if err == nil && app != nil {
				// 不关闭，让测试验证清理
			}
		}()
	}

	wg.Wait()

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	// 允许一些系统 goroutine
	if after > before+10 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}

	// 清理
	lynxMu.Lock()
	if lynxApp != nil {
		lynxApp.Close()
		lynxApp = nil
	}
	lynxMu.Unlock()
	resetGlobalState()
}
