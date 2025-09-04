package run

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of file system event
type EventType int

const (
	FileCreated EventType = iota
	FileModified
	FileDeleted
)

// FileEvent represents a file system event
type FileEvent struct {
	Path string
	Type EventType
	Time time.Time
}

// FileWatcher watches for file changes in a directory
type FileWatcher struct {
	root       string
	events     chan FileEvent
	fileStates map[string]time.Time
	mu         sync.RWMutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(root string) *FileWatcher {
	fw := &FileWatcher{
		root:       root,
		events:     make(chan FileEvent, 100),
		fileStates: make(map[string]time.Time),
		stopChan:   make(chan struct{}),
	}

	// Start watching
	fw.wg.Add(1)
	go fw.watch()

	return fw
}

// Events returns the events channel
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() {
	close(fw.stopChan)
	fw.wg.Wait()
	close(fw.events)
}

// watch monitors file changes
func (fw *FileWatcher) watch() {
	defer fw.wg.Done()

	// Initial scan
	fw.scan()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopChan:
			return
		case <-ticker.C:
			fw.scan()
		}
	}
}

// scan scans the directory for changes
func (fw *FileWatcher) scan() {
	currentStates := make(map[string]time.Time)

	err := filepath.WalkDir(fw.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories and ignored paths
		if d.IsDir() || fw.shouldIgnore(path) {
			return nil
		}

		// Check if it's a Go source file or configuration file
		if !fw.shouldWatch(path) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		modTime := info.ModTime()
		currentStates[path] = modTime

		// Check for changes
		fw.mu.RLock()
		oldTime, exists := fw.fileStates[path]
		fw.mu.RUnlock()

		if !exists {
			// New file
			fw.sendEvent(FileEvent{
				Path: path,
				Type: FileCreated,
				Time: time.Now(),
			})
		} else if !oldTime.Equal(modTime) {
			// Modified file
			fw.sendEvent(FileEvent{
				Path: path,
				Type: FileModified,
				Time: time.Now(),
			})
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	// Check for deleted files
	fw.mu.RLock()
	for path := range fw.fileStates {
		if _, exists := currentStates[path]; !exists {
			fw.sendEvent(FileEvent{
				Path: path,
				Type: FileDeleted,
				Time: time.Now(),
			})
		}
	}
	fw.mu.RUnlock()

	// Update state
	fw.mu.Lock()
	fw.fileStates = currentStates
	fw.mu.Unlock()
}

// shouldIgnore checks if a path should be ignored
func (fw *FileWatcher) shouldIgnore(path string) bool {
	// Get relative path
	relPath, err := filepath.Rel(fw.root, path)
	if err != nil {
		return true
	}

	// Ignore patterns
	ignorePatterns := []string{
		".git",
		".idea",
		"vendor",
		"node_modules",
		"bin",
		"dist",
		"tmp",
		".DS_Store",
		"*.test",
		"*_test.go",
	}

	for _, pattern := range ignorePatterns {
		if strings.Contains(relPath, pattern) {
			return true
		}
	}

	return false
}

// shouldWatch checks if a file should be watched
func (fw *FileWatcher) shouldWatch(path string) bool {
	ext := filepath.Ext(path)
	
	// Watch these file types
	watchExts := []string{
		".go",
		".mod",
		".sum",
		".yaml",
		".yml",
		".json",
		".toml",
		".env",
		".proto",
	}

	for _, watchExt := range watchExts {
		if ext == watchExt {
			return true
		}
	}

	return false
}

// sendEvent sends an event to the channel
func (fw *FileWatcher) sendEvent(event FileEvent) {
	select {
	case fw.events <- event:
	default:
		// Channel full, drop event
	}
}

// Debouncer helps prevent rapid successive triggers
type Debouncer struct {
	duration time.Duration
	timer    *time.Timer
	mu       sync.Mutex
}

// NewDebouncer creates a new debouncer
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
	}
}

// Trigger triggers the debounced function
func (d *Debouncer) Trigger(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer
	if d.timer != nil {
		d.timer.Stop()
	}

	// Set new timer
	d.timer = time.AfterFunc(d.duration, fn)
}