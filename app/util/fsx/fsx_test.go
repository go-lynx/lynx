package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileX(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a/b/c.txt")
	ok, err := Exists(p)
	if err != nil || ok {
		t.Fatalf("Exists before create: %v %v", ok, err)
	}

	if err := WriteFileMkdirAll(p, []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFileMkdirAll: %v", err)
	}
	ok, err = Exists(p)
	if err != nil || !ok {
		t.Fatalf("Exists after create")
	}

	b, err := ReadFileLimit(p, 10)
	if err != nil || string(b) != "hi" {
		t.Fatalf("ReadFileLimit ok: %v %q", err, string(b))
	}
	if _, err := ReadFileLimit(p, 1); err == nil {
		t.Fatalf("ReadFileLimit should error on too large")
	}

	q := filepath.Join(dir, "q.txt")
	if err := AtomicWrite(q, []byte("hello"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	data, err := os.ReadFile(q)
	if err != nil || string(data) != "hello" {
		t.Fatalf("AtomicWrite content: %v %q", err, string(data))
	}
}
