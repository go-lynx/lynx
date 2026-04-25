package tls

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWatcher_CloseIsIdempotent(t *testing.T) {
	watcher := NewFileWatcher()
	watcher.Close()
	watcher.Close()
}

func TestFileWatcher_StartWithInvalidIntervalDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(path, []byte("cert"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	watcher := NewFileWatcher()
	if err := watcher.AddFile(path); err != nil {
		t.Fatalf("add file: %v", err)
	}
	watcher.Start(0)
	watcher.Close()

	if watcher.WaitForChange(10 * time.Millisecond) {
		t.Fatal("closed watcher should not report changes")
	}
}
