package http

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, float64(100), service.rateLimiter.Limit()) // 100 req/s
	assert.Equal(t, 200, service.rateLimiter.Burst())          // burst: 200
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

	// 测试负的请求大小
	service.conf.Addr = ":8080" // 恢复有效地址
	service.maxRequestSize = -1
	err = service.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max request size cannot be negative")
}
