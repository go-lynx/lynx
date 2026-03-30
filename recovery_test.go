package lynx

import (
	"testing"
	"time"
)

// ---- ErrorRecoveryManager tests ----

func TestNewErrorRecoveryManager_Defaults(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	if erm == nil {
		t.Fatal("NewErrorRecoveryManager returned nil")
	}
	defer erm.Stop()

	if erm.maxErrorHistory != 1000 {
		t.Errorf("expected maxErrorHistory=1000, got %d", erm.maxErrorHistory)
	}
	if erm.maxRecoveryHistory != 500 {
		t.Errorf("expected maxRecoveryHistory=500, got %d", erm.maxRecoveryHistory)
	}
	if erm.errorThreshold != 10 {
		t.Errorf("expected errorThreshold=10, got %d", erm.errorThreshold)
	}
	if erm.maxConcurrentRecoveries != 10 {
		t.Errorf("expected maxConcurrentRecoveries=10, got %d", erm.maxConcurrentRecoveries)
	}
}

func TestErrorRecoveryManager_RecordError_StoresHistory(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	erm.RecordError("db", ErrorCategoryDatabase, "connection failed", "db-plugin", ErrorSeverityHigh, nil)

	history := erm.GetErrorHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 error record, got %d", len(history))
	}
	rec := history[0]
	if rec.ErrorType != "db" {
		t.Errorf("ErrorType: expected 'db', got %q", rec.ErrorType)
	}
	if rec.Category != ErrorCategoryDatabase {
		t.Errorf("Category: expected %q, got %q", ErrorCategoryDatabase, rec.Category)
	}
	if rec.Message != "connection failed" {
		t.Errorf("Message: expected 'connection failed', got %q", rec.Message)
	}
}

func TestErrorRecoveryManager_RecordError_NilContextDefaultsToEmpty(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	erm.RecordError("net", ErrorCategoryNetwork, "timeout", "svc", ErrorSeverityLow, nil)

	history := erm.GetErrorHistory()
	if len(history) == 0 {
		t.Fatal("expected at least one error record")
	}
	if history[0].Context == nil {
		t.Error("expected non-nil context map even when nil was passed")
	}
}

func TestErrorRecoveryManager_RegisterCustomStrategy(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	custom := NewDefaultRecoveryStrategy("custom-strat", 5*time.Millisecond)
	erm.RegisterRecoveryStrategy("myerror", custom)

	erm.mu.RLock()
	strat, exists := erm.recoveryStrategies["myerror"]
	erm.mu.RUnlock()

	if !exists {
		t.Fatal("custom strategy not found after registration")
	}
	if strat.Name() != "custom-strat" {
		t.Errorf("expected strategy name 'custom-strat', got %q", strat.Name())
	}
}

func TestErrorRecoveryManager_IsHealthy_TrueWhenNoErrors(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	if !erm.IsHealthy() {
		t.Error("new manager should be healthy")
	}
}

func TestErrorRecoveryManager_IsHealthy_FalseWhenThresholdExceeded(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Exceed the default error threshold (10) for the same error type.
	for i := 0; i <= 10; i++ {
		erm.mu.Lock()
		erm.errorCounts["saturate"]++
		erm.mu.Unlock()
	}

	if erm.IsHealthy() {
		t.Error("manager should be unhealthy when error count exceeds threshold")
	}
}

func TestErrorRecoveryManager_GetErrorStats_ContainsKeys(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	erm.RecordError("plugin", ErrorCategoryPlugin, "crash", "p1", ErrorSeverityMedium, map[string]any{"detail": "oops"})

	stats := erm.GetErrorStats()
	if stats == nil {
		t.Fatal("GetErrorStats should not return nil")
	}
	for _, key := range []string{"error_counts", "recent_errors", "recovery_stats", "circuit_breaker_states"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("stats missing expected key %q", key)
		}
	}
}

func TestErrorRecoveryManager_GetHealthReport_HasHealthyKey(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	report := erm.GetHealthReport()
	if _, ok := report["healthy"]; !ok {
		t.Error("health report missing 'healthy' key")
	}
}

func TestErrorRecoveryManager_ClearHistory(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	erm.RecordError("e", ErrorCategorySystem, "msg", "comp", ErrorSeverityLow, nil)
	erm.ClearHistory()

	if len(erm.GetErrorHistory()) != 0 {
		t.Error("error history should be empty after ClearHistory")
	}
	if len(erm.GetRecoveryHistory()) != 0 {
		t.Error("recovery history should be empty after ClearHistory")
	}
}

func TestErrorRecoveryManager_Stop_Idempotent(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	// Calling Stop multiple times should not panic.
	erm.Stop()
	erm.Stop()
}

func TestErrorRecoveryManager_MaxErrorHistoryCapped(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Directly push more than maxErrorHistory entries to verify capping.
	for i := 0; i < erm.maxErrorHistory+10; i++ {
		erm.RecordError("flood", ErrorCategoryNetwork, "msg", "comp", ErrorSeverityLow, nil)
	}

	history := erm.GetErrorHistory()
	if len(history) > erm.maxErrorHistory {
		t.Errorf("error history exceeds max: got %d, want <= %d", len(history), erm.maxErrorHistory)
	}
}

// ---- ErrorSeverity and ErrorCategory constants ----

func TestErrorSeverityOrder(t *testing.T) {
	if ErrorSeverityLow >= ErrorSeverityMedium {
		t.Error("Low should be less than Medium")
	}
	if ErrorSeverityMedium >= ErrorSeverityHigh {
		t.Error("Medium should be less than High")
	}
	if ErrorSeverityHigh >= ErrorSeverityCritical {
		t.Error("High should be less than Critical")
	}
}

func TestErrorCategoryConstants(t *testing.T) {
	categories := []ErrorCategory{
		ErrorCategoryNetwork, ErrorCategoryDatabase, ErrorCategoryConfig,
		ErrorCategoryPlugin, ErrorCategoryResource, ErrorCategorySecurity,
		ErrorCategoryTimeout, ErrorCategoryValidation, ErrorCategorySystem,
	}
	seen := make(map[ErrorCategory]bool)
	for _, c := range categories {
		if seen[c] {
			t.Errorf("duplicate ErrorCategory value: %q", c)
		}
		seen[c] = true
	}
}

// ---- DefaultRecoveryStrategy tests ----

func TestDefaultRecoveryStrategy_Name(t *testing.T) {
	s := NewDefaultRecoveryStrategy("my-strat", time.Second)
	if s.Name() != "my-strat" {
		t.Errorf("expected 'my-strat', got %q", s.Name())
	}
}

func TestDefaultRecoveryStrategy_CanRecover(t *testing.T) {
	s := NewDefaultRecoveryStrategy("s", time.Second)
	if !s.CanRecover("any", ErrorSeverityLow) {
		t.Error("should be able to recover from Low severity")
	}
	if !s.CanRecover("any", ErrorSeverityMedium) {
		t.Error("should be able to recover from Medium severity")
	}
	if s.CanRecover("any", ErrorSeverityHigh) {
		t.Error("should NOT recover from High severity by default")
	}
}

func TestDefaultRecoveryStrategy_GetTimeout(t *testing.T) {
	dur := 42 * time.Millisecond
	s := NewDefaultRecoveryStrategy("s", dur)
	if s.GetTimeout() != dur {
		t.Errorf("expected %v, got %v", dur, s.GetTimeout())
	}
}
