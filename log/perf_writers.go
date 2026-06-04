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
// It is safe for concurrent use.
type BufferedWriter struct {
	under              io.Writer
	buf                *bufio.Writer
	mu                 sync.Mutex
	metrics            LogPerformanceMetrics
	closed             atomic.Bool
	avgWriteTimeAtomic atomic.Int64 // EMA of write time, nanoseconds
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
	// Flush eagerly on a trailing newline so complete log lines reach the
	// underlying writer promptly; bufio handles size-based flushing otherwise.
	if err == nil && (len(p) > 0 && p[len(p)-1] == '\n') {
		if ferr := b.buf.Flush(); ferr == nil {
			atomic.AddInt64(&b.metrics.FlushCount, 1)
		}
	}
	b.mu.Unlock()
	dur := time.Since(start)

	// Track average write time as a simple EMA (atomic, lock-free).
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
	atomic.AddInt64(&b.metrics.FlushCount, 1)
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
	atomic.AddInt64(&b.metrics.FlushCount, 1)
	return nil
}

// GetMetrics returns a snapshot of metrics.
func (b *BufferedWriter) GetMetrics() LogPerformanceMetrics {
	b.mu.Lock()
	bufferUtilization := b.metrics.BufferUtilization
	lastReset := b.metrics.lastReset
	b.mu.Unlock()

	avgNs := b.avgWriteTimeAtomic.Load()
	return LogPerformanceMetrics{
		TotalLogs:         atomic.LoadInt64(&b.metrics.TotalLogs),
		DroppedLogs:       atomic.LoadInt64(&b.metrics.DroppedLogs),
		AvgWriteTime:      time.Duration(avgNs),
		BufferUtilization: bufferUtilization,
		FlushCount:        atomic.LoadInt64(&b.metrics.FlushCount),
		ErrorCount:        atomic.LoadInt64(&b.metrics.ErrorCount),
		lastReset:         lastReset,
	}
}

// ResetMetrics resets counters.
func (b *BufferedWriter) ResetMetrics() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metrics = LogPerformanceMetrics{lastReset: time.Now()}
	b.avgWriteTimeAtomic.Store(0)
}

// AsyncLogWriter enqueues writes onto a buffered channel drained by a background
// goroutine, so callers never block on I/O. Entries are dropped when the queue
// is full. It is safe for concurrent use.
type AsyncLogWriter struct {
	writer             io.Writer
	closer             io.Closer
	queue              chan []byte
	stopCh             chan struct{}
	wg                 sync.WaitGroup
	metrics            LogPerformanceMetrics
	qLen               int64 // current queue length
	closed             atomic.Bool
	dropWarned         atomic.Int64 // gate to rate-limit drop warnings
	avgWriteTimeAtomic atomic.Int64 // EMA of write time, nanoseconds

	// Dynamic queue size adjustment
	mu            sync.RWMutex
	queueSize     int
	minQueueSize  int
	maxQueueSize  int
	adjustEnabled bool
	resizing      atomic.Bool  // guards against concurrent resizes
	lastResize    atomic.Int64 // last resize time (Unix nano), for rate limiting
}

