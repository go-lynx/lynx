package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	
	// Cache for file operations to improve performance
	sizeCache      atomic.Int64 // Cached total size
	sizeCacheTime  atomic.Int64 // Cache timestamp (Unix nano)
	sizeCacheValid atomic.Bool  // Cache validity flag
	cacheTTL       time.Duration // Cache TTL (default: 5 seconds)
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
		cacheTTL:     5 * time.Second, // Cache for 5 seconds
	}

	// Calculate initial total size (synchronously for accuracy)
	trw.updateTotalSize()
	// Initialize cache
	trw.sizeCache.Store(trw.totalSize)
	trw.sizeCacheTime.Store(time.Now().UnixNano())
	trw.sizeCacheValid.Store(true)

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
		// Update total size (use atomic for better performance)
		trw.sizeMu.Lock()
		trw.totalSize += int64(n)
		totalSize := trw.totalSize
		maxTotalSize := trw.maxTotalSize
		trw.sizeMu.Unlock()

		// Check total size limit (call outside main lock to avoid blocking writes)
		if maxTotalSize > 0 && totalSize > maxTotalSize {
			// Use goroutine to avoid blocking Write() call
			// Use context with timeout to prevent goroutine leak
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				
				done := make(chan struct{})
				go func() {
					defer close(done)
					trw.sizeMu.Lock()
					defer trw.sizeMu.Unlock()
					// Double-check after acquiring lock (might have been updated)
					if trw.totalSize > trw.maxTotalSize {
						// Invalidate cache before enforcement
						trw.invalidateSizeCache()
						trw.enforceTotalSizeLimit()
						// Update cache after enforcement
						trw.updateTotalSize()
					}
				}()
				
				select {
				case <-done:
					// Completed successfully
				case <-ctx.Done():
					// Timeout: log warning but don't block
					fmt.Fprintf(os.Stderr, "[lynx-log-warn] enforceTotalSizeLimit timeout after 30s\n")
				}
			}()
		}
	}
	return n, err
}

