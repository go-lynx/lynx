package fsx

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Exists 判断文件或目录是否存在。
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

// WriteFileMkdirAll 写文件，若父目录不存在则自动创建。
func WriteFileMkdirAll(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ReadFileLimit 读取文件并限制最大字节数，超过返回错误，避免 OOM。
func ReadFileLimit(path string, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("ReadFileLimit: non-positive max")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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

// AtomicWrite 以原子方式写入文件：写到临时文件后 rename 替换。
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
		tmp.Close()
		os.Remove(tmpPath)
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
