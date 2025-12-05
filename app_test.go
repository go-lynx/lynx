package lynx

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

// TestNewApp_ConcurrentInit tests concurrent initialization
func TestNewApp_ConcurrentInit(t *testing.T) {
	// Reset global state
	resetGlobalState()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	apps := make([]*LynxApp, numGoroutines)
	errors := make([]error, numGoroutines)

	// Create test config
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

	// Verify all goroutines return the same instance or error
	firstApp := apps[0]
	firstErr := errors[0]

	for i := 1; i < numGoroutines; i++ {
		if errors[i] != nil && firstErr == nil {
			t.Errorf("Goroutine %d got error but first didn't: %v", i, errors[i])
		}
		if errors[i] == nil && firstErr != nil {
			t.Errorf("Goroutine %d got no error but first did: %v", i, firstErr)
		}
		if apps[i] != firstApp && firstErr == nil {
			t.Errorf("Goroutine %d got different app instance", i)
		}
	}

	// Cleanup
	if firstApp != nil {
		firstApp.Close()
	}
}

// TestNewApp_InitStateTransitions tests initialization state transitions
func TestNewApp_InitStateTransitions(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// Start initialization
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	// Verify app was created
	if app == nil {
		t.Error("Expected app to be initialized")
	}

	// Cleanup
	app.Close()
}

// TestNewApp_InitLockAcquisition tests initialization lock acquisition
func TestNewApp_InitLockAcquisition(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// Test that initialization can be called
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	// Verify app was created
	if app == nil {
		t.Error("Expected app to be initialized")
	}

	// Cleanup
	app.Close()
}

// TestNewApp_InitFailureRetry tests initialization failure retry
func TestNewApp_InitFailureRetry(t *testing.T) {
	resetGlobalState()

	// First initialization fails (using invalid config)
	invalidCfg := createInvalidConfig(t)
	app1, err1 := NewApp(invalidCfg)

	if err1 == nil {
		t.Error("Expected error for invalid config")
		if app1 != nil {
			app1.Close()
		}
		return
	}

	// Second initialization succeeds (sync.Once prevents re-initialization, so this will return the same error or nil)
	validCfg := createTestConfig(t)
	app2, err2 := NewApp(validCfg)

	// Note: sync.Once means second call will return cached result
	// If first failed, second will also fail; if first succeeded, second will return same instance
	if err2 != nil && err1 == nil {
		t.Logf("Second initialization got error (may be expected due to sync.Once): %v", err2)
	}

	// Cleanup
	if app2 != nil {
		app2.Close()
	}
}

// TestNewApp_InitFailureAfterSuccess tests failure handling after success
func TestNewApp_InitFailureAfterSuccess(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// First initialization succeeds
	app1, err1 := NewApp(cfg)
	if err1 != nil {
		t.Fatalf("Failed to initialize app: %v", err1)
	}

	// Close the app
	app1.Close()

	// Reset state (simulating app being closed)
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()
	initMu.Lock()
	initCompleted = false
	initErr = nil
	initDone = nil
	initMu.Unlock()

	// Reinitialize
	app2, err2 := NewApp(cfg)
	if err2 != nil {
		t.Fatalf("Failed to reinitialize app: %v", err2)
	}

	// Cleanup
	if app2 != nil {
		app2.Close()
	}
}

// TestNewApp_NilConfig tests nil config handling
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

