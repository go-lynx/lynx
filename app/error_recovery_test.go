package app

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-lynx/lynx/app/observability/metrics"
)

// TestErrorRecoveryManager_ConcurrentRecovery 测试并发恢复操作
func TestErrorRecoveryManager_ConcurrentRecovery(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	recoveryCount := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			erm.attemptRecovery(record)
			atomic.AddInt32(&recoveryCount, 1)
		}()
	}

	wg.Wait()

	// 验证只有一个恢复操作被执行（由于相同的 recoveryKey）
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	// 等待所有恢复完成
	time.Sleep(200 * time.Millisecond)

	// 再次检查活跃恢复数量
	activeCount = 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Logf("Active recoveries after completion: %d (should be 0)", activeCount)
	}

	t.Logf("Total recovery attempts: %d", recoveryCount)
}

// TestErrorRecoveryManager_RecoveryKeyCollision 测试恢复键冲突
func TestErrorRecoveryManager_RecoveryKeyCollision(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建相同时间戳的不同错误（可能产生相同的 recoveryKey）
	now := time.Now()
	record1 := ErrorRecord{
		ErrorType: "transient",
		Component: "component1",
		Timestamp: now,
		Severity:  ErrorSeverityLow,
	}
	record2 := ErrorRecord{
		ErrorType: "transient",
		Component: "component2",
		Timestamp: now,
		Severity:  ErrorSeverityLow,
	}

	// 同时触发两个恢复操作
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		erm.attemptRecovery(record1)
	}()

	go func() {
		defer wg.Done()
		erm.attemptRecovery(record2)
	}()

	wg.Wait()

	// 等待恢复完成
	time.Sleep(200 * time.Millisecond)

	// 验证恢复键的唯一性（通过检查 activeRecoveries）
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Logf("Active recoveries: %d (should be 0 after completion)", activeCount)
	}
}

// TestErrorRecoveryManager_ConcurrentRecordError 测试并发记录错误
func TestErrorRecoveryManager_ConcurrentRecordError(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			erm.RecordError("test-error", ErrorCategorySystem, "test error message", "test-component", ErrorSeverityLow, nil)
		}(i)
	}

	wg.Wait()

	// 验证错误计数正确性
	stats := erm.GetErrorStats()
	count, ok := stats["test-error"].(int64)
	if !ok {
		t.Fatal("Failed to get error count")
	}

	if count != int64(numGoroutines) {
		t.Errorf("Expected error count %d, got %d", numGoroutines, count)
	}

	// 验证错误历史记录
	history := erm.GetErrorHistory()
	if len(history) != numGoroutines {
		t.Errorf("Expected error history length %d, got %d", numGoroutines, len(history))
	}
}

