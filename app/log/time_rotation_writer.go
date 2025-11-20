package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// RotationStrategy defines the rotation strategy
type RotationStrategy string

const (
	RotationStrategySize RotationStrategy = "size"
	RotationStrategyTime RotationStrategy = "time"
	RotationStrategyBoth RotationStrategy = "both"
)

// RotationInterval defines the time-based rotation interval
type RotationInterval string

const (
	RotationIntervalHourly RotationInterval = "hourly"
	RotationIntervalDaily  RotationInterval = "daily"
	RotationIntervalWeekly RotationInterval = "weekly"
)

// TimeRotationWriter wraps lumberjack with time-based rotation support
type TimeRotationWriter struct {
	baseWriter   *lumberjack.Logger
	filename     string
	strategy     RotationStrategy
	interval     RotationInterval
	lastRotate   time.Time
	mu           sync.Mutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	maxTotalSize int64 // Maximum total size in bytes (0 = unlimited)
	totalSize    int64 // Current total size in bytes
	sizeMu       sync.RWMutex
}

// NewTimeRotationWriter creates a new time-based rotation writer
func NewTimeRotationWriter(filename string, maxSizeMB, maxBackups, maxAgeDays int, compress bool,
	strategy RotationStrategy, interval RotationInterval, maxTotalSizeMB int) *TimeRotationWriter {

	baseWriter := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   compress,
	}

	trw := &TimeRotationWriter{
		baseWriter:   baseWriter,
		filename:     filename,
		strategy:     strategy,
		interval:     interval,
		lastRotate:   time.Now(),
		stopCh:       make(chan struct{}),
		maxTotalSize: int64(maxTotalSizeMB) * 1024 * 1024, // Convert MB to bytes
	}

	// Calculate initial total size
	trw.updateTotalSize()

	// Start time-based rotation goroutine if needed
	if strategy == RotationStrategyTime || strategy == RotationStrategyBoth {
		trw.wg.Add(1)
		go trw.timeRotationLoop()
	}

	return trw
}

// Write implements io.Writer
func (trw *TimeRotationWriter) Write(p []byte) (int, error) {
	trw.mu.Lock()
	defer trw.mu.Unlock()

	// Check if time-based rotation is needed
	if trw.strategy == RotationStrategyTime || trw.strategy == RotationStrategyBoth {
		if trw.shouldRotateByTime() {
			if err := trw.rotate(); err != nil {
				// Log error but continue writing
				fmt.Fprintf(os.Stderr, "[lynx-log-error] Failed to rotate log file: %v\n", err)
			}
		}
	}

	n, err := trw.baseWriter.Write(p)
	if err == nil {
		// Update total size
		trw.sizeMu.Lock()
		trw.totalSize += int64(n)
		trw.sizeMu.Unlock()

		// Check total size limit
		if trw.maxTotalSize > 0 && trw.totalSize > trw.maxTotalSize {
			trw.enforceTotalSizeLimit()
		}
	}
	return n, err
}

// Close implements io.Closer
func (trw *TimeRotationWriter) Close() error {
	trw.mu.Lock()
	defer trw.mu.Unlock()

	select {
	case <-trw.stopCh:
		// Already closed
		return nil
	default:
		close(trw.stopCh)
	}

	trw.wg.Wait()
	return nil
}

// shouldRotateByTime checks if rotation is needed based on time
func (trw *TimeRotationWriter) shouldRotateByTime() bool {
	now := time.Now()
	var nextRotate time.Time

	switch trw.interval {
	case RotationIntervalHourly:
		// Rotate at the start of each hour
		nextRotate = trw.lastRotate.Truncate(time.Hour).Add(time.Hour)
	case RotationIntervalDaily:
		// Rotate at midnight
		nextRotate = trw.lastRotate.Truncate(24 * time.Hour).Add(24 * time.Hour)
	case RotationIntervalWeekly:
		// Rotate at the start of each week (Monday 00:00)
		daysSinceMonday := int(trw.lastRotate.Weekday()) - 1
		if daysSinceMonday < 0 {
			daysSinceMonday = 6 // Sunday
		}
		nextRotate = trw.lastRotate.Truncate(24*time.Hour).AddDate(0, 0, 7-daysSinceMonday)
	default:
		return false
	}

	return now.After(nextRotate) || now.Equal(nextRotate)
}

