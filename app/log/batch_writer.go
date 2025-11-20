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
	underlying io.Writer
	batch      []byte
	batchMu    sync.Mutex
	batchSize  int           // Maximum batch size in bytes
	flushInterval time.Duration // Maximum time to wait before flushing
	stopCh     chan struct{}
	wg         sync.WaitGroup
	closed     atomic.Bool
	
	// Metrics
	totalWrites int64
	totalBatches int64
	totalFlushes int64
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
	
	bw.batchMu.Lock()
	defer bw.batchMu.Unlock()
	
	// If single write exceeds batch size, flush first
	if len(bw.batch) > 0 && len(bw.batch)+len(p) > bw.batchSize {
		if err := bw.flushLocked(); err != nil {
			return 0, err
		}
	}
	
	// If single write is larger than batch size, write directly
	if len(p) > bw.batchSize {
		// Flush any pending data first
		if len(bw.batch) > 0 {
			if err := bw.flushLocked(); err != nil {
				return 0, err
			}
		}
		// Write large data directly
		n, err := bw.underlying.Write(p)
		atomic.AddInt64(&bw.totalWrites, 1)
		return n, err
	}
	
	// Add to batch
	bw.batch = append(bw.batch, p...)
	atomic.AddInt64(&bw.totalWrites, 1)
	
	// Flush if batch is full
	if len(bw.batch) >= bw.batchSize {
		return len(p), bw.flushLocked()
	}
	
	return len(p), nil
}

// flushLocked flushes the batch (must be called with batchMu locked)
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
		atomic.AddInt64(&bw.totalBatches, 1)
		atomic.AddInt64(&bw.totalFlushes, 1)
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
func (bw *BatchWriter) GetMetrics() (writes, batches, flushes int64) {
	return atomic.LoadInt64(&bw.totalWrites),
		atomic.LoadInt64(&bw.totalBatches),
		atomic.LoadInt64(&bw.totalFlushes)
}