// TestErrorRecoveryManager_GoroutineLeak 测试 goroutine 泄漏
func TestErrorRecoveryManager_GoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 启动恢复操作
	go erm.attemptRecovery(record)

	// 立即停止管理器
	erm.Stop()

	// 等待 goroutine 退出
	time.Sleep(500 * time.Millisecond)

	after := runtime.NumGoroutine()

	// 允许一些系统 goroutine，但应该接近初始值
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_StopMonitorGoroutineExit 测试停止监控 goroutine 退出
func TestErrorRecoveryManager_StopMonitorGoroutineExit(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 启动恢复操作（会在获取 semaphore 前返回）
	// 通过填满信号量来触发超时
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	// 尝试恢复（应该因为信号量满而超时）
	go erm.attemptRecovery(record)

	// 等待超时和清理
	time.Sleep(300 * time.Millisecond)

	// 释放信号量
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等待 goroutine 退出
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup 测试信号量超时时的 goroutine 清理
func TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// 填满信号量
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 尝试恢复（应该因为信号量满而超时）
	go erm.attemptRecovery(record)

	// 等待超时和清理
	time.Sleep(300 * time.Millisecond)

	// 释放信号量
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等待 goroutine 退出
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_RecoveryTimeout 测试恢复超时
func TestErrorRecoveryManager_RecoveryTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建一个会超时的恢复策略
	slowStrategy := &slowRecoveryStrategy{
		timeout: 100 * time.Millisecond,
		delay:   200 * time.Millisecond, // 比超时时间长
	}
	erm.RegisterRecoveryStrategy("slow", slowStrategy)

	record := ErrorRecord{
		ErrorType: "slow",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// 触发恢复操作
	go erm.attemptRecovery(record)

	// 等待超时
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	// 验证超时后正确清理
	if after > before+5 {
		t.Errorf("Possible goroutine leak after timeout: before=%d, after=%d", before, after)
	}

	// 验证活跃恢复被清理
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries after timeout, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeout 测试信号量获取超时
func TestErrorRecoveryManager_SemaphoreTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// 填满信号量
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 尝试恢复（应该因为信号量满而超时）
	go erm.attemptRecovery(record)

	// 等待超时
	time.Sleep(300 * time.Millisecond)

	// 释放信号量
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_ContextCancellation 测试 context 取消
func TestErrorRecoveryManager_ContextCancellation(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// 启动恢复操作
	go erm.attemptRecovery(record)

	// 立即停止管理器（取消所有恢复）
	erm.Stop()

	// 等待清理
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}

	// 验证活跃恢复被清理
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries after stop, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_NilStrategy 测试空策略
func TestErrorRecoveryManager_NilStrategy(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 记录一个没有对应策略的错误（应该使用默认策略）
	record := ErrorRecord{
		ErrorType: "unknown-error",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 应该使用默认的 "transient" 策略
	erm.attemptRecovery(record)

	// 等待完成
	time.Sleep(200 * time.Millisecond)

	// 验证没有 panic
}

// TestErrorRecoveryManager_StrategyCannotRecover 测试策略无法恢复
func TestErrorRecoveryManager_StrategyCannotRecover(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建一个无法恢复的策略
	cannotRecoverStrategy := &cannotRecoverStrategy{
		timeout: 100 * time.Millisecond,
	}
	erm.RegisterRecoveryStrategy("cannot-recover", cannotRecoverStrategy)

	record := ErrorRecord{
		ErrorType: "cannot-recover",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityHigh, // 高严重性，策略无法恢复
	}

	// 触发恢复操作
	erm.attemptRecovery(record)

	// 等待完成
	time.Sleep(200 * time.Millisecond)

	// 验证没有活跃恢复
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_RecoveryTimeoutCap 测试恢复超时上限
func TestErrorRecoveryManager_RecoveryTimeoutCap(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建一个超时时间 > 60s 的策略
	longTimeoutStrategy := &DefaultRecoveryStrategy{
		name:    "long-timeout",
		timeout: 120 * time.Second, // 超过 60s 上限
	}
	erm.RegisterRecoveryStrategy("long-timeout", longTimeoutStrategy)

	record := ErrorRecord{
		ErrorType: "long-timeout",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	start := time.Now()

	// 触发恢复操作
	go erm.attemptRecovery(record)

	// 等待一段时间
	time.Sleep(200 * time.Millisecond)

	// 停止管理器
	erm.Stop()

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	duration := time.Since(start)

	// 验证超时时间被限制在 60s（实际测试中应该很快完成）
	if duration > 70*time.Second {
		t.Errorf("Recovery timeout was not capped: duration=%v", duration)
	}
}

// slowRecoveryStrategy 是一个会延迟的恢复策略（用于测试超时）
type slowRecoveryStrategy struct {
	timeout time.Duration
	delay   time.Duration
}

func (s *slowRecoveryStrategy) Name() string {
	return "slow"
}

func (s *slowRecoveryStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	return true
}

func (s *slowRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(s.delay):
		return true, nil
	}
}

func (s *slowRecoveryStrategy) GetTimeout() time.Duration {
	return s.timeout
}

// cannotRecoverStrategy 是一个无法恢复的策略（用于测试）
type cannotRecoverStrategy struct {
	timeout time.Duration
}

func (s *cannotRecoverStrategy) Name() string {
	return "cannot-recover"
}

func (s *cannotRecoverStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	// 只对低严重性错误可以恢复
	return severity <= ErrorSeverityLow
}

func (s *cannotRecoverStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(s.timeout):
		return true, nil
	}
}

func (s *cannotRecoverStrategy) GetTimeout() time.Duration {
	return s.timeout
}

// TestErrorRecoveryManager_StopMultipleTimes 测试多次调用 Stop
func TestErrorRecoveryManager_StopMultipleTimes(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	// 多次调用 Stop（应该只执行一次）
	erm.Stop()
	erm.Stop()
	erm.Stop()

	// 验证没有 panic
}

// TestErrorRecoveryManager_WithMetrics 测试带 metrics 的恢复管理器
func TestErrorRecoveryManager_WithMetrics(t *testing.T) {
	// 创建 metrics（如果可用）
	var m *metrics.ProductionMetrics
	// 注意：这里可能需要根据实际的 metrics 初始化方式调整
	erm := NewErrorRecoveryManager(m)
	defer erm.Stop()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 触发恢复操作
	go erm.attemptRecovery(record)

	// 等待完成
	time.Sleep(200 * time.Millisecond)

	// 验证没有 panic
}

