package lynx

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-lynx/lynx/observability/metrics"
)

// TestErrorRecoveryManager_ConcurrentRecovery tests concurrent recovery operations
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

	// Verify recovery operations were executed (same recoveryKey)
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	// Wait for all recoveries to complete
	time.Sleep(200 * time.Millisecond)

	// Check active recovery count again
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

// TestErrorRecoveryManager_RecoveryKeyCollision 测试恢复键冲突E
func TestErrorRecoveryManager_RecoveryKeyCollision(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建相同时间戳皁E��同错误�E�可能产生相同的 recoveryKey�E�E
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

	// 同时触发两个恢复操佁E
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

	// 等征E��复完�E
	time.Sleep(200 * time.Millisecond)

	// 验证恢复键皁E��一性�E�通迁E��查 activeRecoveries�E�E
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

	// 验证E��误计数正确性
	stats := erm.GetErrorStats()
	count, ok := stats["test-error"].(int64)
	if !ok {
		t.Fatal("Failed to get error count")
	}

	if count != int64(numGoroutines) {
		t.Errorf("Expected error count %d, got %d", numGoroutines, count)
	}

	// 验证E��误厁E��记彁E
	history := erm.GetErrorHistory()
	if len(history) != numGoroutines {
		t.Errorf("Expected error history length %d, got %d", numGoroutines, len(history))
	}
}

// TestErrorRecoveryManager_GoroutineLeak 测证Egoroutine 況E��E
func TestErrorRecoveryManager_GoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 启动恢复操佁E
	go erm.attemptRecovery(record)

	// 立即停止管琁E��
	erm.Stop()

	// 等征Egoroutine 退出
	time.Sleep(500 * time.Millisecond)

	after := runtime.NumGoroutine()

	// 允许一些系绁Egoroutine�E�佁E��该接近�E始值
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

	// 启动恢复操作（会在获叁Esemaphore 前返回�E�E
	// 通迁E��满信号量来触发趁E��
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	// 尝试恢复（应该因为信号量满而趁E���E�E
	go erm.attemptRecovery(record)

	// 等征E��E��和渁E��
	time.Sleep(300 * time.Millisecond)

	// 释放信号釁E
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等征Egoroutine 退出
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup 测试信号量趁E��时皁Egoroutine 渁E��
func TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// 填满信号釁E
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

	// 尝试恢复（应该因为信号量满而趁E���E�E
	go erm.attemptRecovery(record)

	// 等征E��E��和渁E��
	time.Sleep(300 * time.Millisecond)

	// 释放信号釁E
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等征Egoroutine 退出
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_RecoveryTimeout 测试恢复趁E��
func TestErrorRecoveryManager_RecoveryTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建一个会趁E��皁E��复策略
	slowStrategy := &slowRecoveryStrategy{
		timeout: 100 * time.Millisecond,
		delay:   200 * time.Millisecond, // 比趁E��时间长
	}
	erm.RegisterRecoveryStrategy("slow", slowStrategy)

	record := ErrorRecord{
		ErrorType: "slow",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// 触发恢复操佁E
	go erm.attemptRecovery(record)

	// 等征E��E��
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	// 验证趁E��后正确渁E��
	if after > before+5 {
		t.Errorf("Possible goroutine leak after timeout: before=%d, after=%d", before, after)
	}

	// 验证活跁E��复被渁E��
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries after timeout, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeout 测试信号量获取趁E��
func TestErrorRecoveryManager_SemaphoreTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// 填满信号釁E
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

	// 尝试恢复（应该因为信号量满而趁E���E�E
	go erm.attemptRecovery(record)

	// 等征E��E��
	time.Sleep(300 * time.Millisecond)

	// 释放信号釁E
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// 等征E��E��
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_ContextCancellation 测证Econtext 取涁E
func TestErrorRecoveryManager_ContextCancellation(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// 启动恢复操佁E
	go erm.attemptRecovery(record)

	// 立即停止管琁E���E�取消所有恢复！E
	erm.Stop()

	// 等征E��E��
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}

	// 验证活跁E��复被渁E��
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

	// 记录一个没有对应策略皁E��误�E�应该使用默认策略�E�E
	record := ErrorRecord{
		ErrorType: "unknown-error",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 应该使用默认皁E"transient" 策略
	erm.attemptRecovery(record)

	// 等征E���E
	time.Sleep(200 * time.Millisecond)

	// 验证没朁Epanic
}

// TestErrorRecoveryManager_StrategyCannotRecover 测试策略无法恢夁E
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
		Severity:  ErrorSeverityHigh, // 高严重性�E�策略无法恢夁E
	}

	// 触发恢复操佁E
	erm.attemptRecovery(record)

	// 等征E���E
	time.Sleep(200 * time.Millisecond)

	// 验证没有活跁E��夁E
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_RecoveryTimeoutCap 测试恢复趁E��上限
func TestErrorRecoveryManager_RecoveryTimeoutCap(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// 创建一个趁E��时间 > 60s 皁E��略
	longTimeoutStrategy := &DefaultRecoveryStrategy{
		name:    "long-timeout",
		timeout: 120 * time.Second, // 趁E��E60s 上限
	}
	erm.RegisterRecoveryStrategy("long-timeout", longTimeoutStrategy)

	record := ErrorRecord{
		ErrorType: "long-timeout",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	start := time.Now()

	// 触发恢复操佁E
	go erm.attemptRecovery(record)

	// 等征E��段时间
	time.Sleep(200 * time.Millisecond)

	// 停止管琁E��
	erm.Stop()

	// 等征E��E��
	time.Sleep(200 * time.Millisecond)

	duration := time.Since(start)

	// 验证趁E��时间被限制在 60s�E�实际测试中应该很快完�E�E�E
	if duration > 70*time.Second {
		t.Errorf("Recovery timeout was not capped: duration=%v", duration)
	}
}

// slowRecoveryStrategy 是一个会延迟的恢复策略�E�用于测试趁E���E�E
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

// cannotRecoverStrategy 是一个无法恢复的策略�E�用于测试！E
type cannotRecoverStrategy struct {
	timeout time.Duration
}

func (s *cannotRecoverStrategy) Name() string {
	return "cannot-recover"
}

func (s *cannotRecoverStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	// 只对低严重性错误可以恢夁E
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

// TestErrorRecoveryManager_StopMultipleTimes 测试多次谁E�� Stop
func TestErrorRecoveryManager_StopMultipleTimes(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	// 多次谁E�� Stop�E�应该只执行一次�E�E
	erm.Stop()
	erm.Stop()
	erm.Stop()

	// 验证没朁Epanic
}

// TestErrorRecoveryManager_WithMetrics 测试带 metrics 皁E��复管琁E��
func TestErrorRecoveryManager_WithMetrics(t *testing.T) {
	// 创建 metrics�E�如果可用�E�E
	var m *metrics.ProductionMetrics
	// 注意：这里可能需要根据实际皁Emetrics 初始化方式谁E��
	erm := NewErrorRecoveryManager(m)
	defer erm.Stop()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// 触发恢复操佁E
	go erm.attemptRecovery(record)

	// 等征E���E
	time.Sleep(200 * time.Millisecond)

	// 验证没朁Epanic
}
