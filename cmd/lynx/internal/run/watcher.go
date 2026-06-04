package run

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventType is the kind of change reported for a file.
type EventType int

const (
	FileCreated EventType = iota
	FileModified
	FileDeleted
)

// FileEvent is a single observed file change.
type FileEvent struct {
	Path string
	Type EventType
	Time time.Time
}

// FileWatcher polls a directory tree and emits FileEvents on change. It is a
// portable mtime-based poller rather than an OS notification API.
type FileWatcher struct {
	root       string
	events     chan FileEvent
	fileStates map[string]time.Time
	mu         sync.RWMutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewFileWatcher starts watching root and returns the watcher; call Stop to end.
func NewFileWatcher(root string) *FileWatcher {
	fw := &FileWatcher{
		root:       root,
		events:     make(chan FileEvent, 100),
		fileStates: make(map[string]time.Time),
		stopChan:   make(chan struct{}),
	}

	fw.wg.Add(1)
	go fw.watch()

	return fw
}

// Events returns the channel on which file changes are delivered.
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Stop ends watching and closes the Events channel.
func (fw *FileWatcher) Stop() {
	close(fw.stopChan)
	fw.wg.Wait()
	close(fw.events)
}

// watch seeds the baseline then rescans on a fixed interval until stopped.
func (fw *FileWatcher) watch() {
	defer fw.wg.Done()

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

// scan walks the tree once, comparing mtimes against the previous snapshot to
// emit created/modified events, then emits deleted events for vanished files and
// replaces the snapshot.
func (fw *FileWatcher) scan() {
	currentStates := make(map[string]time.Time)

	err := filepath.WalkDir(fw.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() || fw.shouldIgnore(path) {
			return nil
		}

		if !fw.shouldWatch(path) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		modTime := info.ModTime()
		currentStates[path] = modTime

		fw.mu.RLock()
		oldTime, exists := fw.fileStates[path]
		fw.mu.RUnlock()

		if !exists {
			fw.sendEvent(FileEvent{
				Path: path,
				Type: FileCreated,
				Time: time.Now(),
			})
		} else if !oldTime.Equal(modTime) {
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

	fw.mu.Lock()
	fw.fileStates = currentStates
	fw.mu.Unlock()
}

// shouldIgnore reports whether path falls under a build/VCS/editor directory or
// matches an ignored pattern (including test files).
func (fw *FileWatcher) shouldIgnore(path string) bool {
	relPath, err := filepath.Rel(fw.root, path)
	if err != nil {
		return true
	}

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

// shouldWatch reports whether path has a source/config extension worth a rebuild.
func (fw *FileWatcher) shouldWatch(path string) bool {
	ext := filepath.Ext(path)

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

// sendEvent delivers an event, dropping it if the buffer is full to keep the
// scanner non-blocking.
func (fw *FileWatcher) sendEvent(event FileEvent) {
	select {
	case fw.events <- event:
	default:
	}
}

// Debouncer coalesces rapid triggers into a single delayed call.
type Debouncer struct {
	duration time.Duration
	timer    *time.Timer
	mu       sync.Mutex
}

// NewDebouncer returns a Debouncer that fires duration after the last Trigger.
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
	}
}

// Trigger triggers the debounced function. If the previous timer already fired,
// the callback may be running; we do not schedule again to avoid double invocation.
func (d *Debouncer) Trigger(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		if !d.timer.Stop() {
			// Timer already fired or is firing; skip this trigger to avoid racing with callback
			return
		}
	}
	d.timer = time.AfterFunc(d.duration, fn)
}