// NewAsyncLogWriter starts an async writer. queueSize is the initial channel
// capacity (defaults to 2000 when <=0); enableDynamicAdjust lets the queue grow
// or shrink at runtime based on utilization. If w is an io.Closer it is closed
// by Close.
func NewAsyncLogWriter(w io.Writer, queueSize int, enableDynamicAdjust bool) *AsyncLogWriter {
	if queueSize <= 0 {
		queueSize = 2000
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

	if enableDynamicAdjust {
		aw.wg.Add(1)
		go aw.adjustQueueSize()
	}

	return aw
}

func (a *AsyncLogWriter) loop() {
	defer a.wg.Done()
	for {
		a.mu.RLock()
		queue := a.queue
		a.mu.RUnlock()

		select {
		case <-a.stopCh:
			// On shutdown, drain remaining entries. A concurrent resize may swap
			// the queue, so keep draining until the active queue is both empty
			// and unchanged.
			drained := false
			for !drained {
				select {
				case data := <-queue:
					start := time.Now()
					_, err := a.writer.Write(data)
					dur := time.Since(start)
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
					a.mu.RLock()
					newQueue := a.queue
					a.mu.RUnlock()

					if newQueue == queue {
						drained = true
					} else {
						queue = newQueue
					}
				}
			}
			return
		case data := <-queue:
			start := time.Now()
			_, err := a.writer.Write(data)
			dur := time.Since(start)
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
	// Copy the caller's buffer; it may be reused once Write returns.
	buf := make([]byte, len(p))
	copy(buf, p)

	// Retry loop to handle queue resize during write
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		a.mu.RLock()
		queue := a.queue
		a.mu.RUnlock()

		select {
		case queue <- buf:
			a.mu.RLock()
			currentQueue := a.queue
			a.mu.RUnlock()

			if currentQueue == queue {
				atomic.AddInt64(&a.qLen, 1)
				return len(p), nil
			}
			// Queue was resized mid-send: the entry landed in the old queue and
			// will be migrated by resizeQueue. Count as success and do not retry,
			// which would duplicate the entry.
			atomic.AddInt64(&a.qLen, 1)
			return len(p), nil
		default:
			// Queue full: drop the log and periodically warn (every 100 drops)
			// via stderr, since the logger itself may be the thing backed up.
			dropped := atomic.AddInt64(&a.metrics.DroppedLogs, 1)

			if dropped%100 == 1 {
				a.mu.RLock()
				capQ := cap(a.queue)
				a.mu.RUnlock()
				fmt.Fprintf(os.Stderr, "[lynx-log-warn] AsyncLogWriter queue full, dropped %d logs (queue_size=%d, capacity=%d)\n",
					dropped, atomic.LoadInt64(&a.qLen), capQ)
			}

			return len(p), fmt.Errorf("log queue full, dropped log (total dropped: %d)", dropped)
		}
	}

	// Retries exhausted only if a resize raced every attempt; the entry reached
	// the old queue and will be migrated.
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

// adjustQueueSize periodically grows or shrinks the queue based on utilization
// and drop metrics. It requires several consecutive readings in the same
// direction (hysteresis) before resizing to avoid thrashing.
func (a *AsyncLogWriter) adjustQueueSize() {
	defer a.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastDroppedCount := int64(0)
	consecutiveLowUtilization := 0
	consecutiveHighUtilization := 0
	const minConsecutiveChecks = 3

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

			// Grow when dropping logs under sustained high utilization.
			if droppedSinceLastCheck > 0 && metrics.BufferUtilization > 85 {
				consecutiveHighUtilization++
				consecutiveLowUtilization = 0

				if consecutiveHighUtilization >= minConsecutiveChecks {
					newSize := currentSize * 2
					if newSize > a.maxQueueSize {
						newSize = a.maxQueueSize
					}
					if newSize > currentCap {
						a.resizeQueue(newSize)
						consecutiveHighUtilization = 0
					}
				}
			} else if metrics.BufferUtilization < 20 && currentCap > a.minQueueSize {
				// Shrink to reclaim memory when sustained low utilization, but
				// only while the queue is empty so no entries are at risk.
				consecutiveLowUtilization++
				consecutiveHighUtilization = 0

				if consecutiveLowUtilization >= minConsecutiveChecks {
					a.mu.RLock()
					queueLen := len(a.queue)
					a.mu.RUnlock()

					if queueLen == 0 {
						newSize := currentSize / 2
						if newSize < a.minQueueSize {
							newSize = a.minQueueSize
						}
						if newSize < currentCap {
							a.resizeQueue(newSize)
							consecutiveLowUtilization = 0
						}
					} else {
						consecutiveLowUtilization = 0
					}
				}
			} else {
				// Normal range: reset both streaks.
				consecutiveLowUtilization = 0
				consecutiveHighUtilization = 0
			}
		}
	}
}

// resizeQueue swaps the buffered channel for one of newSize, migrating queued
// entries. Resizes are rate limited to once per second and serialized so only
// one runs at a time; entries that can't be migrated synchronously are handed
// to a bounded background goroutine to avoid losing logs.
func (a *AsyncLogWriter) resizeQueue(newSize int) {
	lastResize := a.lastResize.Load()
	if lastResize > 0 {
		timeSinceLastResize := time.Since(time.Unix(0, lastResize))
		if timeSinceLastResize < 1*time.Second {
			return
		}
	}

	if !a.resizing.CompareAndSwap(false, true) {
		return
	}
	defer a.resizing.Store(false)

	a.mu.Lock()
	defer a.mu.Unlock()

	if cap(a.queue) == newSize {
		return
	}

	a.lastResize.Store(time.Now().UnixNano())

	oldQueue := a.queue
	newQueue := make(chan []byte, newSize)

	migrated := 0
	maxMigrate := 1000 // bound synchronous migration so the lock isn't held too long

	for migrated < maxMigrate {
		select {
		case item := <-oldQueue:
			atomic.AddInt64(&a.qLen, -1)
			select {
			case newQueue <- item:
				atomic.AddInt64(&a.qLen, 1)
				migrated++
			default:
				select {
				case oldQueue <- item:
					atomic.AddInt64(&a.qLen, 1)
				default:
					atomic.AddInt64(&a.metrics.DroppedLogs, 1)
				}
			}
		default:
			goto done
		}
	}

done:
	// Peek for entries left after the synchronous bound; anything remaining (or
	// arriving during the swap) is migrated in the background goroutine below.
	hasRemainingItems := false
	select {
	case item := <-oldQueue:
		hasRemainingItems = true
		select {
		case oldQueue <- item:
		default:
		}
	default:
		hasRemainingItems = false
	}

	if hasRemainingItems || migrated > 0 {
		// Bounded by timeout and a max count to prevent goroutine/loop leaks.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		go func() {
			defer cancel()
			bgMigrated := 0
			maxMigrate := 10000

			for bgMigrated < maxMigrate {
				select {
				case <-ctx.Done():
					return
				case item := <-oldQueue:
					a.mu.RLock()
					currentQueue := a.queue
					a.mu.RUnlock()

					select {
					case currentQueue <- item:
						atomic.AddInt64(&a.qLen, 1)
						bgMigrated++
					case <-ctx.Done():
						atomic.AddInt64(&a.metrics.DroppedLogs, 1)
						return
					default:
						// New queue also full: one more try, then drop.
						select {
						case currentQueue <- item:
							atomic.AddInt64(&a.qLen, 1)
							bgMigrated++
						default:
							atomic.AddInt64(&a.metrics.DroppedLogs, 1)
							bgMigrated++
						}
					}
				default:
					return
				}
			}
		}()
	}

	a.queue = newQueue
	a.queueSize = newSize
}

// SetQueueSize requests a queue resize, clamped to [minQueueSize, maxQueueSize].
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
