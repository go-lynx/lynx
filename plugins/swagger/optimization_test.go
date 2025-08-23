package swagger

import (
	"testing"
	"time"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

func TestSafeStringParsing(t *testing.T) {
	parser := NewAnnotationParser(&spec.Swagger{}, []string{"./"})

	// Test safe split with normal input
	parts := parser.safeSplit("a,b,c", ",")
	assert.Equal(t, []string{"a", "b", "c"}, parts)

	// Test safe split with empty input
	parts = parser.safeSplit("", ",")
	assert.Nil(t, parts)

	// Test safe split with very long input
	longInput := string(make([]byte, 2000))
	parts = parser.safeSplit(longInput, ",")
	assert.Nil(t, parts)

	// Test safe fields
	fields := parser.safeFields("a b c")
	assert.Equal(t, []string{"a", "b", "c"}, fields)
}

func TestMemoryManagement(t *testing.T) {
	parser := NewAnnotationParser(&spec.Swagger{}, []string{"./"})

	// Test initial memory stats
	stats := parser.GetMemoryStats()
	assert.Equal(t, 0, stats["models_count"])
	assert.Equal(t, 0, stats["routes_count"])

	// Test memory optimization
	parser.OptimizeMemory()

	// Test cache clearing
	parser.ClearCache()
	stats = parser.GetMemoryStats()
	assert.Equal(t, 0, stats["models_count"])
	assert.Equal(t, 0, stats["routes_count"])
}

func TestFileWatcherConfig(t *testing.T) {
	config := FileWatcherConfig{
		Enabled:       true,
		Interval:      1 * time.Second,
		DebounceDelay: 500 * time.Millisecond,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		BatchSize:     10,
		HealthCheck:   true,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 1*time.Second, config.Interval)
	assert.Equal(t, 500*time.Millisecond, config.DebounceDelay)
	assert.Equal(t, 3, config.MaxRetries)
}

func TestStringBuilderPool(t *testing.T) {
	pool := NewStringBuilderPool(5)

	// Test getting and putting string builders
	sb1 := pool.Get()
	sb1.WriteString("test")
	assert.Equal(t, "test", sb1.String())

	pool.Put(sb1)

	// Test getting again
	sb2 := pool.Get()
	assert.Equal(t, "", sb2.String()) // Should be reset
}

func TestParseStats(t *testing.T) {
	parser := NewAnnotationParser(&spec.Swagger{}, []string{"./"})

	// Test initial stats
	stats := parser.GetStats()
	assert.Equal(t, 0, stats.TotalFiles)
	assert.Equal(t, 0, stats.SuccessFiles)
	assert.Equal(t, 0, stats.FailedFiles)

	// Test error summary
	summary := parser.GetErrorSummary()
	assert.Empty(t, summary)
}
