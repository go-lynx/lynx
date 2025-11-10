package polaris

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// RegistryAdapter Polaris Registry adapter
// Responsibility: implements Kratos registry interface, provides service registration and discovery functionality

// NewServiceRegistry implements ServiceRegistry interface
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	if err := p.checkInitialized(); err != nil {
		log.Warnf("Polaris plugin not initialized, returning nil registrar: %v", err)
		return nil
	}

	// Create Provider API client
	providerAPI := api.NewProviderAPIByContext(p.sdk)
	if providerAPI == nil {
		log.Errorf("Failed to create provider API")
		return nil
	}

	// Return Polaris-based service registrar
	return NewPolarisRegistrar(providerAPI, p.conf.Namespace)
}

// NewServiceDiscovery implements ServiceRegistry interface
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	if err := p.checkInitialized(); err != nil {
		log.Warnf("Polaris plugin not initialized, returning nil discovery: %v", err)
		return nil
	}

	// Create Consumer API client
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		log.Errorf("Failed to create consumer API")
		return nil
	}

	// Return Polaris-based service discovery client with configurable watch interval and retry policy
	return NewPolarisDiscovery(consumerAPI, p.conf.Namespace, p.conf)
}

// parseEndpoints parses endpoint information
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

// PolarisRegistrar Polaris-based service registrar
// Implements Kratos registry.Registrar interface
type PolarisRegistrar struct {
	provider  api.ProviderAPI
	namespace string
	instances map[string]*registry.ServiceInstance
	mu        sync.RWMutex
}

// NewPolarisRegistrar creates new Polaris registrar
func NewPolarisRegistrar(provider api.ProviderAPI, namespace string) *PolarisRegistrar {
	return &PolarisRegistrar{
		provider:  provider,
		namespace: namespace,
		instances: make(map[string]*registry.ServiceInstance),
	}
}

// Register registers service instance
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

// Deregister deregisters service instance
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

// GetService gets service information (implements Discovery interface)
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

// Watch watches service changes (implements Discovery interface)
func (r *PolarisRegistrar) Watch(ctx context.Context, name string) (registry.Watcher, error) {
	return &PolarisWatcher{
		ctx:  ctx,
		name: name,
	}, nil
}

// PolarisDiscovery Polaris-based service discovery client
// Implements Kratos registry.Discovery interface
type PolarisDiscovery struct {
	consumer       api.ConsumerAPI
	namespace      string
	watchInterval  time.Duration
	enableRetry    bool
	maxRetryTimes  int
	baseRetry      time.Duration
}

// NewPolarisDiscovery creates new Polaris discovery client
func NewPolarisDiscovery(consumer api.ConsumerAPI, namespace string, cfg *conf.Polaris) *PolarisDiscovery {
	pd := &PolarisDiscovery{
		consumer:  consumer,
		namespace: namespace,
	}
	// Configure watch interval and retry policy from cfg (with sane defaults)
	if cfg != nil {
		if cfg.HealthCheckInterval != nil && cfg.HealthCheckInterval.AsDuration() > 0 {
			pd.watchInterval = cfg.HealthCheckInterval.AsDuration()
		}
		pd.enableRetry = cfg.EnableRetry
		if cfg.MaxRetryTimes > 0 {
			pd.maxRetryTimes = int(cfg.MaxRetryTimes)
		} else {
			pd.maxRetryTimes = 3
		}
		if cfg.RetryInterval != nil && cfg.RetryInterval.AsDuration() > 0 {
			pd.baseRetry = cfg.RetryInterval.AsDuration()
		} else {
			pd.baseRetry = 2 * time.Second
		}
	} else {
		pd.watchInterval = 5 * time.Second
		pd.enableRetry = true
		pd.maxRetryTimes = 3
		pd.baseRetry = 2 * time.Second
	}
	if pd.watchInterval <= 0 {
		pd.watchInterval = 5 * time.Second
	}
	return pd
}

// GetService gets service instance list
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

