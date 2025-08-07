package polaris

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// RegistryAdapter Polaris Registry 适配器
// 职责：实现 Kratos registry 接口，提供服务注册发现功能

// parseEndpoints 解析端点信息
func parseEndpoints(endpoints []string) (host string, port int, protocol string) {
	if len(endpoints) == 0 {
		return "localhost", 8080, "http"
	}

	endpoint := endpoints[0]

	if strings.HasPrefix(endpoint, "http://") {
		protocol = "http"
		endpoint = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		protocol = "https"
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "grpc://") {
		protocol = "grpc"
		endpoint = strings.TrimPrefix(endpoint, "grpc://")
	} else {
		protocol = "http"
	}

	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		host = parts[0]
		if len(parts) > 1 {
			portStr := strings.Split(parts[1], "?")[0]
			if p, err := strconv.Atoi(portStr); err == nil {
				port = p
			} else {
				port = 8080
			}
		} else {
			port = 8080
		}
	} else {
		host = endpoint
		port = 8080
	}

	return host, port, protocol
}

// PolarisRegistrar 基于 Polaris 的服务注册器
// 实现 Kratos registry.Registrar 接口
type PolarisRegistrar struct {
	provider  api.ProviderAPI
	namespace string
	instances map[string]*registry.ServiceInstance
	mu        sync.RWMutex
}

// NewPolarisRegistrar 创建新的 Polaris 注册器
func NewPolarisRegistrar(provider api.ProviderAPI, namespace string) *PolarisRegistrar {
	return &PolarisRegistrar{
		provider:  provider,
		namespace: namespace,
		instances: make(map[string]*registry.ServiceInstance),
	}
}

// Register 注册服务实例
func (r *PolarisRegistrar) Register(ctx context.Context, service *registry.ServiceInstance) error {
	if service == nil {
		return fmt.Errorf("service instance is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	host, port, protocol := parseEndpoints(service.Endpoints)

	req := &api.InstanceRegisterRequest{
		InstanceRegisterRequest: model.InstanceRegisterRequest{
			Service:   service.Name,
			Namespace: r.namespace,
			Host:      host,
			Port:      port,
			Protocol:  &protocol,
			Version:   &service.Version,
			Metadata:  service.Metadata,
			Weight:    &[]int{100}[0],
			Healthy:   &[]bool{true}[0],
			Isolate:   &[]bool{false}[0],
		},
	}

	_, err := r.provider.Register(req)
	if err != nil {
		return fmt.Errorf("failed to register service %s: %w", service.Name, err)
	}

	instanceKey := fmt.Sprintf("%s:%s:%d", service.Name, host, port)
	r.instances[instanceKey] = service

	log.Infof("Successfully registered service %s at %s:%d", service.Name, host, port)
	return nil
}

// Deregister 注销服务实例
func (r *PolarisRegistrar) Deregister(ctx context.Context, service *registry.ServiceInstance) error {
	if service == nil {
		return fmt.Errorf("service instance is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	host, port, _ := parseEndpoints(service.Endpoints)

	req := &api.InstanceDeRegisterRequest{
		InstanceDeRegisterRequest: model.InstanceDeRegisterRequest{
			Service:   service.Name,
			Namespace: r.namespace,
			Host:      host,
			Port:      port,
		},
	}

	err := r.provider.Deregister(req)
	if err != nil {
		return fmt.Errorf("failed to deregister service %s: %w", service.Name, err)
	}

	instanceKey := fmt.Sprintf("%s:%s:%d", service.Name, host, port)
	delete(r.instances, instanceKey)

	log.Infof("Successfully deregistered service %s at %s:%d", service.Name, host, port)
	return nil
}

// GetService 获取服务信息（实现 Discovery 接口）
func (r *PolarisRegistrar) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var instances []*registry.ServiceInstance
	for _, instance := range r.instances {
		if instance.Name == name {
			instances = append(instances, instance)
		}
	}

	return instances, nil
}

// Watch 监听服务变更（实现 Discovery 接口）
func (r *PolarisRegistrar) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return &PolarisWatcher{
		ctx:  ctx,
		name: name,
	}, nil
}

// PolarisDiscovery 基于 Polaris 的服务发现客户端
// 实现 Kratos registry.Discovery 接口
type PolarisDiscovery struct {
	consumer  api.ConsumerAPI
	namespace string
}

// NewPolarisDiscovery 创建新的 Polaris 发现客户端
func NewPolarisDiscovery(consumer api.ConsumerAPI, namespace string) *PolarisDiscovery {
	return &PolarisDiscovery{
		consumer:  consumer,
		namespace: namespace,
	}
}

// GetService 获取服务实例列表
func (d *PolarisDiscovery) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	req := &api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   name,
			Namespace: d.namespace,
		},
	}

	resp, err := d.consumer.GetInstances(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get service instances for %s: %w", name, err)
	}

	var instances []*registry.ServiceInstance
	for _, instance := range resp.Instances {
		endpoint := fmt.Sprintf("%s://%s:%d", instance.GetProtocol(), instance.GetHost(), instance.GetPort())

		instances = append(instances, &registry.ServiceInstance{
			ID:        instance.GetId(),
			Name:      name,
			Version:   instance.GetVersion(),
			Metadata:  instance.GetMetadata(),
			Endpoints: []string{endpoint},
		})
	}

	return instances, nil
}

// Watch 监听服务变更
func (d *PolarisDiscovery) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	req := &api.WatchServiceRequest{
		WatchServiceRequest: model.WatchServiceRequest{
			Key: model.ServiceKey{
				Service:   name,
				Namespace: d.namespace,
			},
		},
	}

	resp, err := d.consumer.WatchService(req)
	if err != nil {
		return nil, fmt.Errorf("failed to watch service %s: %w", name, err)
	}

	return &PolarisWatcher{
		ctx:      ctx,
		name:     name,
		response: resp,
	}, nil
}

// PolarisWatcher Polaris 服务监听器
// 实现 Kratos registry.Watcher 接口
type PolarisWatcher struct {
	ctx      context.Context
	name     string
	response *model.WatchServiceResponse
}

// Next 获取下一个服务变更事件
func (w *PolarisWatcher) Next() ([]*registry.ServiceInstance, error) {
	if w.response == nil {
		return []*registry.ServiceInstance{}, nil
	}
	return []*registry.ServiceInstance{}, nil
}

// Stop 停止监听
func (w *PolarisWatcher) Stop() error {
	return nil
}
