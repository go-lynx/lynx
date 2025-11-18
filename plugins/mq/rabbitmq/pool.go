package rabbitmq

import (
	"sync"
)

// GoroutinePool manages a pool of goroutines
type GoroutinePool struct {
	workers chan func()
	wg      sync.WaitGroup
	closed  bool
	mu      sync.RWMutex
}

// NewGoroutinePool creates a new goroutine pool
func NewGoroutinePool(size int) *GoroutinePool {
	pool := &GoroutinePool{
		workers: make(chan func(), size),
	}

	// Start worker goroutines
	for i := 0; i < size; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker is the worker goroutine
func (p *GoroutinePool) worker() {
	defer p.wg.Done()

	for task := range p.workers {
		if task != nil {
			task()
		}
	}
}

// Submit submits a task to the pool
func (p *GoroutinePool) Submit(task func()) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return
	}

	select {
	case p.workers <- task:
	default:
		// If pool is full, execute task in current goroutine
		if task != nil {
			task()
		}
	}
}

// Wait waits for all tasks to complete
func (p *GoroutinePool) Wait() {
	p.mu.Lock()
	if !p.closed {
		close(p.workers)
		p.closed = true
	}
	p.mu.Unlock()

	p.wg.Wait()
}

// Close closes the pool and waits for all goroutines to finish
func (p *GoroutinePool) Close() {
	p.mu.Lock()
	if !p.closed {
		close(p.workers)
		p.closed = true
	}
	p.mu.Unlock()
	// Wait for all goroutines to finish to prevent leaks
	p.wg.Wait()
}

// IsClosed checks if the pool is closed
func (p *GoroutinePool) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}
