package polaris

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
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

	// 状态管理
	mu            sync.RWMutex
	initialized   bool
	destroyed     bool
	healthCheckCh chan struct{}

	// 服务信息
	serviceInfo *ServiceInfo

	// 事件处理
	activeWatchers map[string]*ServiceWatcher // 活跃的服务监听器
	configWatchers map[string]*ConfigWatcher  // 活跃的配置监听器
	watcherMutex   sync.RWMutex               // 监听器互斥锁
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

// StartupTasks 实现了 Polaris 插件的自定义启动逻辑。
// 该函数会配置并启动 Polaris 控制平面，添加必要的中间件和配置选项。
func (p *PlugPolaris) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
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

	// 初始化 Polaris SDK 上下文。
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	// 如果初始化失败，记录错误信息并返回错误。
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

	p.initialized = true
	log.Infof("Polaris plugin initialized successfully")
	return nil
}

// CleanupTasks 清理任务
func (p *PlugPolaris) CleanupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	if p.destroyed {
		return nil
	}

	// 记录清理操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("cleanup", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("cleanup", "success")
			}
		}()
	}

	log.Infof("Destroying Polaris plugin")

	// 停止健康检查
	if p.healthCheckCh != nil {
		close(p.healthCheckCh)
	}

	// 销毁 Polaris 实例
	if p.polaris != nil {
		// 这里可以添加 Polaris 实例的清理逻辑
	}

	p.destroyed = true
	log.Infof("Polaris plugin destroyed successfully")
	return nil
}

// CheckHealth 健康检查
func (p *PlugPolaris) CheckHealth() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return NewInitError("Polaris plugin not initialized")
	}

	if p.destroyed {
		return NewInitError("Polaris plugin has been destroyed")
	}

	// 检查 Polaris 实例
	if p.polaris == nil {
		return NewInitError("Polaris instance is nil")
	}

	// 记录健康检查指标
	if p.metrics != nil {
		p.metrics.RecordHealthCheck("polaris", "success")
	}

	return nil
}

// GetMetrics 获取监控指标
func (p *PlugPolaris) GetMetrics() *Metrics {
	return p.metrics
}

// IsInitialized 检查是否已初始化
func (p *PlugPolaris) IsInitialized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}

// IsDestroyed 检查是否已销毁
func (p *PlugPolaris) IsDestroyed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.destroyed
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

// GetServiceInstances 获取服务实例
func (p *PlugPolaris) GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if !p.initialized {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// 记录服务发现操作指标
	if p.metrics != nil {
		p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "success")
			}
		}()
	}

	log.Infof("Getting service instances for: %s", serviceName)

	// 使用熔断器和重试机制执行操作
	var instances []model.Instance
	var lastErr error

	// 使用熔断器包装重试操作
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 创建 Consumer API 客户端
			consumerAPI := api.NewConsumerAPIByContext(p.sdk)
			if consumerAPI == nil {
				return NewInitError("failed to create consumer API")
			}

			// 构建服务发现请求
			req := &api.GetInstancesRequest{
				GetInstancesRequest: model.GetInstancesRequest{
					Service:   serviceName,
					Namespace: p.conf.Namespace,
				},
			}

			// 调用 SDK API 获取服务实例
			resp, err := consumerAPI.GetInstances(req)
			if err != nil {
				lastErr = err
				return err
			}

			instances = resp.Instances
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to get instances for service %s after retries: %v", serviceName, err)
		if p.metrics != nil {
			p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "error")
		}

		return nil, WrapServiceError(lastErr, ErrCodeServiceUnavailable, "failed to get service instances")
	}

	log.Infof("Successfully got %d instances for service %s", len(instances), serviceName)
	return instances, nil
}

// WatchService 监听服务变更
func (p *PlugPolaris) WatchService(serviceName string) (*ServiceWatcher, error) {
	if !p.initialized {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// 记录服务监听操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("watch_service", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("watch_service", "success")
			}
		}()
	}

	log.Infof("Watching service: %s", serviceName)

	// 检查是否已经在监听该服务
	p.watcherMutex.Lock()
	if existingWatcher, exists := p.activeWatchers[serviceName]; exists {
		p.watcherMutex.Unlock()
		log.Infof("Service %s is already being watched", serviceName)
		return existingWatcher, nil
	}
	p.watcherMutex.Unlock()

	// 创建 Consumer API 客户端
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		return nil, NewInitError("failed to create consumer API")
	}

	// 创建服务监听器并连接到 SDK
	watcher := NewServiceWatcher(consumerAPI, serviceName, p.conf.Namespace)
	watcher.metrics = p.metrics // 传递 metrics 引用

	// 设置事件处理回调
	watcher.SetOnInstancesChanged(func(instances []model.Instance) {
		p.handleServiceInstancesChanged(serviceName, instances)
	})

	watcher.SetOnError(func(err error) {
		p.handleServiceWatchError(serviceName, err)
	})

	// 注册监听器
	p.watcherMutex.Lock()
	p.activeWatchers[serviceName] = watcher
	p.watcherMutex.Unlock()

	// 启动监听
	watcher.Start()

	return watcher, nil
}

