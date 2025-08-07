package polaris

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConcurrentStateAccess 测试并发状态访问
func TestConcurrentStateAccess(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 启动多个 goroutine 并发访问状态
	var wg sync.WaitGroup
	concurrentCount := 100

	// 测试并发读取状态
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 并发调用状态检查方法
			_ = plugin.IsInitialized()
			_ = plugin.IsDestroyed()
			_ = plugin.checkInitialized()
		}()
	}

	wg.Wait()

	// 验证状态一致性
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())
}

// TestConcurrentWatcherManagement 测试并发监听器管理
func TestConcurrentWatcherManagement(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 模拟初始化
	plugin.setInitialized()

	var wg sync.WaitGroup
	concurrentCount := 10
	serviceName := "test-service"

	// 并发创建监听器
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 这里只是测试并发安全性，不实际创建监听器
			plugin.watcherMutex.RLock()
			_ = plugin.activeWatchers[serviceName]
			plugin.watcherMutex.RUnlock()
		}()
	}

	wg.Wait()
}

// TestConcurrentCacheAccess 测试并发缓存访问
func TestConcurrentCacheAccess(t *testing.T) {
	plugin := NewPolarisControlPlane()

	var wg sync.WaitGroup
	concurrentCount := 50

	// 并发写入缓存
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			serviceName := fmt.Sprintf("service-%d", index)
			// 模拟缓存更新
			plugin.cacheMutex.Lock()
			if plugin.serviceCache == nil {
				plugin.serviceCache = make(map[string]interface{})
			}
			plugin.serviceCache[serviceName] = map[string]interface{}{
				"service": serviceName,
				"index":   index,
			}
			plugin.cacheMutex.Unlock()
		}(i)
	}

	// 并发读取缓存
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			serviceName := fmt.Sprintf("service-%d", index)
			// 模拟缓存读取
			plugin.cacheMutex.RLock()
			_ = plugin.serviceCache[serviceName]
			plugin.cacheMutex.RUnlock()
		}(i)
	}

	wg.Wait()

	// 验证缓存一致性
	plugin.cacheMutex.RLock()
	cacheSize := len(plugin.serviceCache)
	plugin.cacheMutex.RUnlock()

	assert.Equal(t, concurrentCount, cacheSize)
}

// TestAtomicOperations 测试原子操作
func TestAtomicOperations(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 测试原子状态设置
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	plugin.setInitialized()
	assert.True(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	plugin.setDestroyed()
	assert.True(t, plugin.IsInitialized()) // 一旦初始化，状态不会改变
	assert.True(t, plugin.IsDestroyed())
}

// TestStateConsistency 测试状态一致性
func TestStateConsistency(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 初始状态
	assert.False(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	// 设置初始化状态
	plugin.setInitialized()
	assert.True(t, plugin.IsInitialized())
	assert.False(t, plugin.IsDestroyed())

	// 检查状态检查方法
	err := plugin.checkInitialized()
	assert.Nil(t, err)

	// 设置销毁状态
	plugin.setDestroyed()
	assert.True(t, plugin.IsInitialized()) // 初始化状态不会改变
	assert.True(t, plugin.IsDestroyed())

	// 检查状态检查方法应该返回错误
	err = plugin.checkInitialized()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "has been destroyed")
}

// BenchmarkConcurrentStateAccess 并发状态访问性能测试
func BenchmarkConcurrentStateAccess(b *testing.B) {
	plugin := NewPolarisControlPlane()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = plugin.IsInitialized()
			_ = plugin.IsDestroyed()
			_ = plugin.checkInitialized()
		}
	})
}

// BenchmarkConcurrentCacheAccess 并发缓存访问性能测试
func BenchmarkConcurrentCacheAccess(b *testing.B) {
	plugin := NewPolarisControlPlane()

	// 初始化缓存
	plugin.cacheMutex.Lock()
	plugin.serviceCache = make(map[string]interface{})
	plugin.cacheMutex.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", counter)

			// 写入
			plugin.cacheMutex.Lock()
			plugin.serviceCache[key] = counter
			plugin.cacheMutex.Unlock()

			// 读取
			plugin.cacheMutex.RLock()
			_ = plugin.serviceCache[key]
			plugin.cacheMutex.RUnlock()

			counter++
		}
	})
}