// Watch watches service changes
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
	// create child context to allow Stop() cancel
	cctx, cancel := context.WithCancel(ctx)
	return &PolarisWatcher{
		ctx:          cctx,
		cancel:       cancel,
		name:         name,
		response:     resp,
		consumer:     d.consumer,
		namespace:    d.namespace,
		pollInterval: d.watchInterval,
		enableRetry:  d.enableRetry,
		maxRetries:   d.maxRetryTimes,
		baseRetry:    d.baseRetry,
	}, nil
}

// PolarisWatcher Polaris service watcher
// Implements Kratos registry.Watcher interface
type PolarisWatcher struct {
	ctx          context.Context
	cancel       context.CancelFunc
	name         string
	response     *model.WatchServiceResponse
	consumer     api.ConsumerAPI
	namespace    string
	lastKey      string
	pollInterval time.Duration
	enableRetry  bool
	maxRetries   int
	baseRetry    time.Duration
}

// Next gets next service change event
func (w *PolarisWatcher) Next() ([]*registry.ServiceInstance, error) {
	// Polling fallback: use configurable interval; apply exponential backoff with logging on errors
	interval := w.pollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	attempt := 0

	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case <-ticker.C:
			if w.consumer == nil {
				log.Warnf("polaris watcher consumer is nil, service=%s", w.name)
				return []*registry.ServiceInstance{}, nil
			}
			req := &api.GetInstancesRequest{
				GetInstancesRequest: model.GetInstancesRequest{
					Service:   w.name,
					Namespace: w.namespace,
				},
			}
			resp, err := w.consumer.GetInstances(req)
			if err != nil {
				if w.enableRetry {
					// Exponential backoff capped at 30s
					attempt++
					backoff := w.baseRetry
					if backoff <= 0 {
						backoff = 2 * time.Second
					}
					for i := 1; i < attempt; i++ {
						backoff *= 2
						if backoff > 30*time.Second {
							backoff = 30 * time.Second
							break
						}
					}
					if attempt > w.maxRetries && w.maxRetries > 0 {
						log.Errorf("polaris watcher retries exceeded: service=%s, err=%v", w.name, err)
						return nil, fmt.Errorf("watch get instances failed after retries: %w", err)
					}
					log.Warnf("polaris watcher get instances failed, retrying in %s: service=%s, attempt=%d, err=%v", backoff, w.name, attempt, err)
					select {
					case <-w.ctx.Done():
						return nil, w.ctx.Err()
					case <-time.After(backoff):
						continue
					}
				}
				return nil, fmt.Errorf("failed to get service instances for %s: %w", w.name, err)
			}
			attempt = 0 // reset after success

			// Build current key
			var keyBuilder strings.Builder
			for _, inst := range resp.Instances {
				keyBuilder.WriteString(inst.GetId())
				keyBuilder.WriteString("|")
				keyBuilder.WriteString(inst.GetHost())
				keyBuilder.WriteString(":")
				keyBuilder.WriteString(fmt.Sprintf("%d", inst.GetPort()))
				keyBuilder.WriteString("|")
				keyBuilder.WriteString(inst.GetVersion())
				keyBuilder.WriteString(";")
			}
			curKey := keyBuilder.String()
			if curKey != w.lastKey {
				w.lastKey = curKey
				// Convert to registry.ServiceInstance
				var instances []*registry.ServiceInstance
				for _, instance := range resp.Instances {
					endpoint := fmt.Sprintf("%s://%s:%d", instance.GetProtocol(), instance.GetHost(), instance.GetPort())
					instances = append(instances, &registry.ServiceInstance{
						ID:        instance.GetId(),
						Name:      w.name,
						Version:   instance.GetVersion(),
						Metadata:  instance.GetMetadata(),
						Endpoints: []string{endpoint},
					})
				}
				log.Infof("polaris watcher detected change: service=%s, instances=%d", w.name, len(instances))
				return instances, nil
			}
		}
	}
}

// Stop stops watching
func (w *PolarisWatcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}
