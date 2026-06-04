package log

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// BatchWriter buffers writes and flushes them together to reduce syscalls.
// It runs a background flush loop and is safe for concurrent use.
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

// NewBatchWriter starts a BatchWriter that flushes when the buffer reaches
// batchSize bytes or every flushInterval (<=0 disables periodic flushing).
func NewBatchWriter(w io.Writer, batchSize int, flushInterval time.Duration) *BatchWriter {
	bw := &BatchWriter{
		underlying:    w,
		batch:         make([]byte, 0, batchSize*2), // headroom to absorb a full batch plus one write
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

	// Oversized writes bypass the buffer: flush pending data, then write directly.
	if len(p) > bw.batchSize {
		bw.batchMu.Lock()
		var flushErr error
		if len(bw.batch) > 0 {
			flushErr = bw.flushLocked()
		}
		bw.batchMu.Unlock()

		if flushErr != nil {
			return 0, flushErr
		}

		n, err := bw.underlying.Write(p)
		bw.totalWrites.Add(1)
		if err != nil {
			bw.errorCount.Add(1)
		}
		return n, err
	}

	// Normal path: copy any batch that needs flushing under the lock, then
	// perform the actual write outside it to minimize contention.
	var toFlush []byte
	var needsFlush bool

	bw.batchMu.Lock()
	if len(bw.batch) > 0 && len(bw.batch)+len(p) > bw.batchSize {
		toFlush = make([]byte, len(bw.batch))
		copy(toFlush, bw.batch)
		bw.batch = bw.batch[:0]
		needsFlush = true
	}

	bw.batch = append(bw.batch, p...)
	batchFull := len(bw.batch) >= bw.batchSize
	if batchFull {
		toFlush = make([]byte, len(bw.batch))
		copy(toFlush, bw.batch)
		bw.batch = bw.batch[:0]
		needsFlush = true
	}
	bw.batchMu.Unlock()

	bw.totalWrites.Add(1)

	if needsFlush && len(toFlush) > 0 {
		_, err := bw.underlying.Write(toFlush)
		if err != nil {
			// Best-effort recovery: prepend the unflushed bytes back onto the
			// batch when there is room, otherwise the data is lost.
			bw.batchMu.Lock()
			if len(bw.batch)+len(toFlush) <= bw.batchSize*2 {
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

// flushLocked writes out the buffered batch. It must be called with batchMu held
// and temporarily releases the lock during the underlying write.
func (bw *BatchWriter) flushLocked() error {
	if len(bw.batch) == 0 {
		return nil
	}

	data := make([]byte, len(bw.batch))
	copy(data, bw.batch)
	bw.batch = bw.batch[:0]

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

// Flush writes out any buffered data immediately.
func (bw *BatchWriter) Flush() error {
	bw.batchMu.Lock()
	defer bw.batchMu.Unlock()
	return bw.flushLocked()
}

// flushLoop flushes the batch on each tick until stopped, with a final flush on shutdown.
func (bw *BatchWriter) flushLoop() {
	defer bw.wg.Done()

	if bw.flushInterval <= 0 {
		return
	}

	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bw.stopCh:
			err := bw.Flush()
			if err != nil {
				return
			}
			return
		case <-ticker.C:
			err := bw.Flush()
			if err != nil {
				return
			}
		}
	}
}

// Close stops the flush loop and flushes any remaining data. It is idempotent.
func (bw *BatchWriter) Close() error {
	if bw.closed.Swap(true) {
		return nil
	}

	close(bw.stopCh)
	bw.wg.Wait()

	return bw.Flush()
}

// GetMetrics returns cumulative counts of writes, batches, flushes, and errors.
func (bw *BatchWriter) GetMetrics() (writes, batches, flushes, errors int64) {
	return bw.totalWrites.Load(),
		bw.totalBatches.Load(),
		bw.totalFlushes.Load(),
		bw.errorCount.Load()
}
