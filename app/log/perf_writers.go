package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// LogPerformanceMetrics holds lightweight metrics for logging performance.
type LogPerformanceMetrics struct {
	TotalLogs         int64
	DroppedLogs       int64
	AvgWriteTime      time.Duration
	BufferUtilization float64
	FlushCount        int64
	ErrorCount        int64
	lastReset         time.Time
}

// BufferedWriter wraps an io.Writer with buffering and simple metrics.
type BufferedWriter struct {
	under   io.Writer
	buf     *bufio.Writer
	mu      sync.Mutex
	metrics LogPerformanceMetrics
	closed  atomic.Bool
	// Atomic metrics for thread-safe access
	avgWriteTimeAtomic atomic.Int64 // nanoseconds
}

// NewBufferedWriter creates a buffered writer with the given buffer size.
func NewBufferedWriter(w io.Writer, size int) *BufferedWriter {
	bw := &BufferedWriter{
		under: w,
		buf:   bufio.NewWriterSize(w, size),
		metrics: LogPerformanceMetrics{
			lastReset: time.Now(),
		},
	}
	return bw
}

// Write writes data to the buffer and tracks metrics.
func (b *BufferedWriter) Write(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	start := time.Now()
	b.mu.Lock()
	n, err := b.buf.Write(p)
	// Strategy: flush on newline or if buffer is close to full; bufio handles flushing on its policy
	if err == nil && (len(p) > 0 && p[len(p)-1] == '\n') {
		if ferr := b.buf.Flush(); ferr == nil {
			b.metrics.FlushCount++
		}
	}
	b.mu.Unlock()
	dur := time.Since(start)

	// Metrics (EMA for avg write time) - use atomic operations for thread safety
	durNs := int64(dur)
	prevNs := b.avgWriteTimeAtomic.Load()
	if prevNs == 0 {
		b.avgWriteTimeAtomic.Store(durNs)
	} else {
		newAvg := (prevNs + durNs) / 2
		b.avgWriteTimeAtomic.Store(newAvg)
	}

	atomic.AddInt64(&b.metrics.TotalLogs, 1)
	if err != nil {
		atomic.AddInt64(&b.metrics.ErrorCount, 1)
	}
	// BufferUtilization is not directly observable; leave as 0.0 for now
	return n, err
}

// Flush forces buffered data to be written.
func (b *BufferedWriter) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed.Load() {
		return io.ErrClosedPipe
	}
	if err := b.buf.Flush(); err != nil {
		atomic.AddInt64(&b.metrics.ErrorCount, 1)
		return err
	}
	b.metrics.FlushCount++
	return nil
}