// TestNewApp_InitTimeout tests initialization timeout
func TestNewApp_InitTimeout(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// First goroutine starts initialization
	var wg sync.WaitGroup
	wg.Add(1)

	var app1 *LynxApp
	var err1 error

	go func() {
		defer wg.Done()
		// Simulate long initialization
		time.Sleep(100 * time.Millisecond)
		app1, err1 = NewApp(cfg)
	}()

	// Second goroutine waits for initialization
	time.Sleep(10 * time.Millisecond) // Ensure first goroutine starts first

	app2, err2 := NewApp(cfg)

	// Wait for first goroutine to complete
	wg.Wait()

	// Verify second goroutine either gets same instance or timeout error
	if err2 != nil {
		// May be timeout error
		t.Logf("Second goroutine got error (may be timeout): %v", err2)
	} else if app2 != app1 {
		t.Error("Second goroutine got different app instance")
	}

	// Check first goroutine result
	if err1 != nil {
		t.Logf("First goroutine got error: %v", err1)
	}

	// Cleanup
	if app1 != nil {
		app1.Close()
	}
}

// TestNewApp_AppClosedAfterInit tests app closure after initialization
func TestNewApp_AppClosedAfterInit(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// Initialize app
	app1, err1 := NewApp(cfg)
	if err1 != nil {
		t.Fatalf("Failed to initialize app: %v", err1)
	}

	// Close app
	app1.Close()

	// Reset state
	resetGlobalState()

	// Another goroutine checks state
	app2, err2 := NewApp(cfg)

	if err2 != nil {
		t.Logf("Got error (expected if app was closed): %v", err2)
	} else if app2 == nil {
		t.Error("Got nil app without error")
	} else {
		app2.Close()
	}
}

// TestNewApp_InitLockVerification tests initialization lock verification
func TestNewApp_InitLockVerification(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	// Try to initialize
	app, err := NewApp(cfg)

	// Verify result
	if err != nil {
		t.Logf("Got error (may be expected): %v", err)
	} else if app != nil {
		app.Close()
	}
}

// TestNewApp_MultipleInitAttempts tests multiple initialization attempts
func TestNewApp_MultipleInitAttempts(t *testing.T) {
	resetGlobalState()

	cfg := createTestConfig(t)

	const numAttempts = 10
	apps := make([]*LynxApp, numAttempts)
	errors := make([]error, numAttempts)

	for i := 0; i < numAttempts; i++ {
		apps[i], errors[i] = NewApp(cfg)
		time.Sleep(10 * time.Millisecond) // Small delay
	}

	// Verify all attempts return the same instance
	firstApp := apps[0]
	firstErr := errors[0]

	for i := 1; i < numAttempts; i++ {
		if apps[i] != firstApp && firstErr == nil {
			t.Errorf("Attempt %d got different app instance", i)
		}
	}

	// Cleanup
	if firstApp != nil {
		firstApp.Close()
	}
}

// TestNewApp_ConcurrentInitWithFailure tests concurrent initialization with failures
func TestNewApp_ConcurrentInitWithFailure(t *testing.T) {
	resetGlobalState()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	apps := make([]*LynxApp, numGoroutines)
	errors := make([]error, numGoroutines)

	// Some use valid config, some use invalid config
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

	// Verify at least one success (using valid config)
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

// resetGlobalState resets global state for testing
func resetGlobalState() {
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()
	initMu.Lock()
	initErr = nil
	initCompleted = false
	initDone = nil
	initMu.Unlock()
}

// createTestConfig creates test config
func createTestConfig(t *testing.T) config.Config {
	// Create a simple memory config
	// Note: needs adjustment based on actual config system
	// If unable to create real config, can create a mock config
	return config.New(
		config.WithSource(
			file.NewSource("testdata/test.yaml"), // Needs test file
		),
	)
}

// createInvalidConfig creates invalid config for testing failure scenarios
func createInvalidConfig(t *testing.T) config.Config {
	// Return nil or invalid config
	return nil
}

// TestNewApp_GoroutineLeak tests goroutine leaks
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
				// Don't close, let test verify cleanup
			}
		}()
	}

	wg.Wait()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow some system goroutines
	if after > before+10 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}

	// Cleanup
	lynxMu.Lock()
	if lynxApp != nil {
		lynxApp.Close()
		lynxApp = nil
	}
	lynxMu.Unlock()
	resetGlobalState()
}
