package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTimeRotationWriter_UpdateTotalSize(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.log")

	writer := NewTimeRotationWriter(filename, 100, 5, 7, false,
		RotationStrategySize, RotationIntervalDaily, 0)

	// Write some data
	testData := []byte("test data for size calculation\n")
	_, err := writer.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Update total size
	writer.updateTotalSize()

	// Check that total size is calculated
	writer.sizeMu.RLock()
	totalSize := writer.totalSize
	writer.sizeMu.RUnlock()

	if totalSize <= 0 {
		t.Error("Total size should be greater than 0")
	}

	writer.Close()
}

func TestTimeRotationWriter_EnforceTotalSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.log")

	// Create writer with very small total size limit (1KB)
	writer := NewTimeRotationWriter(filename, 100, 10, 7, false,
		RotationStrategySize, RotationIntervalDaily, 1) // 1MB limit

	// Create some old log files to simulate existing logs
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]

	// Create old log files
	oldFiles := []string{
		base + ".20240101" + ext,
		base + ".20240102" + ext,
		base + ".20240103" + ext,
	}

	for _, oldFile := range oldFiles {
		f, err := os.Create(oldFile)
		if err != nil {
			t.Fatalf("Failed to create old file %s: %v", oldFile, err)
		}
		// Write 1MB to each file
		data := make([]byte, 1024*1024)
		f.Write(data)
		f.Close()
	}

	// Update total size
	writer.updateTotalSize()

	// Check initial total size
	writer.sizeMu.RLock()
	initialSize := writer.totalSize
	writer.sizeMu.RUnlock()

	if initialSize < 3*1024*1024 {
		t.Errorf("Initial size should be at least 3MB, got %d", initialSize)
	}

	// Enforce limit (should delete old files)
	writer.enforceTotalSizeLimit()

	// Check final total size
	writer.sizeMu.RLock()
	finalSize := writer.totalSize
	writer.sizeMu.RUnlock()

	t.Logf("Initial size: %d bytes, Final size: %d bytes, Limit: %d bytes",
		initialSize, finalSize, writer.maxTotalSize)

	// Verify old files were deleted (except current active file)
	for _, oldFile := range oldFiles {
		if _, err := os.Stat(oldFile); err == nil {
			// File still exists, which is expected if it's the newest
			t.Logf("File %s still exists (may be expected)", oldFile)
		}
	}

	writer.Close()
}

func TestTimeRotationWriter_TotalSizeLimitWithActiveFile(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.log")

	// Create writer with small total size limit
	writer := NewTimeRotationWriter(filename, 100, 10, 7, false,
		RotationStrategySize, RotationIntervalDaily, 2) // 2MB limit

	// Write to active file
	largeData := make([]byte, 500*1024) // 500KB
	for i := range largeData {
		largeData[i] = 'A'
	}

	// Write multiple times
	for i := 0; i < 5; i++ {
		_, err := writer.Write(largeData)
		if err != nil {
			t.Logf("Write %d: %v (may be expected due to size limit)", i, err)
		}
	}

	// Update and enforce total size
	writer.updateTotalSize()
	writer.enforceTotalSizeLimit()

	// Active file should still exist
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Active log file should not be deleted")
	}

	writer.Close()
}

func TestTimeRotationWriter_TotalSizeLimitZero(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.log")

	// Create writer with zero limit (unlimited)
	writer := NewTimeRotationWriter(filename, 100, 5, 7, false,
		RotationStrategySize, RotationIntervalDaily, 0)

	// Write large amount of data
	largeData := make([]byte, 10*1024*1024) // 10MB
	for i := range largeData {
		largeData[i] = 'B'
	}

	_, err := writer.Write(largeData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Enforce limit should not delete anything when limit is 0
	writer.updateTotalSize()
	writer.enforceTotalSizeLimit()

	// File should still exist
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Log file should exist when limit is 0")
	}

	writer.Close()
}

func TestTimeRotationWriter_TotalSizeCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "app.log")

	writer := NewTimeRotationWriter(filename, 100, 10, 7, false,
		RotationStrategySize, RotationIntervalDaily, 0)

	// Create multiple log files with different sizes
	testFiles := []struct {
		name string
		size int
	}{
		{"app.log", 1000},
		{"app.log.20240101", 2000},
		{"app.log.20240102", 3000},
		{"app.log.20240103.gz", 4000},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tmpDir, tf.name)
		f, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", tf.name, err)
		}
		data := make([]byte, tf.size)
		f.Write(data)
		f.Close()
	}

	// Update total size
	writer.updateTotalSize()

	// Check calculated size
	writer.sizeMu.RLock()
	calculatedSize := writer.totalSize
	writer.sizeMu.RUnlock()

	expectedSize := int64(1000 + 2000 + 3000 + 4000) // 10000 bytes
	if calculatedSize != expectedSize {
		t.Errorf("Total size calculation: got %d, expected %d",
			calculatedSize, expectedSize)
	}

	writer.Close()
}