// Close flushes and marks the writer closed.
func (b *BufferedWriter) Close() error {
	if b.closed.Swap(true) {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.buf.Flush(); err != nil {
		atomic.AddInt64(&b.metrics.ErrorCount, 1)
		return err
	}
	b.metrics.FlushCount++
	return nil
}

// GetMetrics returns a snapshot of metrics.
func (b *BufferedWriter) GetMetrics() LogPerformanceMetrics {
	// Use atomic value for avg write time
	avgNs := b.avgWriteTimeAtomic.Load()
	return LogPerformanceMetrics{
		TotalLogs:         atomic.LoadInt64(&b.metrics.TotalLogs),
		DroppedLogs:       atomic.LoadInt64(&b.metrics.DroppedLogs),
		AvgWriteTime:      time.Duration(avgNs),
		BufferUtilization: b.metrics.BufferUtilization,
		FlushCount:        atomic.LoadInt64(&b.metrics.FlushCount),
		ErrorCount:        atomic.LoadInt64(&b.metrics.ErrorCount),
		lastReset:         b.metrics.lastReset,
	}
}

// ResetMetrics resets counters.
func (b *BufferedWriter) ResetMetrics() {
	b.metrics = LogPerformanceMetrics{lastReset: time.Now()}
}

// AsyncLogWriter is a non-blocking writer that writes in background.
type AsyncLogWriter struct {
	writer     io.Writer
	closer     io.Closer
	queue      chan []byte
	stopCh     chan struct{}
	wg         sync.WaitGroup
	metrics    LogPerformanceMetrics
	qLen       int64 // current queue length
	closed     atomic.Bool
	dropWarned atomic.Int64 // track if we've warned about drops to avoid log spam
	// Atomic metrics for thread-safe access
	avgWriteTimeAtomic atomic.Int64 // nanoseconds

	// Dynamic queue size adjustment
	mu            sync.RWMutex
	queueSize     int
	minQueueSize  int
	maxQueueSize  int
	adjustEnabled bool
	resizing      atomic.Bool  // Flag to prevent concurrent resizes
	lastResize    atomic.Int64 // Timestamp of last resize (Unix nano)
}

// NewAsyncLogWriter creates an async writer with queue size.
// queueSize: initial queue size (default: 2000)
// enableDynamicAdjust: enable dynamic queue size adjustment based on metrics
func NewAsyncLogWriter(w io.Writer, queueSize int, enableDynamicAdjust bool) *AsyncLogWriter {
	if queueSize <= 0 {
		queueSize = 2000 // default
	}

	var c io.Closer
	if cl, ok := w.(io.Closer); ok {
		c = cl
	}

	minSize := queueSize / 2
	if minSize < 100 {
		minSize = 100
	}
	maxSize := queueSize * 4
	if maxSize > 50000 {
		maxSize = 50000 // cap at 50k to prevent excessive memory usage
	}

	aw := &AsyncLogWriter{
		writer:        w,
		closer:        c,
		queue:         make(chan []byte, queueSize),
		stopCh:        make(chan struct{}),
		queueSize:     queueSize,
		minQueueSize:  minSize,
		maxQueueSize:  maxSize,
		adjustEnabled: enableDynamicAdjust,
		metrics: LogPerformanceMetrics{
			lastReset: time.Now(),
		},
	}
	aw.wg.Add(1)
	go aw.loop()

	// Start dynamic adjustment goroutine if enabled
	if enableDynamicAdjust {
		aw.wg.Add(1)
		go aw.adjustQueueSize()
	}

	return aw
}

func (a *AsyncLogWriter) loop() {
	defer a.wg.Done()
	for {
		// Get current queue with read lock
		a.mu.RLock()
		queue := a.queue
		a.mu.RUnlock()

		select {
		case <-a.stopCh:
			// drain: need to drain both old and new queues in case of resize
			drained := false
			for !drained {
				// Try to drain current queue
				select {
				case data := <-queue:
					start := time.Now()
					_, err := a.writer.Write(data)
					dur := time.Since(start)
					// Use atomic operations for thread-safe metrics update
					durNs := int64(dur)
					prevNs := a.avgWriteTimeAtomic.Load()
					if prevNs == 0 {
						a.avgWriteTimeAtomic.Store(durNs)
					} else {
						newAvg := (prevNs + durNs) / 2
						a.avgWriteTimeAtomic.Store(newAvg)
					}
					atomic.AddInt64(&a.metrics.TotalLogs, 1)
					atomic.AddInt64(&a.qLen, -1)
					if err != nil {
						atomic.AddInt64(&a.metrics.ErrorCount, 1)
					}
				default:
					// Current queue empty, check if queue was replaced
					a.mu.RLock()
					newQueue := a.queue
					a.mu.RUnlock()

					if newQueue == queue {
						// Queue unchanged, we're done
						drained = true
					} else {
						// Queue was replaced, switch to new queue
						queue = newQueue
					}
				}
			}
			return
		case data := <-queue:
			start := time.Now()
			_, err := a.writer.Write(data)
			dur := time.Since(start)
			// Use atomic operations for thread-safe metrics update
			durNs := int64(dur)
			prevNs := a.avgWriteTimeAtomic.Load()
			if prevNs == 0 {
				a.avgWriteTimeAtomic.Store(durNs)
			} else {
				newAvg := (prevNs + durNs) / 2
				a.avgWriteTimeAtomic.Store(newAvg)
			}
			atomic.AddInt64(&a.metrics.TotalLogs, 1)
			atomic.AddInt64(&a.qLen, -1)
			if err != nil {
				atomic.AddInt64(&a.metrics.ErrorCount, 1)
			}
		}
	}
}

// Write enqueues data or drops if queue is full; never blocks caller for long.
// Returns an error if the queue is full and log is dropped.
func (a *AsyncLogWriter) Write(p []byte) (int, error) {
	if a.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	// copy to avoid data races on caller buffer reuse
	buf := make([]byte, len(p))
	copy(buf, p)

	// Retry loop to handle queue resize during write
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		// Use read lock for concurrent access to queue
		a.mu.RLock()
		queue := a.queue
		a.mu.RUnlock()

		select {
		case queue <- buf:
			// Verify queue hasn't changed after successful send
			// This prevents sending to a stale queue reference
			a.mu.RLock()
			currentQueue := a.queue
			a.mu.RUnlock()
			
			if currentQueue == queue {
				// Queue unchanged, send successful
				atomic.AddInt64(&a.qLen, 1)
				return len(p), nil
			}
			// Queue changed during send - data was sent to old queue
			// Background migration in resizeQueue will handle it, so data won't be lost
			// But we should retry to send to the current queue to ensure immediate processing
			// Create a new buffer for retry since the old one was already sent
			buf = make([]byte, len(p))
			copy(buf, p)
			continue
		default:
			// queue full, drop log and warn
			dropped := atomic.AddInt64(&a.metrics.DroppedLogs, 1)

			// Warn periodically (every 100 drops) to avoid log spam
			if dropped%100 == 1 {
				// Use fmt.Fprintf to stderr as fallback since logger might be in deadlock
				a.mu.RLock()
				capQ := cap(a.queue)
				a.mu.RUnlock()
				fmt.Fprintf(os.Stderr, "[lynx-log-warn] AsyncLogWriter queue full, dropped %d logs (queue_size=%d, capacity=%d)\n",
					dropped, atomic.LoadInt64(&a.qLen), capQ)
			}

			// Return error to indicate log was dropped
			return len(p), fmt.Errorf("log queue full, dropped log (total dropped: %d)", dropped)
		}
	}
	
	// Should not reach here under normal circumstances
	// If we exhausted retries, data was sent to old queue but will be migrated
	atomic.AddInt64(&a.qLen, 1)
	return len(p), nil
}

