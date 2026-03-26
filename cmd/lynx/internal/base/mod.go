package base

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// ModulePath extracts Go module path from the specified go.mod file.
// Parameter filename is the path to the go.mod file.
// Returns:
//   - string: The extracted Go module path.
//   - error: Returns corresponding error information if reading file or parsing module path fails; otherwise returns nil.
func ModulePath(filename string) (string, error) {
	// Read the content of the go.mod file at the specified path
	modBytes, err := os.ReadFile(filename)
	if err != nil {
		// Return empty string and error information if file reading fails
		return "", err
	}
	// Call modfile.ModulePath function to extract Go module path from file content
	return modfile.ModulePath(modBytes), nil
}

// SanitizeGeneratedGoMod removes template-local replace directives that point
// outside the generated project directory while preserving in-project replaces
// such as ./api.
func SanitizeGeneratedGoMod(filename string) error {
	modBytes, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	f, err := modfile.Parse(filename, modBytes, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod %s: %w", filename, err)
	}

	projectDir := filepath.Dir(filename)
	changed := false
	for _, rep := range append([]*modfile.Replace(nil), f.Replace...) {
		if rep == nil || rep.New.Version != "" {
			continue
		}

		newPath := strings.TrimSpace(rep.New.Path)
		if newPath == "" || strings.HasPrefix(newPath, "./") {
			continue
		}

		targetPath := newPath
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Clean(filepath.Join(projectDir, targetPath))
		}
		if targetPath == projectDir || strings.HasPrefix(targetPath, projectDir+string(filepath.Separator)) {
			continue
		}

		if err := f.DropReplace(rep.Old.Path, rep.Old.Version); err != nil {
			return fmt.Errorf("drop replace %s from %s: %w", rep.Old.Path, filename, err)
		}
		changed = true
	}

	if !changed {
		return nil
	}

	formatted, err := f.Format()
	if err != nil {
		return fmt.Errorf("format go.mod %s: %w", filename, err)
	}
	return os.WriteFile(filename, formatted, 0644)
}
