package redis

import (
	"context"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/internal/redis/conf"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	name       = "redis"
	confPrefix = "lynx.redis"
)

type PlugRedis struct {
	rdb    *redis.Client
	conf   *conf.Redis
	weight int
}

type Option func(r *PlugRedis)

func Weight(w int) Option {
	return func(r *PlugRedis) {
		r.weight = w
	}
}

func Config(c *conf.Redis) Option {
	return func(r *PlugRedis) {
		r.conf = c
	}
}

func (r *PlugRedis) Load(b config.Value) (plugins.Plugin, error) {
	// 从配置值 b 中扫描并解析 Redis 插件的配置到 r.conf 中。
	err := b.Scan(r.conf)
	// 如果发生错误，返回 nil 和错误信息。
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录 Redis 插件初始化的信息。
	app.Lynx().GetLogHelper().Infof("Initializing Redis")

	// 创建一个新的 Redis 客户端实例，使用之前解析的配置。
	r.rdb = redis.NewClient(&redis.Options{
		// 设置 Redis 服务器的地址。
		Addr: r.conf.Addr,
		// 设置 Redis 服务器的密码。
		Password: r.conf.Password,
		// 设置要连接的 Redis 数据库的编号。
		DB: int(r.conf.Db),
		// 设置最小空闲连接数。
		MinIdleConns: int(r.conf.MinIdleConns),
		// 设置最大空闲连接数。
		MaxIdleConns: int(r.conf.MaxIdleConns),
		// 设置最大活动连接数。
		MaxActiveConns: int(r.conf.MaxActiveConns),
		// 设置拨号超时时间。
		DialTimeout: r.conf.DialTimeout.AsDuration(),
		// 设置写超时时间。
		WriteTimeout: r.conf.WriteTimeout.AsDuration(),
		// 设置读超时时间。
		ReadTimeout: r.conf.ReadTimeout.AsDuration(),
		// 设置连接最大空闲时间。
		ConnMaxIdleTime: r.conf.ConnMaxIdleTime.AsDuration(),
	})

	// 创建一个带有超时的上下文，用于测试 Redis 连接。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用 Ping 方法测试 Redis 连接。
	_, err = r.rdb.Ping(ctx).Result()
	// 如果发生错误，返回 nil 和错误信息。
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录 Redis 服务初始化成功的信息。
	app.Lynx().GetLogHelper().Infof("Redis successfully initialized")

	// 返回 Redis 插件实例和 nil 错误，表示加载成功。
	return r, nil
}

// Unload 方法用于关闭 Redis 连接并释放资源
func (r *PlugRedis) Unload() error {
	// 检查 Redis 客户端实例是否存在，如果不存在则直接返回 nil
	if r.rdb == nil {
		return nil
	}
	// 调用 Redis 客户端的 Close 方法来关闭连接，并传入一个 nil 参数
	// 如果 Close 方法返回错误，则记录错误信息
	if err := r.rdb.Close(); err != nil {
		// 使用 app.Lynx().GetLogHelper() 记录错误信息
		app.Lynx().GetLogHelper().Error(err)
		return err
	}
	// 记录一条信息，指示 Redis 资源正在被关闭
	app.Lynx().GetLogHelper().Info("message", "Closing the Redis resources")
	// 返回 nil，表示卸载过程成功，没有发生错误
	return nil
}

func Redis(opts ...Option) plugins.Plugin {
	r := &PlugRedis{
		weight: 1001,
		conf:   &conf.Redis{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
