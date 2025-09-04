package fsx

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Exists reports whether the file or directory exists.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// WriteFileMkdirAll writes a file, creating parent directories if needed.
func WriteFileMkdirAll(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ReadFileLimit reads a file with a maximum byte limit; returns error if exceeded to avoid OOM.
func ReadFileLimit(path string, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("ReadFileLimit: non-positive max")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			return
		}
	}(f)

	lr := &io.LimitedReader{R: f, N: max + 1}
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("ReadFileLimit: file too large")
	}
	return b, nil
}

// AtomicWrite writes a file atomically: write to a temp file then rename to replace.
func AtomicWrite(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		err := tmp.Close()
		if err != nil {
			return
		}
		err = os.Remove(tmpPath)
		if err != nil {
			return
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
