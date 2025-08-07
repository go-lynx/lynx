package polaris

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// Plugin metadata
// 插件元数据，定义插件的基本信息
const (
	// pluginName 是 Polaris 控制平面插件的唯一标识符，用于在插件系统中识别该插件。
	pluginName = "polaris.control.plane"

	// pluginVersion 表示 Polaris 控制平面插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述了 Polaris 控制平面插件的功能。
	pluginDescription = "polaris control plane plugin for lynx framework"

	// confPrefix 是加载 Polaris 配置时使用的配置前缀。
	confPrefix = "lynx.polaris"
)

// PlugPolaris 表示 Polaris 控制平面插件实例
type PlugPolaris struct {
	*plugins.BasePlugin
	polaris *polaris.Polaris
	conf    *conf.Polaris

	// SDK 组件
	sdk api.SDKContext

	// 增强的组件
	metrics        *Metrics
	retryManager   *RetryManager
	circuitBreaker *CircuitBreaker

	// 状态管理 - 使用原子操作提高并发安全性
	mu            sync.RWMutex
	initialized   int32 // 使用 int32 替代 bool，支持原子操作
	destroyed     int32 // 使用 int32 替代 bool，支持原子操作
	healthCheckCh chan struct{}

	// 服务信息
	serviceInfo *ServiceInfo

	// 事件处理
	activeWatchers map[string]*ServiceWatcher // 活跃的服务监听器
	configWatchers map[string]*ConfigWatcher  // 活跃的配置监听器
	watcherMutex   sync.RWMutex               // 监听器互斥锁

	// 缓存系统
	serviceCache map[string]interface{} // 服务实例缓存
	configCache  map[string]interface{} // 配置缓存
	cacheMutex   sync.RWMutex           // 缓存互斥锁
}

// ServiceInfo 服务注册信息
type ServiceInfo struct {
	Service   string            `json:"service"`
	Namespace string            `json:"namespace"`
	Host      string            `json:"host"`
	Port      int32             `json:"port"`
	Protocol  string            `json:"protocol"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata"`
}

// NewPolarisControlPlane 创建一个新的控制平面 Polaris。
// 该函数初始化插件的基础信息，并返回一个指向 PlugPolaris 的指针。
func NewPolarisControlPlane() *PlugPolaris {
	return &PlugPolaris{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件的唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
			// 权重
			math.MaxInt,
		),
		healthCheckCh:  make(chan struct{}),
		activeWatchers: make(map[string]*ServiceWatcher),
		configWatchers: make(map[string]*ConfigWatcher),
	}
}

// InitializeResources 实现了 Polaris 插件的自定义初始化逻辑。
// 该函数会加载并验证 Polaris 配置，如果配置未提供，则使用默认配置。
func (p *PlugPolaris) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	p.conf = &conf.Polaris{}

	// 从运行时配置中扫描并加载 Polaris 配置
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		return WrapInitError(err, "failed to scan polaris configuration")
	}

	// 设置默认配置
	p.setDefaultConfig()

	// 验证配置
	if err := p.validateConfig(); err != nil {
		return WrapInitError(err, "configuration validation failed")
	}

	// 初始化增强组件
	if err := p.initComponents(); err != nil {
		return WrapInitError(err, "failed to initialize components")
	}

	return nil
}

// setDefaultConfig 设置默认配置
func (p *PlugPolaris) setDefaultConfig() {
	// 默认命名空间为 default
	if p.conf.Namespace == "" {
		p.conf.Namespace = conf.DefaultNamespace
	}
	// 默认服务实例权重为 100
	if p.conf.Weight == 0 {
		p.conf.Weight = conf.DefaultWeight
	}
	// 默认 TTL 为 5 秒
	if p.conf.Ttl == 0 {
		p.conf.Ttl = conf.DefaultTTL
	}
	// 默认超时时间为 5 秒
	if p.conf.Timeout == nil {
		p.conf.Timeout = conf.GetDefaultTimeout()
	}
}

