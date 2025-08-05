package http

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestPerformanceConfig(t *testing.T) {
	// 创建 HTTP 插件实例
	service := NewServiceHttp()

	// 设置自定义性能配置
	service.idleTimeout = 120 * time.Second
	service.keepAliveTimeout = 60 * time.Second
	service.readHeaderTimeout = 30 * time.Second
	service.maxRequestSize = 20 * 1024 * 1024 // 20MB

	// 验证配置是否正确设置
	assert.Equal(t, 120*time.Second, service.idleTimeout)
	assert.Equal(t, 60*time.Second, service.keepAliveTimeout)
	assert.Equal(t, 30*time.Second, service.readHeaderTimeout)
	assert.Equal(t, int64(20*1024*1024), service.maxRequestSize)
}

func TestPerformanceDefaults(t *testing.T) {
	// 创建 HTTP 插件实例
	service := NewServiceHttp()

	// 调用默认配置设置
	service.initPerformanceDefaults()

	// 验证默认值
	assert.Equal(t, 60*time.Second, service.idleTimeout)
	assert.Equal(t, 30*time.Second, service.keepAliveTimeout)
	assert.Equal(t, 20*time.Second, service.readHeaderTimeout)
}

func TestSecurityDefaults(t *testing.T) {
	// 创建 HTTP 插件实例
	service := NewServiceHttp()

	// 调用安全默认配置设置
	service.initSecurityDefaults()

	// 验证默认值
	assert.Equal(t, int64(10*1024*1024), service.maxRequestSize) // 10MB
	assert.NotNil(t, service.rateLimiter)
	assert.Equal(t, rate.Limit(100), service.rateLimiter.Limit()) // 100 req/s
	assert.Equal(t, 200, service.rateLimiter.Burst())             // burst: 200
}

func TestGracefulShutdownDefaults(t *testing.T) {
	// 创建 HTTP 插件实例
	service := NewServiceHttp()

	// 调用优雅关闭默认配置设置
	service.initGracefulShutdownDefaults()

	// 验证默认值
	assert.Equal(t, 30*time.Second, service.shutdownTimeout)
}

func TestConfigurationValidation(t *testing.T) {
	// 创建 HTTP 插件实例
	service := NewServiceHttp()

	// 设置有效配置
	service.conf = &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Timeout: &durationpb.Duration{Seconds: 10},
	}
	service.maxRequestSize = 10 * 1024 * 1024
	service.idleTimeout = 60 * time.Second
	service.keepAliveTimeout = 30 * time.Second
	service.readHeaderTimeout = 20 * time.Second
	service.shutdownTimeout = 30 * time.Second

	// 验证配置
	err := service.validateConfig()
	assert.NoError(t, err)

	// 测试无效地址
	service.conf.Addr = "invalid-address"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid address format")

	// 测试无效端口
	service.conf.Addr = ":99999"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port number")

	// 测试无效网络协议
	service.conf.Addr = ":8080" // 恢复有效地址
	service.conf.Network = "invalid"
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid network protocol")

	// 测试负的请求大小
	service.conf.Network = "tcp" // 恢复有效网络
	service.maxRequestSize = -1
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max request size cannot be negative")

	// 测试过大的请求大小
	service.maxRequestSize = 200 * 1024 * 1024 // 200MB
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max request size cannot exceed 100MB")

	// 测试无效的超时时间
	service.maxRequestSize = 10 * 1024 * 1024 // 恢复有效大小
	service.conf.Timeout = &durationpb.Duration{Seconds: -1}
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be positive")

	// 测试过长的超时时间
	service.conf.Timeout = &durationpb.Duration{Seconds: 400} // 6.67 minutes
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout cannot exceed 5 minutes")

	// 测试无效的性能配置
	service.conf.Timeout = &durationpb.Duration{Seconds: 10} // 恢复有效超时
	service.idleTimeout = -1 * time.Second
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idle timeout cannot be negative")

	// 测试过长的空闲超时
	service.idleTimeout = 700 * time.Second // 11.67 minutes
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idle timeout cannot exceed 10 minutes")

	// 测试无效的限流配置
	service.idleTimeout = 60 * time.Second        // 恢复有效空闲超时
	service.rateLimiter = rate.NewLimiter(0, 100) // 0 req/s
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit must be positive")

	// 测试过高的限流
	service.rateLimiter = rate.NewLimiter(15000, 100) // 15k req/s
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit cannot exceed 10,000 requests per second")
}