// GetConfigValue 获取配置值
func (p *PlugPolaris) GetConfigValue(fileName, group string) (string, error) {
	if !p.initialized {
		return "", NewInitError("Polaris plugin not initialized")
	}

	// 记录配置操作指标
	if p.metrics != nil {
		p.metrics.RecordConfigOperation("get", fileName, group, "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordConfigOperation("get", fileName, group, "success")
			}
		}()
	}

	log.Infof("Getting config: %s, group: %s", fileName, group)

	// 创建 Config API 客户端
	configAPI := api.NewConfigFileAPIBySDKContext(p.sdk)
	if configAPI == nil {
		return "", NewInitError("failed to create config API")
	}

	// 调用 SDK API 获取配置
	config, err := configAPI.GetConfigFile(p.conf.Namespace, group, fileName)
	if err != nil {
		log.Errorf("Failed to get config %s:%s: %v", fileName, group, err)
		if p.metrics != nil {
			p.metrics.RecordConfigOperation("get", fileName, group, "error")
		}
		return "", WrapServiceError(err, ErrCodeConfigGetFailed, "failed to get config value")
	}

	// 检查配置是否存在
	if config == nil {
		log.Warnf("Config %s:%s not found", fileName, group)
		return "", NewServiceError(ErrCodeConfigNotFound, "config not found")
	}

	// 获取配置内容
	content := config.GetContent()
	log.Infof("Successfully got config %s:%s, content length: %d", fileName, group, len(content))
	return content, nil
}

// WatchConfig 监听配置变更
func (p *PlugPolaris) WatchConfig(fileName, group string) (*ConfigWatcher, error) {
	if !p.initialized {
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

// CheckRateLimit 检查限流
func (p *PlugPolaris) CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if !p.initialized {
		return false, NewInitError("Polaris plugin not initialized")
	}

	// 记录限流检查操作指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("check_rate_limit", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("check_rate_limit", "success")
			}
		}()
	}

	log.Infof("Checking rate limit for service: %s", serviceName)

	// 创建 Limit API 客户端
	limitAPI := api.NewLimitAPIByContext(p.sdk)
	if limitAPI == nil {
		return false, NewInitError("failed to create limit API")
	}

	// 构建限流请求
	quotaReq := api.NewQuotaRequest()
	quotaReq.SetService(serviceName)
	quotaReq.SetNamespace(p.conf.Namespace)

	// 设置标签
	if labels != nil {
		for key, value := range labels {
			quotaReq.AddArgument(model.BuildQueryArgument(key, value))
		}
	}

	// 调用 SDK API 检查限流
	future, err := limitAPI.GetQuota(quotaReq)
	if err != nil {
		log.Errorf("Failed to check rate limit for service %s: %v", serviceName, err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("check_rate_limit", "error")
		}
		return false, WrapServiceError(err, ErrCodeRateLimitFailed, "failed to check rate limit")
	}

	// 获取限流结果
	result := future.Get()
	if result == nil {
		log.Errorf("Rate limit result is nil for service %s", serviceName)
		return false, NewServiceError(ErrCodeRateLimitFailed, "rate limit result is nil")
	}

	// 检查是否被限流
	if result.Code == model.QuotaResultOk {
		log.Debugf("Rate limit check passed for service %s", serviceName)
		return true, nil
	} else {
		log.Warnf("Rate limit exceeded for service %s, code: %d", serviceName, result.Code)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("rate_limit_exceeded", "success")
		}
		return false, NewServiceError(ErrCodeRateLimitExceeded, "rate limit exceeded")
	}
}

// HTTPRateLimit 和 GRPCRateLimit 方法现在在 limiter.go 中实现

// NewServiceRegistry 实现 ServiceRegistry 接口
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	if !p.initialized {
		log.Warnf("Polaris plugin not initialized, returning nil registrar")
		return nil
	}

	// 创建 Provider API 客户端
	providerAPI := api.NewProviderAPIByContext(p.sdk)
	if providerAPI == nil {
		log.Errorf("Failed to create provider API")
		return nil
	}

	// 返回基于 Polaris 的服务注册器
	return NewPolarisRegistrar(providerAPI, p.conf.Namespace)
}

// NewServiceDiscovery 实现 ServiceRegistry 接口
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	if !p.initialized {
		log.Warnf("Polaris plugin not initialized, returning nil discovery")
		return nil
	}

	// 创建 Consumer API 客户端
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		log.Errorf("Failed to create consumer API")
		return nil
	}

	// 返回基于 Polaris 的服务发现客户端
	return NewPolarisDiscovery(consumerAPI, p.conf.Namespace)
}

// 事件处理方法