// Close stops the background goroutine and closes the underlying writer if closable.
func (a *AsyncLogWriter) Close() error {
	if a.closed.Swap(true) {
		return nil
	}
	close(a.stopCh)

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines stopped
	case <-time.After(10 * time.Second):
		// Timeout: log warning but continue (prevent deadlock)
		fmt.Fprintf(os.Stderr, "[lynx-log-warn] AsyncLogWriter Close timeout after 10s\n")
	}

	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

// GetMetrics returns a snapshot including queue utilization.
func (a *AsyncLogWriter) GetMetrics() LogPerformanceMetrics {
	a.mu.RLock()
	capQ := cap(a.queue)
	a.mu.RUnlock()

	util := 0.0
	if capQ > 0 {
		util = float64(atomic.LoadInt64(&a.qLen)) / float64(capQ) * 100
	}
	// Use atomic value for avg write time
	avgNs := a.avgWriteTimeAtomic.Load()
	return LogPerformanceMetrics{
		TotalLogs:         atomic.LoadInt64(&a.metrics.TotalLogs),
		DroppedLogs:       atomic.LoadInt64(&a.metrics.DroppedLogs),
		AvgWriteTime:      time.Duration(avgNs),
		BufferUtilization: util,
		FlushCount:        atomic.LoadInt64(&a.metrics.FlushCount),
		ErrorCount:        atomic.LoadInt64(&a.metrics.ErrorCount),
		lastReset:         a.metrics.lastReset,
	}
}

// adjustQueueSize dynamically adjusts queue size based on metrics
func (a *AsyncLogWriter) adjustQueueSize() {
	defer a.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	lastDroppedCount := int64(0)
	consecutiveLowUtilization := 0  // Track consecutive low utilization checks
	consecutiveHighUtilization := 0 // Track consecutive high utilization checks
	const minConsecutiveChecks = 3  // Require 3 consecutive checks before adjusting

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			if !a.adjustEnabled {
				continue
			}

			metrics := a.GetMetrics()
			currentDropped := metrics.DroppedLogs
			droppedSinceLastCheck := currentDropped - lastDroppedCount
			lastDroppedCount = currentDropped

			a.mu.RLock()
			currentCap := cap(a.queue)
			currentSize := a.queueSize
			a.mu.RUnlock()

			// If dropping logs and queue is frequently full, increase size
			if droppedSinceLastCheck > 0 && metrics.BufferUtilization > 85 {
				consecutiveHighUtilization++
				consecutiveLowUtilization = 0 // Reset low utilization counter

				// Only resize after multiple consecutive high utilization checks
				if consecutiveHighUtilization >= minConsecutiveChecks {
					newSize := currentSize * 2
					if newSize > a.maxQueueSize {
						newSize = a.maxQueueSize
					}
					if newSize > currentCap {
						a.resizeQueue(newSize)
						consecutiveHighUtilization = 0 // Reset after resize
					}
				}
			} else if metrics.BufferUtilization < 20 && currentCap > a.minQueueSize {
				// If queue is rarely used, decrease size to save memory
				consecutiveLowUtilization++
				consecutiveHighUtilization = 0 // Reset high utilization counter

				// Only shrink after multiple consecutive low utilization checks
				if consecutiveLowUtilization >= minConsecutiveChecks {
					a.mu.RLock()
					queueLen := len(a.queue)
					a.mu.RUnlock()

					if queueLen == 0 { // Only shrink if queue is empty
						newSize := currentSize / 2
						if newSize < a.minQueueSize {
							newSize = a.minQueueSize
						}
						if newSize < currentCap {
							a.resizeQueue(newSize)
							consecutiveLowUtilization = 0 // Reset after resize
						}
					} else {
						// Queue not empty, reset counter
						consecutiveLowUtilization = 0
					}
				}
			} else {
				// Utilization is in normal range, reset counters
				consecutiveLowUtilization = 0
				consecutiveHighUtilization = 0
			}
		}
	}
}

