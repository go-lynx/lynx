package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
)

// InitializeResources 实现 Redis 插件的自定义初始化逻辑
// 从运行时配置中扫描并加载 Redis 配置，若配置未提供则使用默认配置
// 参数 rt 为运行时环境
// 返回错误信息，如果配置加载失败则返回相应错误
func (r *PlugRedis) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	r.conf = &conf.Redis{}

	// 从运行时配置中扫描并加载 Redis 配置
	err := rt.GetConfig().Value(confPrefix).Scan(r.conf)
	if err != nil {
		return err
	}

	// 验证配置并设置默认值
	if err := ValidateAndSetDefaults(r.conf); err != nil {
		return fmt.Errorf("redis configuration validation failed: %w", err)
	}

	return nil
}

// StartupTasks 启动 Redis 客户端并进行健康检查
// 返回错误信息，如果启动或健康检查失败则返回相应错误
func (r *PlugRedis) StartupTasks() error {
	// 记录启动 Redis 客户端日志
	log.Infof("starting redis client")

	// 启动计数
	redisStartupTotal.Inc()

	// 创建 Redis 通用客户端（支持单机/集群/哨兵）
	r.rdb = redis.NewUniversalClient(r.buildUniversalOptions())

	// 注册命令级指标 Hook
	r.rdb.AddHook(metricsHook{})

	// 启动时做一次快速健康检查（短超时）
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	start := time.Now()
	_, err := r.rdb.Ping(pingCtx).Result()
	cancel()
	if err != nil {
		// 启动失败需回滚资源
		_ = r.rdb.Close()
		redisStartupFailedTotal.Inc()
		return err
	}
	latency := time.Since(start)
	redisPingLatency.Observe(latency.Seconds())
	// 判定模式（单机/集群/哨兵）
	mode := r.detectMode()
	log.Infof("redis client successfully started, mode=%s, addrs=%v, ping_latency=%s", mode, r.currentAddrs(), latency)

	// 在启动阶段做一次增强检查
	r.enhancedReadinessCheck(mode)

	// 启动池统计采集器
	r.startPoolStatsCollector()
	// 启动信息采集器
	r.startInfoCollector(mode)
	return nil
}

// CleanupTasks 关闭 Redis 客户端
// 返回错误信息，如果关闭客户端失败则返回相应错误
func (r *PlugRedis) CleanupTasks() error {
	// 若 Redis 客户端未初始化，直接返回 nil
	if r.rdb == nil {
		return nil
	}
	// 停止采集器
	if r.statsQuit != nil {
		close(r.statsQuit)
		r.statsWG.Wait()
	}
	// 关闭 Redis 客户端
	if err := r.rdb.Close(); err != nil {
		// 返回带插件信息的错误
		return plugins.NewPluginError(r.ID(), "Stop", "Failed to stop Redis client", err)
	}
	return nil
}

// Configure 允许在运行时更新 Redis 服务器的配置
// 参数 c 应为指向 conf.Redis 结构体的指针，包含新的配置信息
// 返回错误信息，如果配置更新失败则返回相应错误
func (r *PlugRedis) Configure(c any) error {
	// 若传入的配置为 nil，直接返回 nil
	if c == nil {
		return nil
	}
	// 将传入的配置转换为 *conf.Redis 类型并更新到插件配置中
	r.conf = c.(*conf.Redis)
	return nil
}

// CheckHealth 实现 Redis 服务器的健康检查接口
// 对 Redis 服务器进行必要的健康检查，并更新提供的健康报告
// 参数 report 为健康报告指针，用于记录健康检查结果
// 返回错误信息，如果健康检查失败则返回相应错误
func (r *PlugRedis) CheckHealth() error {
	// 使用固定短超时进行健康检查，避免受连接空闲配置影响
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	// 确保在函数结束时取消上下文
	defer cancel()

	// 执行 Redis 客户端 Ping 操作进行健康检查
	start := time.Now()
	_, err := r.rdb.Ping(ctx).Result()
	latency := time.Since(start)
	redisPingLatency.Observe(latency.Seconds())
	log.Infof("redis health check: addrs=%v, ping_latency=%s", r.currentAddrs(), latency)
	if err != nil {
		return err
	}
	return nil
}
