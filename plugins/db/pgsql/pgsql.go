package pgsql

import (
	"context"
	sql "database/sql"

	stdlib "github.com/jackc/pgx/v5/stdlib"

	"time"

	esql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
// 插件元数据，包含插件名称、版本、描述和配置前缀
const (
	// 插件名称
	pluginName = "pgsql.client"
	// 插件版本
	pluginVersion = "v2.0.0"
	// 插件描述
	pluginDescription = "pgsql client plugin for lynx framework"
	// 配置前缀
	confPrefix = "lynx.pgsql"
)

// DBPgsqlClient 表示 PgSQL 客户端插件实例
type DBPgsqlClient struct {
	// 继承基础插件
	*plugins.BasePlugin
	// 数据库驱动
	dri *esql.Driver
	// PgSQL 配置
	conf *conf.Pgsql
}

// NewPgsqlClient 创建一个新的 PgSQL 客户端插件实例
// 返回一个指向 DBPgsqlClient 结构体的指针
func NewPgsqlClient() *DBPgsqlClient {
	return &DBPgsqlClient{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
			// 权重
			101,
		),
	}
}

// InitializeResources 从运行时配置中扫描并加载 PgSQL 配置
// 参数 rt 为运行时环境
// 返回错误信息，如果配置加载失败则返回相应错误
func (p *DBPgsqlClient) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	p.conf = &conf.Pgsql{}

	// 从运行时配置中扫描并加载 PgSQL 配置
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		return err
	}

	// 设置默认配置
	defaultConf := &conf.Pgsql{
		Driver:      "postgres",
		Source:      "postgres://admin:123456@127.0.0.1:5432/demo?sslmode=disable",
		MinConn:     10,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 10},
		MaxLifeTime: &durationpb.Duration{Seconds: 300},
	}

	// 对未设置的字段使用默认值
	if p.conf.Driver == "" {
		p.conf.Driver = defaultConf.Driver
	}
	if p.conf.Source == "" {
		p.conf.Source = defaultConf.Source
	}
	if p.conf.MinConn == 0 {
		p.conf.MinConn = defaultConf.MinConn
	}
	if p.conf.MaxConn == 0 {
		p.conf.MaxConn = defaultConf.MaxConn
	}
	if p.conf.MaxIdleTime == nil {
		p.conf.MaxIdleTime = defaultConf.MaxIdleTime
	}
	if p.conf.MaxLifeTime == nil {
		p.conf.MaxLifeTime = defaultConf.MaxLifeTime
	}

	return nil
}

// StartupTasks 初始化数据库连接并进行健康检查
// 返回错误信息，如果连接或健康检查失败则返回相应错误
func (p *DBPgsqlClient) StartupTasks() error {
	// 记录数据库初始化日志
	log.Infof("initializing database")

	// 注册数据库驱动
	sql.Register("postgres", stdlib.GetDefaultDriver())

	// 打开数据库连接
	drv, err := esql.Open(
		p.conf.Driver,
		p.conf.Source,
	)

	if err != nil {
		// 记录打开数据库连接失败日志
		log.Errorf("failed opening connection to dataBase: %v", err)
		// 发生错误时 panic
		panic(err)
	}

	// 设置连接池的最大空闲连接数
	drv.DB().SetMaxIdleConns(int(p.conf.MinConn))
	// 设置连接池的最大打开连接数
	drv.DB().SetMaxOpenConns(int(p.conf.MaxConn))
	// 设置连接的最大空闲时间
	drv.DB().SetConnMaxIdleTime(p.conf.MaxIdleTime.AsDuration())
	// 设置连接的最大生命周期
	drv.DB().SetConnMaxLifetime(p.conf.MaxLifeTime.AsDuration())

	// 将数据库驱动赋值给实例
	p.dri = drv
	// 记录数据库初始化成功日志
	log.Infof("database successfully initialized")
	// 原代码此处返回值有误，正确返回 nil
	return nil
}

// CleanupTasks 关闭数据库连接
// 返回错误信息，如果关闭连接失败则返回相应错误
func (p *DBPgsqlClient) CleanupTasks() error {
	if p.dri == nil {
		return nil
	}
	// 关闭数据库连接
	if err := p.dri.Close(); err != nil {
		// 记录关闭数据库连接失败日志
		log.Error(err)
		return err
	}
	// 记录关闭数据库资源日志
	log.Info("message", "Closing the DataBase resources")
	return nil
}

// Configure 更新 HTTP 服务器的配置。
// 该函数接收一个任意类型的参数，尝试将其转换为 *conf.Http 类型，如果转换成功则更新配置。
func (p *DBPgsqlClient) Configure(c any) error {
	// 尝试将传入的配置转换为 *conf.Http 类型
	if mysqlConf, ok := c.(*conf.Pgsql); ok {
		// 转换成功，更新配置
		p.conf = mysqlConf
		return nil
	}
	// 转换失败，返回配置无效错误
	return plugins.ErrInvalidConfiguration
}

// CheckHealth 对 HTTP 服务器进行健康检查。
// 该函数目前直接返回 nil，表示服务器健康，可根据实际需求添加检查逻辑。
func (p *DBPgsqlClient) CheckHealth() error {
	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 执行数据库连接健康检查
	err := p.dri.DB().PingContext(ctx)
	if err != nil {
		// 原代码此处返回值有误，正确返回错误信息
		return err
	}
	return nil
}
