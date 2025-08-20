package base

import (
	"os"

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
