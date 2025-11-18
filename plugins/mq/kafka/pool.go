package kafka

import (
	"sync"
)

// GoroutinePool is a simple goroutine pool implementation
type GoroutinePool struct {
	wg     sync.WaitGroup
	ch     chan struct{}
	closed bool
	mu     sync.RWMutex
}

// NewGoroutinePool creates a new goroutine pool
func NewGoroutinePool(size int) *GoroutinePool {
	return &GoroutinePool{
		ch:     make(chan struct{}, size),
		closed: false,
	}
}

// Submit submits a task to the pool for execution
func (p *GoroutinePool) Submit(task func()) {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	
	if closed {
		// If pool is closed, execute task in current goroutine
		if task != nil {
			task()
		}
		return
	}

	p.wg.Add(1)
	select {
	case p.ch <- struct{}{}: // Acquire token
		go func() {
			defer func() {
				<-p.ch // Release token
				p.wg.Done()
			}()
			if task != nil {
				task()
			}
		}()
	default:
		// If pool is full, execute task in current goroutine
		p.wg.Done()
		if task != nil {
			task()
		}
	}
}

// Wait waits for all tasks to complete
func (p *GoroutinePool) Wait() {
	p.wg.Wait()
}

// Close closes the pool gracefully
// After Close(), no new tasks will be accepted, but existing tasks will complete
func (p *GoroutinePool) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		// Close channel to signal no more tasks
		close(p.ch)
	}
	p.mu.Unlock()
	// Wait for all tasks to complete
	p.wg.Wait()
}

// IsClosed checks if the pool is closed
func (p *GoroutinePool) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// PoolConfig goroutine pool configuration
type PoolConfig struct {
	Size int // Pool size
}

// DefaultPoolConfig default goroutine pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Size: 30,
	}
}