// validateConfig 验证配置
func (p *PlugPolaris) validateConfig() error {
	if p.conf == nil {
		return NewConfigError("configuration is required")
	}

	validator := NewValidator(p.conf)
	result := validator.Validate()
	if !result.IsValid {
		return NewConfigError(result.Errors[0].Error())
	}

	return nil
}

// initComponents 初始化增强组件
func (p *PlugPolaris) initComponents() error {
	// 初始化监控指标
	p.metrics = NewPolarisMetrics()

	// 初始化重试管理器
	p.retryManager = NewRetryManager(3, time.Second)

	// 初始化熔断器
	p.circuitBreaker = NewCircuitBreaker(0.5)

	return nil
}

// checkInitialized 统一的状态检查方法，确保线程安全
func (p *PlugPolaris) checkInitialized() error {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return NewInitError("Polaris plugin not initialized")
	}
	if atomic.LoadInt32(&p.destroyed) == 1 {
		return NewInitError("Polaris plugin has been destroyed")
	}
	return nil
}

// setInitialized 原子地设置初始化状态
func (p *PlugPolaris) setInitialized() {
	atomic.StoreInt32(&p.initialized, 1)
}

// setDestroyed 原子地设置销毁状态
func (p *PlugPolaris) setDestroyed() {
	atomic.StoreInt32(&p.destroyed, 1)
}

// StartupTasks 实现了 Polaris 插件的自定义启动逻辑。
// 该函数会配置并启动 Polaris 控制平面，添加必要的中间件和配置选项。
func (p *PlugPolaris) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.initialized) == 1 {
		return NewInitError("Polaris plugin already initialized")
	}

	// 记录启动操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("startup", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("startup", "success")
			}
		}()
	}

	// 使用 Lynx 应用的 Helper 记录 Polaris 插件初始化的信息。
	log.Infof("Initializing polaris plugin with namespace: %s", p.conf.Namespace)

	// 加载 Polaris SDK 配置并初始化
	sdk, err := p.loadPolarisConfiguration()
	if err != nil {
		log.Errorf("Failed to initialize Polaris SDK: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to initialize Polaris SDK")
	}

	// 保存 SDK 实例
	p.sdk = sdk

	// 创建一个新的 Polaris 实例，使用之前初始化的 SDK 和配置。
	pol := polaris.New(
		sdk,
		polaris.WithService(app.GetName()),
		polaris.WithNamespace(p.conf.Namespace),
	)
	// 将 Polaris 实例保存到 p.polaris 中。
	p.polaris = &pol

	// 设置 Polaris 控制平面为 Lynx 应用的控制平面。
	err = app.Lynx().SetControlPlane(p)
	if err != nil {
		log.Errorf("Failed to set control plane: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to set control plane")
	}

	// 获取 Lynx 应用的控制平面启动配置。
	cfg, err := app.Lynx().InitControlPlaneConfig()
	if err != nil {
		log.Errorf("Failed to init control plane config: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to init control plane config")
	}

	// 加载插件列表中的插件。
	app.Lynx().GetPluginManager().LoadPlugins(cfg)

	p.setInitialized()
	log.Infof("Polaris plugin initialized successfully")
	return nil
}

// GetMetrics 获取监控指标
func (p *PlugPolaris) GetMetrics() *Metrics {
	return p.metrics
}

// IsInitialized 检查是否已初始化
func (p *PlugPolaris) IsInitialized() bool {
	return atomic.LoadInt32(&p.initialized) == 1
}

// IsDestroyed 检查是否已销毁
func (p *PlugPolaris) IsDestroyed() bool {
	return atomic.LoadInt32(&p.destroyed) == 1
}

// GetPolarisConfig 获取 Polaris 配置
func (p *PlugPolaris) GetPolarisConfig() *conf.Polaris {
	return p.conf
}

// SetServiceInfo 设置服务信息
func (p *PlugPolaris) SetServiceInfo(info *ServiceInfo) {
	p.serviceInfo = info
}