// rotate performs the rotation
func (trw *TimeRotationWriter) rotate() error {
	// For time-based rotation, we need to manually rename the file
	// since lumberjack only rotates on size

	// Generate new filename with timestamp
	timestamp := trw.lastRotate.Format(trw.getTimestampFormat())
	ext := filepath.Ext(trw.filename)
	base := trw.filename[:len(trw.filename)-len(ext)]
	newFilename := fmt.Sprintf("%s.%s%s", base, timestamp, ext)

	// Check if current file exists and has content
	info, err := os.Stat(trw.filename)
	if err == nil && info.Size() > 0 {
		// Rename current file
		if err := os.Rename(trw.filename, newFilename); err != nil {
			return fmt.Errorf("failed to rename log file: %w", err)
		}
	}

	// Recreate lumberjack writer to start writing to new file
	trw.baseWriter = &lumberjack.Logger{
		Filename:   trw.filename,
		MaxSize:    trw.baseWriter.MaxSize,
		MaxBackups: trw.baseWriter.MaxBackups,
		MaxAge:     trw.baseWriter.MaxAge,
		Compress:   trw.baseWriter.Compress,
	}

	// Update last rotate time
	trw.lastRotate = time.Now()

	// Update total size
	trw.updateTotalSize()

	return nil
}

// getTimestampFormat returns the timestamp format based on interval
func (trw *TimeRotationWriter) getTimestampFormat() string {
	switch trw.interval {
	case RotationIntervalHourly:
		return "2006010215" // YYYYMMDDHH
	case RotationIntervalDaily:
		return "20060102" // YYYYMMDD
	case RotationIntervalWeekly:
		return "20060102" // YYYYMMDD (Monday's date)
	default:
		return "20060102150405" // Full timestamp
	}
}

// timeRotationLoop runs in background to check for time-based rotation
func (trw *TimeRotationWriter) timeRotationLoop() {
	defer trw.wg.Done()

	// Calculate check interval based on rotation interval
	var checkInterval time.Duration
	switch trw.interval {
	case RotationIntervalHourly:
		checkInterval = 1 * time.Minute
	case RotationIntervalDaily:
		checkInterval = 1 * time.Hour
	case RotationIntervalWeekly:
		checkInterval = 1 * time.Hour
	default:
		checkInterval = 1 * time.Minute
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-trw.stopCh:
			return
		case <-ticker.C:
			trw.mu.Lock()
			if trw.shouldRotateByTime() {
				if err := trw.rotate(); err != nil {
					fmt.Fprintf(os.Stderr, "[lynx-log-error] Time-based rotation failed: %v\n", err)
				}
			}
			trw.mu.Unlock()
		}
	}
}

// updateTotalSize calculates the total size of all log files
func (trw *TimeRotationWriter) updateTotalSize() {
	trw.sizeMu.Lock()
	defer trw.sizeMu.Unlock()

	dir := filepath.Dir(trw.filename)
	base := filepath.Base(trw.filename)
	ext := filepath.Ext(base)
	baseName := base[:len(base)-len(ext)]

	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match log files: base.log, base.YYYYMMDD.log, base.YYYYMMDD.log.gz, etc.
		if name == base ||
			(len(name) > len(base) && name[:len(baseName)] == baseName &&
				(name[len(name)-len(ext):] == ext || name[len(name)-len(ext)-3:] == ext+".gz")) {
			info, err := entry.Info()
			if err == nil {
				total += info.Size()
			}
		}
	}

	trw.totalSize = total
}

// enforceTotalSizeLimit deletes oldest files when total size exceeds limit
func (trw *TimeRotationWriter) enforceTotalSizeLimit() {
	trw.sizeMu.Lock()
	defer trw.sizeMu.Unlock()

	if trw.totalSize <= trw.maxTotalSize {
		return
	}

	dir := filepath.Dir(trw.filename)
	base := filepath.Base(trw.filename)
	ext := filepath.Ext(base)
	baseName := base[:len(base)-len(ext)]

	// Collect all log files with their mod times
	type logFile struct {
		path string
		time time.Time
		size int64
	}

	var files []logFile
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == base ||
			(len(name) > len(baseName) && name[:len(baseName)] == baseName &&
				(name[len(name)-len(ext):] == ext || name[len(name)-len(ext)-3:] == ext+".gz")) {
			info, err := entry.Info()
			if err == nil {
				files = append(files, logFile{
					path: filepath.Join(dir, name),
					time: info.ModTime(),
					size: info.Size(),
				})
			}
		}
	}

	// Sort by mod time (oldest first) and delete until under limit
	// Simple bubble sort for small number of files
	for i := 0; i < len(files)-1; i++ {
		for j := 0; j < len(files)-i-1; j++ {
			if files[j].time.After(files[j+1].time) {
				files[j], files[j+1] = files[j+1], files[j]
			}
		}
	}

	// Delete oldest files until under limit
	for _, file := range files {
		if trw.totalSize <= trw.maxTotalSize {
			break
		}
		// Don't delete the current active log file
		if filepath.Base(file.path) == base {
			continue
		}
		if err := os.Remove(file.path); err == nil {
			trw.totalSize -= file.size
		}
	}

	// Recalculate to ensure accuracy
	trw.updateTotalSize()
}
