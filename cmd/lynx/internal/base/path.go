package base

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// lynxHome gets the home directory of Lynx tool.
// If the home directory doesn't exist, create it.
// Returns the path to Lynx tool home directory.
func lynxHome() string {
	// Get current user's home directory
	dir, err := os.UserHomeDir()
	if err != nil {
		// If getting fails, log error and terminate program
		log.Fatal(err)
	}
	// Concatenate the path to Lynx tool home directory
	home := filepath.Join(dir, ".lynx")
	// Check if home directory exists
	if _, err := os.Stat(home); os.IsNotExist(err) {
		// If doesn't exist, recursively create directory
		if err := os.MkdirAll(home, 0o700); err != nil {
			// If creation fails, log error and terminate program
			log.Fatal(err)
		}
	}
	return home
}

// lynxHomeWithDir gets the path of specified subdirectory under Lynx tool home directory.
// If the subdirectory doesn't exist, create it.
// Parameter dir is the specified subdirectory name.
// Returns the path of specified subdirectory under Lynx tool home directory.
func lynxHomeWithDir(dir string) string {
	// Concatenate the path of specified subdirectory under Lynx tool home directory
	home := filepath.Join(lynxHome(), dir)
	// Check if subdirectory exists
	if _, err := os.Stat(home); os.IsNotExist(err) {
		// If doesn't exist, recursively create directory
		if err := os.MkdirAll(home, 0o700); err != nil {
			// If creation fails, log error and terminate program
			log.Fatal(err)
		}
	}
	return home
}

// copyFile copies source file to target file and replaces file content according to replacement rules.
// Parameters: src is source file path, dst is target file path, replaces is replacement rules list, format is [old1, new1, old2, new2, ...].
// Returns errors that may occur during copying.
func copyFile(src, dst string, replaces []string) error {
	// Get source file information
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// Read source file content
	buf, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Simple heuristic: if contains NUL, treat as binary, skip replacement
	if bytes.IndexByte(buf, 0) == -1 && len(replaces) > 0 {
		var old string
		// Iterate through replacement rules list
		for i, next := range replaces {
			if i%2 == 0 {
				// Even-indexed elements are old strings
				old = next
				continue
			}
			// Odd-indexed elements are new strings, perform global replacement
			buf = bytes.ReplaceAll(buf, []byte(old), []byte(next))
		}
	}
	// Write replaced content to target file and maintain file permissions
	return os.WriteFile(dst, buf, srcInfo.Mode())
}

// copyDir recursively copies source directory to target directory, replaces file content according to replacement rules, and ignores specified files or directories.
// Parameters: src is source directory path, dst is target directory path, replaces is replacement rules list, ignores is list of files or directories to ignore.
// Returns errors that may occur during copying.
func copyDir(src, dst string, replaces, ignores []string) error {
	// Get source directory information
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	// Recursively create target directory and maintain directory permissions
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}
	// Read all files and subdirectories under source directory
	fds, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	// Iterate through all files and subdirectories under source directory
	for _, fd := range fds {
		// Check if current file or directory should be ignored
		if hasSets(fd.Name(), ignores) {
			continue
		}
		// Concatenate complete path of source file or subdirectory
		srcFilePath := filepath.Join(src, fd.Name())
		// Concatenate complete path of target file or subdirectory
		dstFilePath := filepath.Join(dst, fd.Name())
		var e error
		if fd.IsDir() {
			// If it's a directory, recursively call copyDir function
			e = copyDir(srcFilePath, dstFilePath, replaces, ignores)
		} else {
			// If it's a file, call copyFile function
			e = copyFile(srcFilePath, dstFilePath, replaces)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

// hasSets checks if the specified name is in the given set.
// Parameters: name is the name to check, sets is the set list.
// Returns boolean value indicating whether the name is in the set.
func hasSets(name string, sets []string) bool {
	// Iterate through the set list
	for _, ig := range sets {
		if ig == name {
			return true
		}
	}
	return false
}

// Tree prints creation information of all files under the specified directory, including file name and file size.
// Parameters: path is the directory path to traverse, dir is the base directory for formatting output path.
func Tree(path string, dir string) {
	// Recursively traverse all files and subdirectories under the specified directory
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		// If no error, file info is not empty, and it's not a directory
		if err == nil && info != nil && !info.IsDir() {
			// Print file creation information, including file name and file size
			fmt.Printf("%s %s (%v bytes)\n", color.GreenString("CREATED"), strings.Replace(path, dir+"/", "", -1), info.Size())
		}
		return nil
	})
}