// Close implements io.Closer
func (trw *TimeRotationWriter) Close() error {
	trw.mu.Lock()
	
	// Check if already closed
	select {
	case <-trw.stopCh:
		trw.mu.Unlock()
		return nil
	default:
		close(trw.stopCh)
	}
	trw.mu.Unlock()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		trw.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		// Timeout: log warning but return (prevent deadlock)
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] TimeRotationWriter Close timeout after 5s\n")
		return nil
	}
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

	// Invalidate cache and update total size
	trw.invalidateSizeCache()
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

	// Calculate base check interval based on rotation interval
	var baseCheckInterval time.Duration
	switch trw.interval {
	case RotationIntervalHourly:
		baseCheckInterval = 30 * time.Second // Check every 30 seconds for hourly rotation
	case RotationIntervalDaily:
		baseCheckInterval = 5 * time.Minute // Check every 5 minutes for daily rotation
	case RotationIntervalWeekly:
		baseCheckInterval = 30 * time.Minute // Check every 30 minutes for weekly rotation
	default:
		baseCheckInterval = 1 * time.Minute
	}

	ticker := time.NewTicker(baseCheckInterval)
	defer ticker.Stop()
	
	var currentInterval = baseCheckInterval

	for {
		select {
		case <-trw.stopCh:
			return
		case <-ticker.C:
			trw.mu.Lock()
			now := time.Now()
			
			// Calculate time until next rotation
			var nextRotate time.Time
			switch trw.interval {
			case RotationIntervalHourly:
				nextRotate = trw.lastRotate.Truncate(time.Hour).Add(time.Hour)
			case RotationIntervalDaily:
				nextRotate = trw.lastRotate.Truncate(24 * time.Hour).Add(24 * time.Hour)
			case RotationIntervalWeekly:
				daysSinceMonday := int(trw.lastRotate.Weekday()) - 1
				if daysSinceMonday < 0 {
					daysSinceMonday = 6 // Sunday
				}
				nextRotate = trw.lastRotate.Truncate(24*time.Hour).AddDate(0, 0, 7-daysSinceMonday)
			default:
				nextRotate = now.Add(baseCheckInterval)
			}
			
			// If rotation is due, perform rotation
			if trw.shouldRotateByTime() {
				if err := trw.rotate(); err != nil {
					fmt.Fprintf(os.Stderr, "[lynx-log-error] Time-based rotation failed: %v\n", err)
				}
				// Reset to base interval after rotation
				if currentInterval != baseCheckInterval {
					ticker.Reset(baseCheckInterval)
					currentInterval = baseCheckInterval
				}
				trw.mu.Unlock()
				continue
			}
			
			// Dynamically adjust check interval based on time until rotation
			// Check more frequently as rotation time approaches
			timeUntilRotate := nextRotate.Sub(now)
			var newInterval time.Duration
			
			if timeUntilRotate <= 0 {
				// Should have rotated, use base interval
				newInterval = baseCheckInterval
			} else if timeUntilRotate < 2*time.Minute {
				// Within 2 minutes: check every 10 seconds for precision
				newInterval = 10 * time.Second
			} else if timeUntilRotate < 10*time.Minute {
				// Within 10 minutes: check every 30 seconds
				newInterval = 30 * time.Second
			} else if timeUntilRotate < 1*time.Hour {
				// Within 1 hour: check every 2 minutes
				newInterval = 2 * time.Minute
			} else {
				// Far from rotation: use base interval
				newInterval = baseCheckInterval
			}
			
			// Only reset ticker if interval changed significantly (more than 20% difference)
			if newInterval != currentInterval {
				diff := newInterval - currentInterval
				if diff < 0 {
					diff = -diff
				}
				// Reset if change is significant (more than 20% of current interval)
				if diff > currentInterval/5 || newInterval < currentInterval {
					ticker.Reset(newInterval)
					currentInterval = newInterval
				}
			}
			trw.mu.Unlock()
		}
	}
}

// updateTotalSize calculates the total size of all log files
// This operation can be slow with many files, so we use caching
func (trw *TimeRotationWriter) updateTotalSize() {
	trw.sizeMu.Lock()
	defer trw.sizeMu.Unlock()

	// Check cache first
	if trw.sizeCacheValid.Load() {
		cacheTime := trw.sizeCacheTime.Load()
		if cacheTime > 0 {
			cacheAge := time.Since(time.Unix(0, cacheTime))
			if cacheAge < trw.cacheTTL {
				// Use cached value
				trw.totalSize = trw.sizeCache.Load()
				return
			}
		}
	}

	dir := filepath.Dir(trw.filename)
	base := filepath.Base(trw.filename)
	ext := filepath.Ext(base)
	baseName := base[:len(base)-len(ext)]

	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		// On error, use cached value if available
		if trw.sizeCacheValid.Load() {
			trw.totalSize = trw.sizeCache.Load()
		}
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
	// Update cache
	trw.sizeCache.Store(total)
	trw.sizeCacheTime.Store(time.Now().UnixNano())
	trw.sizeCacheValid.Store(true)
}

// invalidateSizeCache marks the size cache as invalid
func (trw *TimeRotationWriter) invalidateSizeCache() {
	trw.sizeCacheValid.Store(false)
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

	// Sort by mod time (oldest first) using insertion sort (better for small arrays)
	// For large number of files, consider using sort.Slice, but for log files this is usually fine
	if len(files) > 1 {
		for i := 1; i < len(files); i++ {
			key := files[i]
			j := i - 1
			for j >= 0 && files[j].time.After(key.time) {
				files[j+1] = files[j]
				j--
			}
			files[j+1] = key
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

	// Invalidate cache and recalculate to ensure accuracy
	trw.invalidateSizeCache()
	trw.updateTotalSize()
}
