package polaris

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// ServiceWatcher 服务监听器
// 监听服务实例变更
type ServiceWatcher struct {
	consumer    api.ConsumerAPI
	serviceName string
	namespace   string

	// 监听控制
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex

	// 回调函数
	onInstancesChanged func(instances []model.Instance)
	onError            func(error)

	// 状态
	isRunning     bool
	lastInstances []model.Instance
}

// NewServiceWatcher 创建新的服务监听器
func NewServiceWatcher(consumer api.ConsumerAPI, serviceName, namespace string) *ServiceWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceWatcher{
		consumer:    consumer,
		serviceName: serviceName,
		namespace:   namespace,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetOnInstancesChanged 设置实例变更回调
func (sw *ServiceWatcher) SetOnInstancesChanged(callback func(instances []model.Instance)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.onInstancesChanged = callback
}

// SetOnError 设置错误回调
func (sw *ServiceWatcher) SetOnError(callback func(error)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.onError = callback
}

// Start 启动监听
func (sw *ServiceWatcher) Start() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.isRunning {
		return
	}

	sw.isRunning = true
	go sw.watchLoop()

	log.Infof("Started watching service: %s in namespace: %s", sw.serviceName, sw.namespace)
}

// Stop 停止监听
func (sw *ServiceWatcher) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.isRunning {
		return
	}

	sw.cancel()
	sw.isRunning = false

	log.Infof("Stopped watching service: %s", sw.serviceName)
}

// watchLoop 监听循环
func (sw *ServiceWatcher) watchLoop() {
	ticker := time.NewTicker(10 * time.Second) // 每10秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-sw.ctx.Done():
			return
		case <-ticker.C:
			sw.checkInstances()
		}
	}
}

// checkInstances 检查实例变更
func (sw *ServiceWatcher) checkInstances() {
	req := &api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   sw.serviceName,
			Namespace: sw.namespace,
		},
	}

	resp, err := sw.consumer.GetInstances(req)
	if err != nil {
		log.Errorf("Failed to get instances for service %s: %v", sw.serviceName, err)
		sw.notifyError(err)
		return
	}

	// 检查实例是否发生变化
	if sw.hasInstancesChanged(resp.Instances) {
		sw.lastInstances = resp.Instances
		sw.notifyInstancesChanged(resp.Instances)

		log.Infof("Service %s instances changed: %d instances",
			sw.serviceName, len(resp.Instances))
	}
}

// hasInstancesChanged 检查实例是否发生变化
func (sw *ServiceWatcher) hasInstancesChanged(newInstances []model.Instance) bool {
	if len(sw.lastInstances) != len(newInstances) {
		return true
	}

	// 简单的实例数量比较，实际应用中可能需要更复杂的比较逻辑
	return true
}

// notifyInstancesChanged 通知实例变更
func (sw *ServiceWatcher) notifyInstancesChanged(instances []model.Instance) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if sw.onInstancesChanged != nil {
		sw.onInstancesChanged(instances)
	}
}

// notifyError 通知错误
func (sw *ServiceWatcher) notifyError(err error) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if sw.onError != nil {
		sw.onError(err)
	}
}

// GetLastInstances 获取最后的实例列表
func (sw *ServiceWatcher) GetLastInstances() []model.Instance {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.lastInstances
}

// IsRunning 检查是否正在运行
func (sw *ServiceWatcher) IsRunning() bool {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.isRunning
}

// ConfigWatcher 配置监听器
// 监听配置变更
type ConfigWatcher struct {
	configAPI polaris.ConfigAPI
	fileName  string
	group     string
	namespace string

	// 监听控制
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex

	// 回调函数
	onConfigChanged func(config polaris.ConfigFile)
	onError         func(error)

	// 状态
	isRunning  bool
	lastConfig polaris.ConfigFile
}

// NewConfigWatcher 创建新的配置监听器
func NewConfigWatcher(configAPI polaris.ConfigAPI, fileName, group, namespace string) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		configAPI: configAPI,
		fileName:  fileName,
		group:     group,
		namespace: namespace,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetOnConfigChanged 设置配置变更回调
func (cw *ConfigWatcher) SetOnConfigChanged(callback func(config polaris.ConfigFile)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onConfigChanged = callback
}

// SetOnError 设置错误回调
func (cw *ConfigWatcher) SetOnError(callback func(error)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onError = callback
}

// Start 启动监听
func (cw *ConfigWatcher) Start() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.isRunning {
		return
	}

	cw.isRunning = true
	go cw.watchLoop()

	log.Infof("Started watching config: %s:%s in namespace: %s",
		cw.group, cw.fileName, cw.namespace)
}

// Stop 停止监听
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.isRunning {
		return
	}

	cw.cancel()
	cw.isRunning = false

	log.Infof("Stopped watching config: %s:%s", cw.group, cw.fileName)
}

// watchLoop 监听循环
func (cw *ConfigWatcher) watchLoop() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-cw.ctx.Done():
			return
		case <-ticker.C:
			cw.checkConfig()
		}
	}
}

// checkConfig 检查配置变更
func (cw *ConfigWatcher) checkConfig() {
	config, err := cw.configAPI.GetConfigFile(cw.namespace, cw.group, cw.fileName)
	if err != nil {
		log.Errorf("Failed to get config %s:%s: %v", cw.group, cw.fileName, err)
		cw.notifyError(err)
		return
	}

	// 检查配置是否发生变化
	if cw.hasConfigChanged(config) {
		cw.lastConfig = config
		cw.notifyConfigChanged(config)

		log.Infof("Config %s:%s changed",
			cw.group, cw.fileName)
	}
}

// hasConfigChanged 检查配置是否发生变化
func (cw *ConfigWatcher) hasConfigChanged(newConfig polaris.ConfigFile) bool {
	if cw.lastConfig == nil {
		return true
	}

	// 简化版本比较，实际项目中可以根据需要实现更精确的比较
	return true
}

// notifyConfigChanged 通知配置变更
func (cw *ConfigWatcher) notifyConfigChanged(config polaris.ConfigFile) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if cw.onConfigChanged != nil {
		cw.onConfigChanged(config)
	}
}

// notifyError 通知错误
func (cw *ConfigWatcher) notifyError(err error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if cw.onError != nil {
		cw.onError(err)
	}
}

// GetLastConfig 获取最后的配置
func (cw *ConfigWatcher) GetLastConfig() polaris.ConfigFile {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.lastConfig
}

// IsRunning 检查是否正在运行
func (cw *ConfigWatcher) IsRunning() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.isRunning
}
