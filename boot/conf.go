package boot

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

// LoadLocalBootstrapConfig 从本地文件或目录加载引导配置。
// 它从 flagConf 指定的路径读取配置，并初始化应用程序的配置状态。
//
// 返回值:
//   - error: 配置加载过程中发生的任何错误
func (b *Boot) LoadLocalBootstrapConfig() error {
	// 检查 Boot 实例是否为 nil
	if b == nil {
		return fmt.Errorf("boot instance is nil")
	}

	// 检查配置路径是否为空
	if flagConf == "" {
		return fmt.Errorf("configuration path is empty")
	}

	// 记录尝试加载配置的日志
	log.Infof("loading local bootstrap configuration from: %s", flagConf)

	// 从本地文件创建配置源
	source := file.NewSource(flagConf)
	// 检查配置源是否创建成功
	if source == nil {
		return fmt.Errorf("failed to create configuration source from: %s", flagConf)
	}

	// 创建新的配置实例
	cfg := config.New(
		// 指定配置源
		config.WithSource(source),
	)
	// 检查配置实例是否创建成功
	if cfg == nil {
		return fmt.Errorf("failed to create configuration instance")
	}

	// 加载配置
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 在设置清理操作之前存储配置
	// 这样可以确保如果清理设置失败，不会丢失配置引用
	b.conf = cfg

	// 设置配置清理操作
	if err := b.setupConfigCleanup(cfg); err != nil {
		// 记录清理设置失败的日志，但继续执行
		// 配置仍然有效且可用
		log.Warnf("Failed to setup configuration cleanup: %v", err)
	}

	return nil
}

// setupConfigCleanup 确保在配置资源不再需要时进行正确的清理。
func (b *Boot) setupConfigCleanup(cfg config.Config) error {
	// 检查配置实例是否为 nil
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil")
	}

	// 设置延迟清理函数
	b.cleanup = func() {
		// 关闭配置资源
		if err := cfg.Close(); err != nil {
			// 记录关闭配置失败的日志
			log.Errorf("failed to close configuration: %v", err)
		}
	}

	return nil
}
