package kafka

import (
	"sync"
)

// GoroutinePool 是一个简单的 goroutine 池实现
type GoroutinePool struct {
	wg sync.WaitGroup
	ch chan struct{}
}

// NewGoroutinePool 创建一个新的 goroutine 池
func NewGoroutinePool(size int) *GoroutinePool {
	return &GoroutinePool{
		ch: make(chan struct{}, size),
	}
}

// Submit 提交一个任务到池中执行
func (p *GoroutinePool) Submit(task func()) {
	p.wg.Add(1)
	p.ch <- struct{}{} // 获取令牌
	go func() {
		defer func() {
			<-p.ch // 释放令牌
			p.wg.Done()
		}()
		task()
	}()
}

// Wait 等待所有任务完成
func (p *GoroutinePool) Wait() {
	p.wg.Wait()
}

// PoolConfig 协程池配置
type PoolConfig struct {
	Size int // 池大小
}

// DefaultPoolConfig 默认协程池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Size: 10,
	}
}
