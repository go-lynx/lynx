package log

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// BatchWriter wraps an io.Writer with batch writing optimization
// It collects multiple writes and flushes them together to reduce system calls
type BatchWriter struct {
	underlying    io.Writer
	batch         []byte
	batchMu       sync.Mutex
	batchSize     int           // Maximum batch size in bytes
	flushInterval time.Duration // Maximum time to wait before flushing
	stopCh        chan struct{}
	wg            sync.WaitGroup
	closed        atomic.Bool

	// Metrics
	totalWrites  atomic.Int64
	totalBatches atomic.Int64
	totalFlushes atomic.Int64
	errorCount   atomic.Int64
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(w io.Writer, batchSize int, flushInterval time.Duration) *BatchWriter {
	bw := &BatchWriter{
		underlying:    w,
		batch:         make([]byte, 0, batchSize*2), // Pre-allocate with some headroom
		batchSize:     batchSize,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}

	bw.wg.Add(1)
	go bw.flushLoop()

	return bw
}

// Write implements io.Writer
func (bw *BatchWriter) Write(p []byte) (int, error) {
	if bw.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	// If single write is larger than batch size, write directly without locking
	if len(p) > bw.batchSize {
		// Flush any pending data first (with lock)
		bw.batchMu.Lock()
		var flushErr error
		if len(bw.batch) > 0 {
			flushErr = bw.flushLocked()
		}
		bw.batchMu.Unlock()

		if flushErr != nil {
			return 0, flushErr
		}

		// Write large data directly (no lock needed)
		n, err := bw.underlying.Write(p)
		bw.totalWrites.Add(1)
		if err != nil {
			bw.errorCount.Add(1)
		}
		return n, err
	}

	// For normal writes, minimize lock holding time
	var toFlush []byte
	var needsFlush bool

	bw.batchMu.Lock()
	// Check if we need to flush before adding
	if len(bw.batch) > 0 && len(bw.batch)+len(p) > bw.batchSize {
		// Copy batch for flushing outside lock
		toFlush = make([]byte, len(bw.batch))
		copy(toFlush, bw.batch)
		bw.batch = bw.batch[:0] // Reset batch
		needsFlush = true
	}

	// Add to batch
	bw.batch = append(bw.batch, p...)
	batchFull := len(bw.batch) >= bw.batchSize
	if batchFull {
		// Copy batch for flushing outside lock
		toFlush = make([]byte, len(bw.batch))
		copy(toFlush, bw.batch)
		bw.batch = bw.batch[:0] // Reset batch
		needsFlush = true
	}
	bw.batchMu.Unlock()

	bw.totalWrites.Add(1)

	// Flush outside lock to reduce contention
	if needsFlush && len(toFlush) > 0 {
		_, err := bw.underlying.Write(toFlush)
		if err != nil {
			// Write failed: need to restore data to batch to prevent data loss
			// This is a best-effort recovery - if batch is full, data may still be lost
			bw.batchMu.Lock()
			// Try to restore if there's space (unlikely but possible if batch was reset)
			if len(bw.batch)+len(toFlush) <= bw.batchSize*2 {
				// Prepend toFlush to batch to maintain order
				restored := make([]byte, len(toFlush), len(toFlush)+len(bw.batch))
				copy(restored, toFlush)
				bw.batch = append(restored, bw.batch...)
			}
			bw.batchMu.Unlock()
			bw.errorCount.Add(1)
			return len(p), err
		}
		bw.totalBatches.Add(1)
		bw.totalFlushes.Add(1)
	}

	return len(p), nil
}

// flushLocked flushes the batch (must be called with batchMu locked)
// Note: This method is kept for backward compatibility but Write() now handles flushing outside lock
func (bw *BatchWriter) flushLocked() error {
	if len(bw.batch) == 0 {
		return nil
	}

	data := make([]byte, len(bw.batch))
	copy(data, bw.batch)
	bw.batch = bw.batch[:0] // Reset batch

	// Unlock before writing to avoid blocking other writes
	bw.batchMu.Unlock()
	_, err := bw.underlying.Write(data)
	bw.batchMu.Lock()

	if err == nil {
		bw.totalBatches.Add(1)
		bw.totalFlushes.Add(1)
	} else {
		bw.errorCount.Add(1)
	}

	return err
}

// Flush forces a flush of the current batch
func (bw *BatchWriter) Flush() error {
	bw.batchMu.Lock()
	defer bw.batchMu.Unlock()
	return bw.flushLocked()
}

// flushLoop periodically flushes the batch
func (bw *BatchWriter) flushLoop() {
	defer bw.wg.Done()

	if bw.flushInterval <= 0 {
		return // No periodic flushing
	}

	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bw.stopCh:
			// Final flush on shutdown
			bw.Flush()
			return
		case <-ticker.C:
			bw.Flush()
		}
	}
}

// Close implements io.Closer
func (bw *BatchWriter) Close() error {
	if bw.closed.Swap(true) {
		return nil
	}

	close(bw.stopCh)
	bw.wg.Wait()

	// Final flush
	return bw.Flush()
}

// GetMetrics returns batch writer metrics
func (bw *BatchWriter) GetMetrics() (writes, batches, flushes, errors int64) {
	return bw.totalWrites.Load(),
		bw.totalBatches.Load(),
		bw.totalFlushes.Load(),
		bw.errorCount.Load()
}