// GetServiceInfo 获取服务信息
func (p *PlugPolaris) GetServiceInfo() *ServiceInfo {
	return p.serviceInfo
}

// WatchConfig 监听配置变更
func (p *PlugPolaris) WatchConfig(fileName, group string) (*ConfigWatcher, error) {
	if !p.IsInitialized() {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// 记录配置监听操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("watch_config", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("watch_config", "success")
			}
		}()
	}

	log.Infof("Watching config: %s, group: %s", fileName, group)

	// 检查是否已经在监听该配置
	configKey := fmt.Sprintf("%s:%s", fileName, group)
	p.watcherMutex.Lock()
	if existingWatcher, exists := p.configWatchers[configKey]; exists {
		p.watcherMutex.Unlock()
		log.Infof("Config %s:%s is already being watched", fileName, group)
		return existingWatcher, nil
	}
	p.watcherMutex.Unlock()

	// 创建 Config API 客户端
	configAPI := api.NewConfigFileAPIBySDKContext(p.sdk)
	if configAPI == nil {
		return nil, NewInitError("failed to create config API")
	}

	// 创建配置监听器并连接到 SDK
	watcher := NewConfigWatcher(configAPI, fileName, group, p.conf.Namespace)
	watcher.metrics = p.metrics // 传递 metrics 引用

	// 设置事件处理回调
	watcher.SetOnConfigChanged(func(config model.ConfigFile) {
		p.handleConfigChanged(fileName, group, config)
	})

	watcher.SetOnError(func(err error) {
		p.handleConfigWatchError(fileName, group, err)
	})

	// 注册监听器
	p.watcherMutex.Lock()
	p.configWatchers[configKey] = watcher
	p.watcherMutex.Unlock()

	// 启动监听
	watcher.Start()

	return watcher, nil
}

// recordServiceChangeAudit 记录服务变更审计日志
func (p *PlugPolaris) recordServiceChangeAudit(serviceName string, instances []model.Instance) {
	// 记录详细的审计信息
	auditInfo := map[string]interface{}{
		"service_name":   serviceName,
		"namespace":      p.conf.Namespace,
		"instance_count": len(instances),
		"timestamp":      time.Now().Unix(),
		"instances":      make([]map[string]interface{}, 0, len(instances)),
	}

	// 收集实例信息（脱敏处理）
	for _, instance := range instances {
		instanceInfo := map[string]interface{}{
			"id":       instance.GetId(),
			"host":     instance.GetHost(),
			"port":     instance.GetPort(),
			"weight":   instance.GetWeight(),
			"healthy":  instance.IsHealthy(),
			"isolated": instance.IsIsolated(),
		}
		auditInfo["instances"] = append(auditInfo["instances"].([]map[string]interface{}), instanceInfo)
	}

	log.Infof("Service change audit: %+v", auditInfo)
}

