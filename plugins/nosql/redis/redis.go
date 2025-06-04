package redis

import (
	"context"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugins/nosql/redis/v2/conf"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
)

// 插件元数据
const (
	// 插件唯一名称
	pluginName = "redis.client"
	// 插件版本号
	pluginVersion = "v2.0.0"
	// 插件描述信息
	pluginDescription = "redis plugin for lynx framework"
	// 配置前缀，用于从配置中读取插件相关配置
	confPrefix = "lynx.redis"
)

// PlugRedis 表示 Redis 插件实例
type PlugRedis struct {
	// 继承基础插件
	*plugins.BasePlugin
	// Redis 配置
	conf *conf.Redis
	// Redis 客户端实例
	rdb *redis.Client
}

// NewRedisClient 创建一个新的 Redis 插件实例
// 返回一个指向 PlugRedis 结构体的指针
func NewRedisClient() *PlugRedis {
	return &PlugRedis{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
		),
		conf: &conf.Redis{},
	}
}

// InitializeResources 实现 Redis 插件的自定义初始化逻辑
// 从运行时配置中扫描并加载 Redis 配置，若配置未提供则使用默认配置
// 参数 rt 为运行时环境
// 返回错误信息，如果配置加载失败则返回相应错误
func (r *PlugRedis) InitializeResources(rt plugins.Runtime) error {
	// 若未提供配置，添加默认配置
	if r.conf == nil {
		r.conf = &conf.Redis{
			Network:         "tcp",
			Addr:            "localhost:6379",
			Password:        "",
			Db:              0,
			MinIdleConns:    10,
			MaxIdleConns:    20,
			MaxActiveConns:  20,
			DialTimeout:     &durationpb.Duration{Seconds: 10, Nanos: 0},
			ReadTimeout:     &durationpb.Duration{Seconds: 10, Nanos: 0},
			WriteTimeout:    &durationpb.Duration{Seconds: 10, Nanos: 0},
			ConnMaxIdleTime: &durationpb.Duration{Seconds: 10, Nanos: 0},
		}
	}
	// 从运行时配置中扫描并加载 Redis 配置
	err := rt.GetConfig().Value(confPrefix).Scan(r.conf)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks 启动 Redis 客户端并进行健康检查
// 返回错误信息，如果启动或健康检查失败则返回相应错误
func (r *PlugRedis) StartupTasks() error {
	// 记录启动 Redis 客户端日志
	log.Infof("starting redis client")

	// 创建 Redis 客户端实例
	r.rdb = redis.NewClient(&redis.Options{
		Addr:            r.conf.Addr,
		Password:        r.conf.Password,
		DB:              int(r.conf.Db),
		MinIdleConns:    int(r.conf.MinIdleConns),
		MaxIdleConns:    int(r.conf.MaxIdleConns),
		MaxActiveConns:  int(r.conf.MaxActiveConns),
		DialTimeout:     r.conf.DialTimeout.AsDuration(),
		WriteTimeout:    r.conf.WriteTimeout.AsDuration(),
		ReadTimeout:     r.conf.ReadTimeout.AsDuration(),
		ConnMaxIdleTime: r.conf.ConnMaxIdleTime.AsDuration(),
	})

	// 记录 Redis 客户端启动成功日志
	log.Infof("redis client successfully started")
	return nil
}

// CleanupTasks 关闭 Redis 客户端
// 返回错误信息，如果关闭客户端失败则返回相应错误
func (r *PlugRedis) CleanupTasks() error {
	// 若 Redis 客户端未初始化，直接返回 nil
	if r.rdb == nil {
		return nil
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
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(
		context.Background(),
		r.conf.ConnMaxIdleTime.AsDuration(),
	)
	// 确保在函数结束时取消上下文
	defer cancel()

	// 执行 Redis 客户端 Ping 操作进行健康检查
	_, err := r.rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}