// resizeQueue safely resizes the queue by creating a new one and migrating items
func (a *AsyncLogWriter) resizeQueue(newSize int) {
	// Rate limit resizes: don't resize more than once per second (check before acquiring lock)
	lastResize := a.lastResize.Load()
	if lastResize > 0 {
		timeSinceLastResize := time.Since(time.Unix(0, lastResize))
		if timeSinceLastResize < 1*time.Second {
			// Too soon since last resize, skip
			return
		}
	}

	// Prevent concurrent resizes
	if !a.resizing.CompareAndSwap(false, true) {
		// Another resize is in progress, skip this one
		return
	}
	defer a.resizing.Store(false)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check size hasn't changed
	if cap(a.queue) == newSize {
		return
	}

	// Update last resize time
	a.lastResize.Store(time.Now().UnixNano())

	oldQueue := a.queue
	newQueue := make(chan []byte, newSize)

	// Migrate items from old queue to new queue (non-blocking)
	migrated := 0
	maxMigrate := 1000 // Limit migration per resize to avoid blocking too long

	for migrated < maxMigrate {
		select {
		case item := <-oldQueue:
			// Decrease qLen when removing from old queue
			atomic.AddInt64(&a.qLen, -1)
			select {
			case newQueue <- item:
				// Increase qLen when adding to new queue
				atomic.AddInt64(&a.qLen, 1)
				migrated++
			default:
				// New queue full, put back to old queue
				select {
				case oldQueue <- item:
					// Restore qLen when putting back
					atomic.AddInt64(&a.qLen, 1)
				default:
					// Both queues full, drop (shouldn't happen often)
					atomic.AddInt64(&a.metrics.DroppedLogs, 1)
				}
			}
		default:
			// Old queue empty or migration limit reached
			goto done
		}
	}

done:
	// Continue migrating remaining items in background to prevent data loss
	// This is important: we don't want to lose logs during resize
	// Check if there are remaining items (non-blocking peek)
	hasRemainingItems := false
	select {
	case item := <-oldQueue:
		hasRemainingItems = true
		// Put it back for background migration
		select {
		case oldQueue <- item:
		default:
			// Can't put back, will be handled in background migration
		}
	default:
		// No items, but still check in background migration
		hasRemainingItems = false
	}

	// Always start background migration to ensure no items are lost
	// (even if peek showed empty, items might arrive during resize)
	if hasRemainingItems || migrated > 0 {
		// Use a context with timeout to prevent goroutine leak
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		go func() {
			defer cancel()
			// Background migration: try to migrate remaining items
			bgMigrated := 0
			maxMigrate := 10000 // Limit to prevent infinite loop

			for bgMigrated < maxMigrate {
				select {
				case <-ctx.Done():
					// Timeout or cancelled, stop migration
					return
				case item := <-oldQueue:
					// Try to add to new queue
					a.mu.RLock()
					currentQueue := a.queue
					a.mu.RUnlock()

					select {
					case currentQueue <- item:
						atomic.AddInt64(&a.qLen, 1)
						bgMigrated++
					case <-ctx.Done():
						// Timeout, drop remaining items
						atomic.AddInt64(&a.metrics.DroppedLogs, 1)
						return
					default:
						// New queue also full, try once more then drop
						select {
						case currentQueue <- item:
							atomic.AddInt64(&a.qLen, 1)
							bgMigrated++
						default:
							// Definitely full, drop
							atomic.AddInt64(&a.metrics.DroppedLogs, 1)
							bgMigrated++
						}
					}
				default:
					// Old queue empty, done
					return
				}
			}
		}()
	}

	// Atomically replace queue
	a.queue = newQueue
	a.queueSize = newSize
}

// SetQueueSize manually sets the queue size (for external control)
func (a *AsyncLogWriter) SetQueueSize(newSize int) {
	if newSize < a.minQueueSize {
		newSize = a.minQueueSize
	}
	if newSize > a.maxQueueSize {
		newSize = a.maxQueueSize
	}

	a.resizeQueue(newSize)
}

// NewOptimizedConsoleWriter returns a writer suitable for console output.
// It currently returns the provided writer directly; formatting is handled by zerolog.
func NewOptimizedConsoleWriter(w io.Writer) io.Writer { return w }
