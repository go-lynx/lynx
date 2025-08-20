package kafka

import (
	"sync"
)

// GoroutinePool is a simple goroutine pool implementation
type GoroutinePool struct {
	wg sync.WaitGroup
	ch chan struct{}
}

// NewGoroutinePool creates a new goroutine pool
func NewGoroutinePool(size int) *GoroutinePool {
	return &GoroutinePool{
		ch: make(chan struct{}, size),
	}
}

// Submit submits a task to the pool for execution
func (p *GoroutinePool) Submit(task func()) {
	p.wg.Add(1)
	p.ch <- struct{}{} // Acquire token
	go func() {
		defer func() {
			<-p.ch // Release token
			p.wg.Done()
		}()
		task()
	}()
}

// Wait waits for all tasks to complete
func (p *GoroutinePool) Wait() {
	p.wg.Wait()
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
