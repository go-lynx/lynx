package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	client      *kgo.Client
	interval    time.Duration
	timeout     time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	isHealthy   bool
	lastCheck   time.Time
	errorCount  int
	maxErrors   int
	onHealthy   func()
	onUnhealthy func(error)
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(client *kgo.Client, interval, timeout time.Duration) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		client:      client,
		interval:    interval,
		timeout:     timeout,
		ctx:         ctx,
		cancel:      cancel,
		isHealthy:   true,
		maxErrors:   3,
		onHealthy:   func() {},
		onUnhealthy: func(err error) {},
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	go hc.run()
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	hc.cancel()
}

// run 运行健康检查循环
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.check()
		}
	}
}

// check 执行健康检查
func (hc *HealthChecker) check() {
	// 通过 Metadata 请求探测集群健康
	ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
	defer cancel()

	// 发送空 MetadataRequest（请求全部 topic 的元数据）
	var req kmsg.MetadataRequest
	_, err := req.RequestWith(ctx, hc.client)

	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.lastCheck = time.Now()

	if err != nil {
		hc.errorCount++
		if hc.isHealthy && hc.errorCount >= hc.maxErrors {
			hc.isHealthy = false
			// 回调不可阻塞主循环
			go hc.onUnhealthy(err)
		}
		log.WarnfCtx(hc.ctx, "Kafka health check failed (%d/%d): %v", hc.errorCount, hc.maxErrors, err)
		return
	}

	if !hc.isHealthy {
		// 状态从不健康 -> 健康
		hc.isHealthy = true
		hc.errorCount = 0
		go hc.onHealthy()
		log.InfofCtx(hc.ctx, "Kafka health recovered")
	} else {
		// 维持健康，清零错误计数
		hc.errorCount = 0
	}
}

// IsHealthy 检查是否健康
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.isHealthy
}

// GetLastCheck 获取最后检查时间
func (hc *HealthChecker) GetLastCheck() time.Time {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.lastCheck
}

// GetErrorCount 获取错误计数
func (hc *HealthChecker) GetErrorCount() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.errorCount
}

// SetCallbacks 设置回调函数
func (hc *HealthChecker) SetCallbacks(onHealthy func(), onUnhealthy func(error)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onHealthy = onHealthy
	hc.onUnhealthy = onUnhealthy
}

// ConnectionManager 连接管理器
type ConnectionManager struct {
	client        *kgo.Client
	brokers       []string
	healthChecker *HealthChecker
	mu            sync.RWMutex
	isConnected   bool
	reconnectChan chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewConnectionManager 创建新的连接管理器
func NewConnectionManager(client *kgo.Client, brokers []string) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	cm := &ConnectionManager{
		client:        client,
		brokers:       brokers,
		reconnectChan: make(chan struct{}, 10),
		ctx:           ctx,
		cancel:        cancel,
	}

	// 创建健康检查器
	cm.healthChecker = NewHealthChecker(client, 30*time.Second, 10*time.Second)
	cm.healthChecker.SetCallbacks(
		func() { cm.onHealthy() },
		func(err error) { cm.onUnhealthy(err) },
	)

	return cm
}

// Start 启动连接管理器
func (cm *ConnectionManager) Start() {
	cm.healthChecker.Start()
	go cm.handleReconnections()
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() {
	cm.cancel()
	cm.healthChecker.Stop()
}

// onHealthy 连接恢复时的回调
func (cm *ConnectionManager) onHealthy() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.isConnected = true
	log.InfofCtx(cm.ctx, "Kafka connection established")
}

// onUnhealthy 连接失败时的回调
func (cm *ConnectionManager) onUnhealthy(err error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.isConnected = false
	log.ErrorfCtx(cm.ctx, "Kafka connection lost: %v", err)

	// 触发重连
	select {
	case cm.reconnectChan <- struct{}{}:
	default:
	}
}

// handleReconnections 处理重连
func (cm *ConnectionManager) handleReconnections() {
	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-cm.reconnectChan:
			cm.reconnect()
		}
	}
}

// reconnect 重连逻辑
func (cm *ConnectionManager) reconnect() {
	log.InfofCtx(cm.ctx, "Attempting to reconnect to Kafka...")
	// franz-go 自带连接管理，这里通过触发一次 Metadata 来加速恢复
	ctx, cancel := context.WithTimeout(cm.ctx, 10*time.Second)
	defer cancel()
	var req kmsg.MetadataRequest
	_, err := req.RequestWith(ctx, cm.client)
	if err != nil {
		log.WarnfCtx(cm.ctx, "Reconnect metadata request failed: %v", err)
	}
	// 轻微退避，避免风暴
	time.Sleep(2 * time.Second)
}

// IsConnected 检查是否已连接
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isConnected
}

// GetHealthChecker 获取健康检查器
func (cm *ConnectionManager) GetHealthChecker() *HealthChecker {
	return cm.healthChecker
}

// ForceReconnect 强制重连
func (cm *ConnectionManager) ForceReconnect() {
	select {
	case cm.reconnectChan <- struct{}{}:
	default:
	}
}
