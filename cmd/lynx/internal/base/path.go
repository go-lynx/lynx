package base

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// lynxHome returns ~/.lynx, creating it if missing.
func lynxHome() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	home := filepath.Join(dir, ".lynx")
	if _, err := os.Stat(home); os.IsNotExist(err) {
		if err := os.MkdirAll(home, 0o700); err != nil {
			return "", fmt.Errorf("create lynx home %q: %w", home, err)
		}
	}
	return home, nil
}

// lynxHomeWithDir returns dir under ~/.lynx, creating it if missing.
func lynxHomeWithDir(dir string) (string, error) {
	root, err := lynxHome()
	if err != nil {
		return "", err
	}
	home := filepath.Join(root, dir)
	if _, err := os.Stat(home); os.IsNotExist(err) {
		if err := os.MkdirAll(home, 0o700); err != nil {
			return "", fmt.Errorf("create lynx dir %q: %w", home, err)
		}
	}
	return home, nil
}

// copyFile copies src to dst, applying replaces as a flat [old1, new1, old2,
// new2, ...] list of substitutions. Files containing a NUL byte are treated as
// binary and copied verbatim. The source file mode is preserved.
func copyFile(src, dst string, replaces []string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	buf, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if bytes.IndexByte(buf, 0) == -1 && len(replaces) > 0 {
		var old string
		for i, next := range replaces {
			if i%2 == 0 {
				old = next
				continue
			}
			buf = bytes.ReplaceAll(buf, []byte(old), []byte(next))
		}
	}
	return os.WriteFile(dst, buf, srcInfo.Mode())
}

// copyDir recursively copies src to dst, applying replaces to each file (see
// copyFile) and skipping any entry whose name appears in ignores.
func copyDir(src, dst string, replaces, ignores []string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}
	fds, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, fd := range fds {
		if hasSets(fd.Name(), ignores) {
			continue
		}
		srcFilePath := filepath.Join(src, fd.Name())
		dstFilePath := filepath.Join(dst, fd.Name())
		var e error
		if fd.IsDir() {
			e = copyDir(srcFilePath, dstFilePath, replaces, ignores)
		} else {
			e = copyFile(srcFilePath, dstFilePath, replaces)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

// hasSets reports whether name is present in sets.
func hasSets(name string, sets []string) bool {
	for _, ig := range sets {
		if ig == name {
			return true
		}
	}
	return false
}

// Tree prints a "CREATED" line for every file under path, with names made
// relative to dir.
func Tree(path string, dir string) {
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			fmt.Printf("%s %s (%v bytes)\n", color.GreenString("CREATED"), strings.Replace(path, dir+"/", "", -1), info.Size())
		}
		return nil
	})
}
