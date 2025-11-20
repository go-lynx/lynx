package log

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

// mockWriter is a simple mock writer for testing
type mockWriter struct {
	data   []byte
	mu     sync.Mutex
	writes int
}

func (m *mockWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = append(m.data, p...)
	m.writes++
	return len(p), nil
}

func (m *mockWriter) getData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.data...)
}

func (m *mockWriter) getWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writes
}

func TestBatchWriter_Write(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 100*time.Millisecond)

	testData := []byte("test log entry\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	// Wait a bit for batch to potentially flush
	time.Sleep(150 * time.Millisecond)

	// Flush explicitly to ensure data is written
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	written := mock.getData()
	if !bytes.Equal(written, testData) {
		t.Errorf("Written data mismatch: got %q, expected %q", written, testData)
	}

	bw.Close()
}

func TestBatchWriter_BatchSize(t *testing.T) {
	mock := &mockWriter{}
	batchSize := 100
	bw := NewBatchWriter(mock, batchSize, 1*time.Second)

	// Write data smaller than batch size
	smallData := make([]byte, 50)
	for i := range smallData {
		smallData[i] = 'A'
	}

	// Write multiple times
	for i := 0; i < 3; i++ {
		_, err := bw.Write(smallData)
		if err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	// Should not flush yet (total 150 bytes, batch size 100, but first batch not full)
	// Write one more to exceed batch size
	_, err := bw.Write(smallData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Flush to ensure all data is written
	bw.Flush()

	written := mock.getData()
	expectedSize := 4 * len(smallData) // 4 writes
	if len(written) != expectedSize {
		t.Errorf("Written size mismatch: got %d, expected %d", len(written), expectedSize)
	}

	bw.Close()
}

func TestBatchWriter_LargeWrite(t *testing.T) {
	mock := &mockWriter{}
	batchSize := 100
	bw := NewBatchWriter(mock, batchSize, 1*time.Second)

	// Write data larger than batch size
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = 'B'
	}

	n, err := bw.Write(largeData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Write returned %d, expected %d", n, len(largeData))
	}

	// Large writes should be written directly
	bw.Flush()

	written := mock.getData()
	if !bytes.Equal(written, largeData) {
		t.Errorf("Large write data mismatch")
	}

	// Should have been written directly (not batched)
	writeCount := mock.getWriteCount()
	if writeCount < 1 {
		t.Errorf("Expected at least 1 write, got %d", writeCount)
	}

	bw.Close()
}

func TestBatchWriter_FlushInterval(t *testing.T) {
	mock := &mockWriter{}
	flushInterval := 50 * time.Millisecond
	bw := NewBatchWriter(mock, 1024, flushInterval)

	// Write some data
	testData := []byte("test data\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait for flush interval
	time.Sleep(flushInterval + 20*time.Millisecond)

	// Data should be flushed by now
	written := mock.getData()
	if len(written) == 0 {
		t.Error("Data was not flushed after interval")
	}

	bw.Close()
}

func TestBatchWriter_Flush(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 1*time.Second)

	testData := []byte("test flush\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Flush should write data immediately
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	written := mock.getData()
	if !bytes.Equal(written, testData) {
		t.Errorf("Flush data mismatch: got %q, expected %q", written, testData)
	}

	bw.Close()
}

func TestBatchWriter_Close(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 1*time.Second)

	testData := []byte("test close\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Close should flush remaining data
	err = bw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Wait a bit to ensure flush completed
	time.Sleep(50 * time.Millisecond)

	written := mock.getData()
	if !bytes.Equal(written, testData) {
		t.Errorf("Close flush data mismatch: got %q, expected %q", written, testData)
	}

	// Second close should be safe
	err = bw.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}

	// Write after close should fail
	_, err = bw.Write([]byte("should fail\n"))
	if err == nil {
		t.Error("Write after close should fail")
	}
	if err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got %v", err)
	}
}

func TestBatchWriter_ConcurrentWrite(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 100*time.Millisecond)

	// Concurrent writes
	var wg sync.WaitGroup
	numWrites := 10
	wg.Add(numWrites)

	for i := 0; i < numWrites; i++ {
		go func(id int) {
			defer wg.Done()
			data := []byte("concurrent write " + string(rune(id+'0')) + "\n")
			_, err := bw.Write(data)
			if err != nil {
				t.Errorf("Concurrent write %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Flush to ensure all data is written
	bw.Flush()

	written := mock.getData()
	if len(written) == 0 {
		t.Error("No data was written")
	}

	bw.Close()
}

func TestBatchWriter_GetMetrics(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 100*time.Millisecond)

	// Write some data
	testData := []byte("test metrics\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Get metrics
	writes, batches, flushes, _ := bw.GetMetrics()

	if writes < 1 {
		t.Errorf("Expected at least 1 write, got %d", writes)
	}

	t.Logf("Metrics: writes=%d, batches=%d, flushes=%d", writes, batches, flushes)

	bw.Close()
}

func TestBatchWriter_EmptyFlush(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 100*time.Millisecond)

	// Flush with no data should not error
	err := bw.Flush()
	if err != nil {
		t.Errorf("Empty flush failed: %v", err)
	}

	written := mock.getData()
	if len(written) != 0 {
		t.Errorf("Expected no data, got %q", written)
	}

	bw.Close()
}

func TestBatchWriter_ZeroFlushInterval(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBatchWriter(mock, 1024, 0) // No periodic flushing

	testData := []byte("test no interval\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Data should not be flushed automatically
	written := mock.getData()
	if len(written) != 0 {
		t.Error("Data should not be flushed with zero interval")
	}

	// Manual flush should work
	bw.Flush()
	written = mock.getData()
	if !bytes.Equal(written, testData) {
		t.Errorf("Manual flush data mismatch")
	}

	bw.Close()
}