// handleServiceInstancesChanged 处理服务实例变更事件
func (p *PlugPolaris) handleServiceInstancesChanged(serviceName string, instances []model.Instance) {
	log.Infof("Service %s instances changed: %d instances", serviceName, len(instances))

	// 记录服务发现指标
	if p.metrics != nil {
		p.metrics.RecordServiceDiscovery(serviceName, p.conf.Namespace, "changed")
	}

	// 这里可以添加更多的业务逻辑，比如：
	// 1. 更新本地缓存
	// 2. 通知其他组件
	// 3. 触发负载均衡更新
	// 4. 记录审计日志
	// 5. 发送通知

	// 示例：更新服务实例缓存
	p.updateServiceInstanceCache(serviceName, instances)

	// 示例：通知相关组件
	p.notifyServiceChange(serviceName, instances)
}

// handleServiceWatchError 处理服务监听错误事件
func (p *PlugPolaris) handleServiceWatchError(serviceName string, err error) {
	log.Errorf("Service %s watch error: %v", serviceName, err)

	// 记录错误指标
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("service_watch_error", "error")
	}

	// 这里可以添加错误处理逻辑，比如：
	// 1. 重试机制
	// 2. 降级处理
	// 3. 告警通知
	// 4. 错误恢复

	// 示例：尝试重新连接
	go p.retryServiceWatch(serviceName)
}

// updateServiceInstanceCache 更新服务实例缓存
func (p *PlugPolaris) updateServiceInstanceCache(serviceName string, instances []model.Instance) {
	// 这里可以实现本地缓存更新逻辑
	// 比如使用内存缓存或分布式缓存
	log.Debugf("Updating service instance cache for %s: %d instances", serviceName, len(instances))
}

// notifyServiceChange 通知服务变更
func (p *PlugPolaris) notifyServiceChange(serviceName string, instances []model.Instance) {
	// 这里可以实现通知逻辑
	// 比如通过消息队列、Webhook、事件总线等
	log.Debugf("Notifying service change for %s: %d instances", serviceName, len(instances))
}

// retryServiceWatch 重试服务监听
func (p *PlugPolaris) retryServiceWatch(serviceName string) {
	// 实现重试逻辑
	log.Infof("Retrying service watch for %s", serviceName)

	// 等待一段时间后重试
	time.Sleep(5 * time.Second)

	// 重新创建监听器
	if _, err := p.WatchService(serviceName); err == nil {
		log.Infof("Successfully recreated service watcher for %s", serviceName)
	} else {
		log.Errorf("Failed to recreate service watcher for %s: %v", serviceName, err)
	}
}

// handleConfigChanged 处理配置变更事件
func (p *PlugPolaris) handleConfigChanged(fileName, group string, config model.ConfigFile) {
	log.Infof("Config %s:%s changed", fileName, group)

	// 记录配置变更指标
	if p.metrics != nil {
		p.metrics.RecordConfigChange(fileName, group)
	}

	// 这里可以添加更多的业务逻辑，比如：
	// 1. 更新配置缓存
	// 2. 重新加载应用配置
	// 3. 通知配置变更
	// 4. 触发配置热更新
	// 5. 记录配置审计日志

	// 示例：更新配置缓存
	p.updateConfigCache(fileName, group, config)

	// 示例：通知配置变更
	p.notifyConfigChange(fileName, group, config)

	// 示例：触发配置热更新
	p.triggerConfigReload(fileName, group, config)
}

// handleConfigWatchError 处理配置监听错误事件
func (p *PlugPolaris) handleConfigWatchError(fileName, group string, err error) {
	log.Errorf("Config %s:%s watch error: %v", fileName, group, err)

	// 记录错误指标
	if p.metrics != nil {
		p.metrics.RecordConfigOperation("watch_error", fileName, group, "error")
	}

	// 这里可以添加错误处理逻辑，比如：
	// 1. 重试机制
	// 2. 降级处理
	// 3. 告警通知
	// 4. 错误恢复

	// 示例：尝试重新连接
	go p.retryConfigWatch(fileName, group)
}

// updateConfigCache 更新配置缓存
func (p *PlugPolaris) updateConfigCache(fileName, group string, config model.ConfigFile) {
	// 这里可以实现配置缓存更新逻辑
	log.Debugf("Updating config cache for %s:%s", fileName, group)
}

// notifyConfigChange 通知配置变更
func (p *PlugPolaris) notifyConfigChange(fileName, group string, config model.ConfigFile) {
	// 这里可以实现通知逻辑
	// 比如通过消息队列、Webhook、事件总线等
	log.Debugf("Notifying config change for %s:%s", fileName, group)
}

// triggerConfigReload 触发配置重载
func (p *PlugPolaris) triggerConfigReload(fileName, group string, config model.ConfigFile) {
	// 这里可以实现配置热更新逻辑
	log.Debugf("Triggering config reload for %s:%s", fileName, group)
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
