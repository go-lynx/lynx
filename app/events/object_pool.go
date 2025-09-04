package events

import (
	"sync"
)

// EventBufferPool provides a pool of event buffers to reduce memory allocations
type EventBufferPool struct {
	pool sync.Pool
}

// NewEventBufferPool creates a new event buffer pool
func NewEventBufferPool() *EventBufferPool {
	return &EventBufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate with larger capacity to match typical batch sizes
				return make([]LynxEvent, 0, 64)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *EventBufferPool) Get() []LynxEvent {
	return p.pool.Get().([]LynxEvent)
}

// Put returns a buffer to the pool after clearing it
func (p *EventBufferPool) Put(buf []LynxEvent) {
	// Clear the buffer before returning to pool
	buf = buf[:0]
	p.pool.Put(buf)
}

// GetWithCapacity retrieves a buffer with specified capacity
func (p *EventBufferPool) GetWithCapacity(capacity int) []LynxEvent {
	buf := p.Get()
	// If the buffer from the pool has sufficient capacity, use it directly
	if cap(buf) >= capacity {
		return buf
	}
	// If capacity is insufficient, return to pool first, then create new
	p.Put(buf)
	return make([]LynxEvent, 0, capacity)
}

// MetadataPool provides a pool of metadata maps
type MetadataPool struct {
	pool sync.Pool
}

// NewMetadataPool creates a new metadata pool
func NewMetadataPool() *MetadataPool {
	return &MetadataPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make(map[string]any, 8) // Pre-allocate with reasonable size
			},
		},
	}
}

// Get retrieves a metadata map from the pool
func (p *MetadataPool) Get() map[string]any {
	return p.pool.Get().(map[string]any)
}

// Put returns a metadata map to the pool after clearing it
func (p *MetadataPool) Put(m map[string]any) {
	// Clear the map before returning to pool
	for k := range m {
		delete(m, k)
	}
	p.pool.Put(m)
}

// Global pools for reuse across all event buses
var (
	globalEventBufferPool = NewEventBufferPool()
	globalMetadataPool    = NewMetadataPool()
)

// GetGlobalEventBufferPool returns the global event buffer pool
func GetGlobalEventBufferPool() *EventBufferPool {
	return globalEventBufferPool
}

// GetGlobalMetadataPool returns the global metadata pool
func GetGlobalMetadataPool() *MetadataPool {
	return globalMetadataPool
}
