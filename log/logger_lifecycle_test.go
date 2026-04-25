package log

import (
	"sync"
	"testing"

	kconf "github.com/go-kratos/kratos/v2/config"
)

func TestLoggerLifecycle_ReinitializesCleanly(t *testing.T) {
	cfg := kconf.New()

	if err := InitLogger("svc-a", "host-a", "v1", cfg); err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	Info("first logger initialized")

	if err := InitLogger("svc-b", "host-b", "v2", cfg); err != nil {
		t.Fatalf("second init failed: %v", err)
	}
	Info("second logger initialized")

	CleanupLoggers()
	CleanupLoggers()
}

func TestLoggerLifecycle_ConcurrentCleanupIsIdempotent(t *testing.T) {
	cfg := kconf.New()
	if err := InitLogger("svc", "host", "v1", cfg); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			CleanupLoggers()
		}()
	}
	wg.Wait()
}
