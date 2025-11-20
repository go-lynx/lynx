package log

import (
	"bufio"
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
	// Metrics (EMA for avg write time)
	prev := b.metrics.AvgWriteTime
	if prev == 0 {
		b.metrics.AvgWriteTime = dur
	} else {
		b.metrics.AvgWriteTime = (prev + dur) / 2
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
	return LogPerformanceMetrics{
		TotalLogs:         atomic.LoadInt64(&b.metrics.TotalLogs),
		DroppedLogs:       atomic.LoadInt64(&b.metrics.DroppedLogs),
		AvgWriteTime:      b.metrics.AvgWriteTime,
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
}

// NewAsyncLogWriter creates an async writer with queue size.
func NewAsyncLogWriter(w io.Writer, queueSize int) *AsyncLogWriter {
	var c io.Closer
	if cl, ok := w.(io.Closer); ok {
		c = cl
	}
	aw := &AsyncLogWriter{
		writer: w,
		closer: c,
		queue:  make(chan []byte, queueSize),
		stopCh: make(chan struct{}),
		metrics: LogPerformanceMetrics{
			lastReset: time.Now(),
		},
	}
	aw.wg.Add(1)
	go aw.loop()
	return aw
}

func (a *AsyncLogWriter) loop() {
	defer a.wg.Done()
	for {
		select {
		case <-a.stopCh:
			// drain
			for {
				select {
				case data := <-a.queue:
					start := time.Now()
					_, err := a.writer.Write(data)
					dur := time.Since(start)
					prev := a.metrics.AvgWriteTime
					if prev == 0 {
						a.metrics.AvgWriteTime = dur
					} else {
						a.metrics.AvgWriteTime = (prev + dur) / 2
					}
					atomic.AddInt64(&a.metrics.TotalLogs, 1)
					atomic.AddInt64(&a.qLen, -1)
					if err != nil {
						atomic.AddInt64(&a.metrics.ErrorCount, 1)
					}
				default:
					return
				}
			}
		case data := <-a.queue:
			start := time.Now()
			_, err := a.writer.Write(data)
			dur := time.Since(start)
			prev := a.metrics.AvgWriteTime
			if prev == 0 {
				a.metrics.AvgWriteTime = dur
			} else {
				a.metrics.AvgWriteTime = (prev + dur) / 2
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
	select {
	case a.queue <- buf:
		atomic.AddInt64(&a.qLen, 1)
		return len(p), nil
	default:
		// queue full, drop log and warn
		dropped := atomic.AddInt64(&a.metrics.DroppedLogs, 1)
		
		// Warn periodically (every 100 drops) to avoid log spam
		if dropped%100 == 1 {
			// Use fmt.Fprintf to stderr as fallback since logger might be in deadlock
			fmt.Fprintf(os.Stderr, "[lynx-log-warn] AsyncLogWriter queue full, dropped %d logs (queue_size=%d, capacity=%d)\n",
				dropped, atomic.LoadInt64(&a.qLen), cap(a.queue))
		}
		
		// Return error to indicate log was dropped
		return len(p), fmt.Errorf("log queue full, dropped log (total dropped: %d)", dropped)
	}
}

// Close stops the background goroutine and closes the underlying writer if closable.
func (a *AsyncLogWriter) Close() error {
	if a.closed.Swap(true) {
		return nil
	}
	close(a.stopCh)
	a.wg.Wait()
	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

// GetMetrics returns a snapshot including queue utilization.
func (a *AsyncLogWriter) GetMetrics() LogPerformanceMetrics {
	util := 0.0
	capQ := cap(a.queue)
	if capQ > 0 {
		util = float64(atomic.LoadInt64(&a.qLen)) / float64(capQ) * 100
	}
	return LogPerformanceMetrics{
		TotalLogs:         atomic.LoadInt64(&a.metrics.TotalLogs),
		DroppedLogs:       atomic.LoadInt64(&a.metrics.DroppedLogs),
		AvgWriteTime:      a.metrics.AvgWriteTime,
		BufferUtilization: util,
		FlushCount:        atomic.LoadInt64(&a.metrics.FlushCount),
		ErrorCount:        atomic.LoadInt64(&a.metrics.ErrorCount),
		lastReset:         a.metrics.lastReset,
	}
}

// NewOptimizedConsoleWriter returns a writer suitable for console output.
// It currently returns the provided writer directly; formatting is handled by zerolog.
func NewOptimizedConsoleWriter(w io.Writer) io.Writer { return w }
