package interfaces

// DependencyInjector defines the interface for dependency injection
type DependencyInjector interface {
	// RegisterGrpcServiceProvider registers a gRPC service provider
	RegisterGrpcServiceProvider(provider GrpcServiceProvider)
	
	// RegisterGrpcClientProvider registers a gRPC client provider
	RegisterGrpcClientProvider(provider GrpcClientProvider)
	
	// RegisterGrpcSubscribeProvider registers a gRPC subscribe provider
	RegisterGrpcSubscribeProvider(provider GrpcSubscribeProvider)
	
	// RegisterTLSConfigProvider registers a TLS config provider
	RegisterTLSConfigProvider(provider TLSConfigProvider)
	
	// RegisterLoggerProvider registers a logger provider
	RegisterLoggerProvider(provider LoggerProvider)
	
	// GetGrpcServiceProvider returns the registered gRPC service provider
	GetGrpcServiceProvider() GrpcServiceProvider
	
	// GetGrpcClientProvider returns the registered gRPC client provider
	GetGrpcClientProvider() GrpcClientProvider
	
	// GetGrpcSubscribeProvider returns the registered gRPC subscribe provider
	GetGrpcSubscribeProvider() GrpcSubscribeProvider
	
	// GetTLSConfigProvider returns the registered TLS config provider
	GetTLSConfigProvider() TLSConfigProvider
	
	// GetLoggerProvider returns the registered logger provider
	GetLoggerProvider() LoggerProvider
}

// DefaultDependencyInjector provides a default implementation of DependencyInjector
type DefaultDependencyInjector struct {
	grpcServiceProvider   GrpcServiceProvider
	grpcClientProvider    GrpcClientProvider
	grpcSubscribeProvider GrpcSubscribeProvider
	tlsConfigProvider     TLSConfigProvider
	loggerProvider        LoggerProvider
}

// NewDefaultDependencyInjector creates a new default dependency injector
func NewDefaultDependencyInjector() *DefaultDependencyInjector {
	return &DefaultDependencyInjector{}
}

func (d *DefaultDependencyInjector) RegisterGrpcServiceProvider(provider GrpcServiceProvider) {
	d.grpcServiceProvider = provider
}

func (d *DefaultDependencyInjector) RegisterGrpcClientProvider(provider GrpcClientProvider) {
	d.grpcClientProvider = provider
}

func (d *DefaultDependencyInjector) RegisterGrpcSubscribeProvider(provider GrpcSubscribeProvider) {
	d.grpcSubscribeProvider = provider
}

func (d *DefaultDependencyInjector) RegisterTLSConfigProvider(provider TLSConfigProvider) {
	d.tlsConfigProvider = provider
}

func (d *DefaultDependencyInjector) RegisterLoggerProvider(provider LoggerProvider) {
	d.loggerProvider = provider
}

func (d *DefaultDependencyInjector) GetGrpcServiceProvider() GrpcServiceProvider {
	return d.grpcServiceProvider
}

func (d *DefaultDependencyInjector) GetGrpcClientProvider() GrpcClientProvider {
	return d.grpcClientProvider
}

func (d *DefaultDependencyInjector) GetGrpcSubscribeProvider() GrpcSubscribeProvider {
	return d.grpcSubscribeProvider
}

func (d *DefaultDependencyInjector) GetTLSConfigProvider() TLSConfigProvider {
	return d.tlsConfigProvider
}

func (d *DefaultDependencyInjector) GetLoggerProvider() LoggerProvider {
	return d.loggerProvider
}
