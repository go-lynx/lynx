package polaris

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// ServiceWatcher 和 ConfigWatcher 模块
// 职责：底层服务变更监听和配置变更监听
// 与 registry_impl.go 的区别：
// - watchers.go: 底层监听能力，直接与 Polaris SDK 交互
// - registry_impl.go: Kratos 框架适配，实现 registry 接口

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

	// 监控指标
	metrics *Metrics
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
		metrics:     nil, // 将在使用时设置
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

	// 记录监听启动指标
	if sw.metrics != nil {
		sw.metrics.RecordSDKOperation("service_watch_start", "success")
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

	// 记录监听停止指标
	if sw.metrics != nil {
		sw.metrics.RecordSDKOperation("service_watch_stop", "success")
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
	// 如果实例数量不同，认为有变化
	if len(sw.lastInstances) != len(newInstances) {
		return true
	}

	// 如果之前没有实例，现在有实例，认为有变化
	if len(sw.lastInstances) == 0 && len(newInstances) > 0 {
		return true
	}

	// 如果之前有实例，现在没有实例，认为有变化
	if len(sw.lastInstances) > 0 && len(newInstances) == 0 {
		return true
	}

	// 如果实例数量相同，进行详细比较
	lastInstancesMap := make(map[string]model.Instance)
	for _, instance := range sw.lastInstances {
		key := instance.GetId()
		lastInstancesMap[key] = instance
	}

	// 检查每个新实例
	for _, newInstance := range newInstances {
		key := newInstance.GetId()
		lastInstance, exists := lastInstancesMap[key]

		if !exists {
			// 发现新实例
			return true
		}

		// 比较实例属性
		if !sw.compareInstance(lastInstance, newInstance) {
			// 实例属性发生变化
			return true
		}

		// 从映射中移除已比较的实例
		delete(lastInstancesMap, key)
	}

	// 如果还有剩余的旧实例，说明有实例被移除
	if len(lastInstancesMap) > 0 {
		return true
	}

	return false
}

// notifyInstancesChanged 通知实例变更
func (sw *ServiceWatcher) notifyInstancesChanged(instances []model.Instance) {
	// 记录实例变更指标
	if sw.metrics != nil {
		sw.metrics.RecordServiceDiscovery(sw.serviceName, sw.namespace, "changed")
	}

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
	configAPI api.ConfigFileAPI
	fileName  string
	group     string
	namespace string

	// 监听控制
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex

	// 回调函数
	onConfigChanged func(config model.ConfigFile)
	onError         func(error)

	// 状态
	isRunning  bool
	lastConfig model.ConfigFile

	// 监控指标
	metrics *Metrics
}

// NewConfigWatcher 创建新的配置监听器
func NewConfigWatcher(configAPI api.ConfigFileAPI, fileName, group, namespace string) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		configAPI: configAPI,
		fileName:  fileName,
		group:     group,
		namespace: namespace,
		ctx:       ctx,
		cancel:    cancel,
		metrics:   nil, // 将在使用时设置
	}
}

// SetOnConfigChanged 设置配置变更回调
func (cw *ConfigWatcher) SetOnConfigChanged(callback func(config model.ConfigFile)) {
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

	// 记录配置监听启动指标
	if cw.metrics != nil {
		cw.metrics.RecordSDKOperation("config_watch_start", "success")
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

	// 记录配置监听停止指标
	if cw.metrics != nil {
		cw.metrics.RecordSDKOperation("config_watch_stop", "success")
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
	// 记录配置检查操作指标
	if cw.metrics != nil {
		cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "start")
		defer func() {
			if cw.metrics != nil {
				cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "success")
			}
		}()
	}

	config, err := cw.configAPI.GetConfigFile(cw.namespace, cw.group, cw.fileName)
	if err != nil {
		log.Errorf("Failed to get config %s:%s: %v", cw.group, cw.fileName, err)
		if cw.metrics != nil {
			cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "error")
		}
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
func (cw *ConfigWatcher) hasConfigChanged(newConfig model.ConfigFile) bool {
	// 如果之前没有配置，现在有配置，认为有变化
	if cw.lastConfig == nil && newConfig != nil {
		return true
	}

	// 如果之前有配置，现在没有配置，认为有变化
	if cw.lastConfig != nil && newConfig == nil {
		return true
	}

	// 如果两个配置都是 nil，认为没有变化
	if cw.lastConfig == nil && newConfig == nil {
		return false
	}

	// 比较配置的命名空间
	if cw.lastConfig.GetNamespace() != newConfig.GetNamespace() {
		return true
	}

	// 比较配置的文件组
	if cw.lastConfig.GetFileGroup() != newConfig.GetFileGroup() {
		return true
	}

	// 比较配置的文件名
	if cw.lastConfig.GetFileName() != newConfig.GetFileName() {
		return true
	}

	// 比较配置的内容
	if cw.lastConfig.GetContent() != newConfig.GetContent() {
		return true
	}

	// 比较是否有内容
	if cw.lastConfig.HasContent() != newConfig.HasContent() {
		return true
	}

	return false
}

// notifyConfigChanged 通知配置变更
func (cw *ConfigWatcher) notifyConfigChanged(config model.ConfigFile) {
	// 记录配置变更指标
	if cw.metrics != nil {
		cw.metrics.RecordConfigChange(cw.fileName, cw.group)
	}

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
func (cw *ConfigWatcher) GetLastConfig() model.ConfigFile {
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

// compareInstance 比较两个实例是否相同
func (sw *ServiceWatcher) compareInstance(instance1, instance2 model.Instance) bool {
	// 比较基本信息
	if instance1.GetId() != instance2.GetId() ||
		instance1.GetHost() != instance2.GetHost() ||
		instance1.GetPort() != instance2.GetPort() ||
		instance1.GetProtocol() != instance2.GetProtocol() ||
		instance1.GetVersion() != instance2.GetVersion() {
		return false
	}

	// 比较权重
	if instance1.GetWeight() != instance2.GetWeight() {
		return false
	}

	// 比较健康状态
	if instance1.IsHealthy() != instance2.IsHealthy() {
		return false
	}

	// 比较隔离状态
	if instance1.IsIsolated() != instance2.IsIsolated() {
		return false
	}

	// 比较元数据
	metadata1 := instance1.GetMetadata()
	metadata2 := instance2.GetMetadata()

	if len(metadata1) != len(metadata2) {
		return false
	}

	for key, value1 := range metadata1 {
		if value2, exists := metadata2[key]; !exists || value1 != value2 {
			return false
		}
	}

	return true
}