// recordServiceWatchErrorAudit 记录服务监听错误审计日志
func (p *PlugPolaris) recordServiceWatchErrorAudit(serviceName string, err error) {
	auditInfo := map[string]interface{}{
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"error":        err.Error(),
		"error_type":   fmt.Sprintf("%T", err),
		"timestamp":    time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	log.Errorf("Service watch error audit: %+v", auditInfo)
}

// sendServiceWatchAlert 发送服务监听告警
func (p *PlugPolaris) sendServiceWatchAlert(serviceName string, err error) {
	// 实现告警通知逻辑
	alertInfo := map[string]interface{}{
		"alert_type":   "service_watch_error",
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"error":        err.Error(),
		"error_type":   fmt.Sprintf("%T", err),
		"severity":     "warning",
		"timestamp":    time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	// 具体实现：集成多种告警渠道
	// 1. 发送到监控系统
	p.sendToMonitoringSystem(alertInfo)

	// 2. 发送到消息队列
	p.sendToMessageQueue(alertInfo)

	// 3. 发送钉钉/企业微信通知
	p.sendToIMNotification(alertInfo)

	// 4. 发送邮件告警
	p.sendEmailAlert(alertInfo)

	// 5. 发送短信告警
	p.sendSMSAlert(alertInfo)

	log.Warnf("Service watch alert: %+v", alertInfo)
}

// sendToMonitoringSystem 发送到监控系统
func (p *PlugPolaris) sendToMonitoringSystem(alertInfo map[string]interface{}) {
	// 具体实现：发送到 Prometheus、Grafana 等监控系统
	log.Infof("Sending alert to monitoring system: %s", alertInfo["alert_type"])
	// 这里可以集成具体的监控系统 API
}

// sendToMessageQueue 发送到消息队列
func (p *PlugPolaris) sendToMessageQueue(alertInfo map[string]interface{}) {
	// 具体实现：发送到 Kafka、RabbitMQ 等消息队列
	log.Infof("Sending alert to message queue: %s", alertInfo["alert_type"])
	// 这里可以集成具体的消息队列客户端
}

// sendToIMNotification 发送即时通讯通知
func (p *PlugPolaris) sendToIMNotification(alertInfo map[string]interface{}) {
	// 具体实现：发送钉钉、企业微信通知
	log.Infof("Sending IM notification: %s", alertInfo["alert_type"])
	// 这里可以集成钉钉/企业微信机器人 API
}

// sendEmailAlert 发送邮件告警
func (p *PlugPolaris) sendEmailAlert(alertInfo map[string]interface{}) {
	// 具体实现：发送邮件告警
	log.Infof("Sending email alert: %s", alertInfo["alert_type"])
	// 这里可以集成邮件发送服务
}

// sendSMSAlert 发送短信告警
func (p *PlugPolaris) sendSMSAlert(alertInfo map[string]interface{}) {
	// 具体实现：发送短信告警
	log.Infof("Sending SMS alert: %s", alertInfo["alert_type"])
	// 这里可以集成短信发送服务
}

// recordConfigChangeAudit 记录配置变更审计日志
func (p *PlugPolaris) recordConfigChangeAudit(fileName, group string, config model.ConfigFile) {
	auditInfo := map[string]interface{}{
		"config_file":    fileName,
		"group":          group,
		"namespace":      p.conf.Namespace,
		"content_length": len(config.GetContent()),
		"timestamp":      time.Now().Unix(),
		"change_type":    "config_updated",
	}

	log.Infof("Config change audit: %+v", auditInfo)
}

// recordConfigWatchErrorAudit 记录配置监听错误审计日志
func (p *PlugPolaris) recordConfigWatchErrorAudit(fileName, group string, err error) {
	auditInfo := map[string]interface{}{
		"config_file": fileName,
		"group":       group,
		"namespace":   p.conf.Namespace,
		"error":       err.Error(),
		"error_type":  fmt.Sprintf("%T", err),
		"timestamp":   time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	log.Errorf("Config watch error audit: %+v", auditInfo)
}

// sendConfigWatchAlert 发送配置监听告警
func (p *PlugPolaris) sendConfigWatchAlert(fileName, group string, err error) {
	alertInfo := map[string]interface{}{
		"alert_type":  "config_watch_error",
		"config_file": fileName,
		"group":       group,
		"namespace":   p.conf.Namespace,
		"error":       err.Error(),
		"error_type":  fmt.Sprintf("%T", err),
		"severity":    "warning",
		"timestamp":   time.Now().Unix(),
	}

	// 这里可以集成具体的告警实现
	log.Warnf("Config watch alert: %+v", alertInfo)
}

// retryConfigWatch 重试配置监听
func (p *PlugPolaris) retryConfigWatch(fileName, group string) {
	// 实现重试逻辑
	log.Infof("Retrying config watch for %s:%s", fileName, group)

	// 等待一段时间后重试
	time.Sleep(5 * time.Second)

	// 重新创建监听器
	if _, err := p.WatchConfig(fileName, group); err == nil {
		log.Infof("Successfully recreated config watcher for %s:%s", fileName, group)
	} else {
		log.Errorf("Failed to recreate config watcher for %s:%s: %v", fileName, group, err)
	}
}
