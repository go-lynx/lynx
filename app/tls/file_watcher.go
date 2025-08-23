package tls

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// FileWatcher monitors certificate files for changes
type FileWatcher struct {
	mu sync.RWMutex

	// Files being monitored
	files map[string]*MonitoredFile

	// Change detection
	changeDetected bool
	changeChan     chan struct{}

	// Control
	stopChan chan struct{}
	running  bool
}

// MonitoredFile represents a file being monitored
type MonitoredFile struct {
	Path         string
	LastModified time.Time
	LastHash     string
	Size         int64
}

// NewFileWatcher creates a new file watcher instance
func NewFileWatcher() *FileWatcher {
	return &FileWatcher{
		files:      make(map[string]*MonitoredFile),
		changeChan: make(chan struct{}, 1),
		stopChan:   make(chan struct{}),
	}
}

// AddFile adds a file to the monitoring list
func (fw *FileWatcher) AddFile(filePath string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
	}

	// Check if file exists
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", absPath, err)
	}

	// Calculate initial hash
	hash, err := fw.calculateFileHash(absPath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash for %s: %w", absPath, err)
	}

	// Add to monitoring list
	fw.files[absPath] = &MonitoredFile{
		Path:         absPath,
		LastModified: fileInfo.ModTime(),
		LastHash:     hash,
		Size:         fileInfo.Size(),
	}

	log.Infof("Added file to monitoring: %s", absPath)
	return nil
}

// RemoveFile removes a file from the monitoring list
func (fw *FileWatcher) RemoveFile(filePath string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Warnf("Failed to resolve absolute path for %s: %v", filePath, err)
		return
	}

	if _, exists := fw.files[absPath]; exists {
		delete(fw.files, absPath)
		log.Infof("Removed file from monitoring: %s", absPath)
	}
}

// Start starts the file monitoring
func (fw *FileWatcher) Start(checkInterval time.Duration) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.running {
		return
	}

	fw.running = true
	go fw.monitorLoop(checkInterval)
	log.Infof("File watcher started with check interval: %v", checkInterval)
}

// Stop stops the file monitoring
func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.running {
		return
	}

	close(fw.stopChan)
	fw.running = false
	log.Infof("File watcher stopped")
}

// monitorLoop is the main monitoring loop
func (fw *FileWatcher) monitorLoop(checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fw.checkForChanges()
		case <-fw.stopChan:
			return
		}
	}
}

// checkForChanges checks all monitored files for changes
func (fw *FileWatcher) checkForChanges() {
	fw.mu.RLock()
	files := make(map[string]*MonitoredFile, len(fw.files))
	for k, v := range fw.files {
		files[k] = &MonitoredFile{
			Path:         v.Path,
			LastModified: v.LastModified,
			LastHash:     v.LastHash,
			Size:         v.Size,
		}
	}
	fw.mu.RUnlock()

	changed := false
	for filePath, monitoredFile := range files {
		if fw.hasFileChanged(filePath, monitoredFile) {
			changed = true
			log.Infof("File changed detected: %s", filePath)
		}
	}

	if changed {
		fw.mu.Lock()
		fw.changeDetected = true
		fw.mu.Unlock()

		// Notify change (non-blocking)
		select {
		case fw.changeChan <- struct{}{}:
		default:
			// Channel is full, change already notified
		}
	}
}

// hasFileChanged checks if a specific file has changed
func (fw *FileWatcher) hasFileChanged(filePath string, monitoredFile *MonitoredFile) bool {
	// Check if file still exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Warnf("Failed to stat monitored file %s: %v", filePath, err)
		return false
	}

	// Check modification time
	if !fileInfo.ModTime().Equal(monitoredFile.LastModified) {
		return true
	}

	// Check file size
	if fileInfo.Size() != monitoredFile.Size {
		return true
	}

	// Check file hash (more expensive, but more reliable)
	hash, err := fw.calculateFileHash(filePath)
	if err != nil {
		log.Warnf("Failed to calculate hash for %s: %v", filePath, err)
		return false
	}

	if hash != monitoredFile.LastHash {
		return true
	}

	return false
}

// calculateFileHash calculates MD5 hash of a file
func (fw *FileWatcher) calculateFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:]), nil
}

// HasChanged returns whether any changes have been detected
func (fw *FileWatcher) HasChanged() bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.changeDetected
}

// ResetChanged resets the change detection flag
func (fw *FileWatcher) ResetChanged() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.changeDetected = false
}

// WaitForChange waits for a file change to be detected
func (fw *FileWatcher) WaitForChange(timeout time.Duration) bool {
	select {
	case <-fw.changeChan:
		return true
	case <-time.After(timeout):
		return false
	case <-fw.stopChan:
		return false
	}
}

// GetMonitoredFiles returns a copy of the currently monitored files
func (fw *FileWatcher) GetMonitoredFiles() map[string]*MonitoredFile {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	files := make(map[string]*MonitoredFile, len(fw.files))
	for k, v := range fw.files {
		files[k] = &MonitoredFile{
			Path:         v.Path,
			LastModified: v.LastModified,
			LastHash:     v.LastHash,
			Size:         v.Size,
		}
	}
	return files
}

// Close closes the file watcher and cleans up resources
func (fw *FileWatcher) Close() {
	fw.Stop()
	close(fw.changeChan)
}
