package pgsql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"

	esql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
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
	// 默认配置常量
	defaultDriver      = "postgres"
	defaultSource      = "postgres://admin:123456@127.0.0.1:5432/demo?sslmode=disable"
	defaultMinConn     = 10
	defaultMaxConn     = 20
	defaultMaxIdleTime = 10 * time.Second
	defaultMaxLifeTime = 300 * time.Second
	// 健康检查超时时间
	healthCheckTimeout = 5 * time.Second
	// 最大重试次数
	maxRetryAttempts = 3
	// 重试间隔
	retryInterval = 2 * time.Second
)

// DBPgsqlClient 表示 PgSQL 客户端插件实例
type DBPgsqlClient struct {
	// 继承基础插件
	*plugins.BasePlugin
	// 数据库驱动
	dri *esql.Driver
	// PgSQL 配置
	conf *conf.Pgsql
	// 连接池统计信息
	stats *ConnectionPoolStats
	// 关闭信号通道
	closeChan chan struct{}
	// 是否已关闭
	closed bool
	// Prometheus 监控指标
	prometheusMetrics *PrometheusMetrics
}

// ConnectionPoolStats 连接池统计信息
type ConnectionPoolStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	MaxIdleConnections int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
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
		closeChan: make(chan struct{}),
		stats:     &ConnectionPoolStats{},
	}
}

// validateConfig 验证配置参数的有效性
func (p *DBPgsqlClient) validateConfig() error {
	if p.conf == nil {
		return fmt.Errorf("configuration is nil")
	}

	// 验证连接字符串格式
	if p.conf.Source != "" {
		if !strings.Contains(p.conf.Source, "://") {
			return fmt.Errorf("invalid connection string format, expected format: postgres://user:password@host:port/dbname")
		}
	}

	// 验证连接池配置
	if p.conf.MinConn < 0 {
		return fmt.Errorf("min_conn cannot be negative")
	}
	if p.conf.MaxConn <= 0 {
		return fmt.Errorf("max_conn must be positive")
	}
	if p.conf.MinConn > p.conf.MaxConn {
		return fmt.Errorf("min_conn (%d) cannot be greater than max_conn (%d)", p.conf.MinConn, p.conf.MaxConn)
	}

	// 验证时间配置
	if p.conf.MaxIdleTime != nil {
		if p.conf.MaxIdleTime.AsDuration() < 0 {
			return fmt.Errorf("max_idle_time cannot be negative")
		}
	}
	if p.conf.MaxLifeTime != nil {
		if p.conf.MaxLifeTime.AsDuration() < 0 {
			return fmt.Errorf("max_life_time cannot be negative")
		}
	}

	return nil
}

// setDefaultConfig 设置默认配置
func (p *DBPgsqlClient) setDefaultConfig() {
	if p.conf.Driver == "" {
		p.conf.Driver = defaultDriver
	}
	if p.conf.Source == "" {
		p.conf.Source = defaultSource
	}
	if p.conf.MinConn == 0 {
		p.conf.MinConn = defaultMinConn
	}
	if p.conf.MaxConn == 0 {
		p.conf.MaxConn = defaultMaxConn
	}
	if p.conf.MaxIdleTime == nil {
		p.conf.MaxIdleTime = durationpb.New(defaultMaxIdleTime)
	}
	if p.conf.MaxLifeTime == nil {
		p.conf.MaxLifeTime = durationpb.New(defaultMaxLifeTime)
	}

	// Prometheus 监控配置可以通过环境变量或配置文件设置
	// 暂时使用默认配置
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
		log.Errorf("failed to scan pgsql configuration: %v", err)
		return fmt.Errorf("failed to load pgsql configuration: %w", err)
	}

	// 设置默认配置
	p.setDefaultConfig()

	// 验证配置
	if err := p.validateConfig(); err != nil {
		log.Errorf("invalid pgsql configuration: %v", err)
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	log.Infof("pgsql configuration loaded successfully: driver=%s, min_conn=%d, max_conn=%d",
		p.conf.Driver, p.conf.MinConn, p.conf.MaxConn)
	return nil
}

// connectWithRetry 带重试机制的数据库连接
func (p *DBPgsqlClient) connectWithRetry() (*esql.Driver, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		log.Infof("attempting to connect to database (attempt %d/%d)", attempt, maxRetryAttempts)

		// 注册数据库驱动
		sql.Register("postgres", stdlib.GetDefaultDriver())

		// 打开数据库连接
		drv, err := esql.Open(p.conf.Driver, p.conf.Source)
		if err != nil {
			lastErr = err
			log.Warnf("connection attempt %d failed: %v", attempt, err)

			if attempt < maxRetryAttempts {
				log.Infof("retrying in %v...", retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetryAttempts, lastErr)
		}

		// 配置连接池
		db := drv.DB()
		db.SetMaxIdleConns(int(p.conf.MinConn))
		db.SetMaxOpenConns(int(p.conf.MaxConn))
		db.SetConnMaxIdleTime(p.conf.MaxIdleTime.AsDuration())
		db.SetConnMaxLifetime(p.conf.MaxLifeTime.AsDuration())

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			drv.Close()
			lastErr = err
			log.Warnf("connection test failed on attempt %d: %v", attempt, err)

			if attempt < maxRetryAttempts {
				log.Infof("retrying in %v...", retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			return nil, fmt.Errorf("connection test failed after %d attempts: %w", maxRetryAttempts, lastErr)
		}

		log.Infof("database connection established successfully on attempt %d", attempt)
		return drv, nil
	}

	return nil, fmt.Errorf("failed to establish database connection: %w", lastErr)
}

// initPrometheusMetrics 初始化 Prometheus 监控指标
func (p *DBPgsqlClient) initPrometheusMetrics() {
	// 创建 Prometheus 配置
	promConfig := createPrometheusConfig(p.conf)

	// 创建 Prometheus 指标
	p.prometheusMetrics = NewPrometheusMetrics(promConfig)
}

// MetricsGatherer 返回该插件的 Prometheus Gatherer（用于统一注册聚合）
func (p *DBPgsqlClient) MetricsGatherer() prometheus.Gatherer {
	if p == nil || p.prometheusMetrics == nil {
		return nil
	}
	return p.prometheusMetrics.GetGatherer()
}

// updateStats 更新连接池统计信息
func (p *DBPgsqlClient) updateStats() {
	if p.dri == nil {
		return
	}

	db := p.dri.DB()
	stats := db.Stats()
	p.stats.MaxOpenConnections = stats.MaxOpenConnections
	p.stats.OpenConnections = stats.OpenConnections
	p.stats.InUse = stats.InUse
	p.stats.Idle = stats.Idle
	p.stats.MaxIdleConnections = int(p.conf.MinConn) // 使用配置的最小连接数
	p.stats.WaitCount = stats.WaitCount
	p.stats.WaitDuration = stats.WaitDuration
	p.stats.MaxIdleClosed = stats.MaxIdleClosed
	p.stats.MaxLifetimeClosed = stats.MaxLifetimeClosed

	// 更新 Prometheus 指标
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.UpdateMetrics(p.stats, p.conf)
	}
}

// StartupTasks 初始化数据库连接并进行健康检查
// 返回错误信息，如果连接或健康检查失败则返回相应错误
func (p *DBPgsqlClient) StartupTasks() error {
	log.Infof("initializing pgsql database connection")

	// 使用重试机制连接数据库
	drv, err := p.connectWithRetry()
	if err != nil {
		log.Errorf("failed to initialize database connection: %v", err)
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// 将数据库驱动赋值给实例
	p.dri = drv

	// 初始化 Prometheus 监控
	p.initPrometheusMetrics()

	// 更新统计信息
	p.updateStats()

	log.Infof("pgsql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		p.conf.MaxConn, p.conf.MinConn)
	return nil
}

// CleanupTasks 优雅关闭数据库连接
// 返回错误信息，如果关闭连接失败则返回相应错误
func (p *DBPgsqlClient) CleanupTasks() error {
	if p.dri == nil || p.closed {
		return nil
	}

	log.Infof("closing pgsql database connection")

	// 标记为已关闭
	p.closed = true
	close(p.closeChan)

	// 优雅关闭连接
	if err := p.dri.Close(); err != nil {
		log.Errorf("failed to close database connection: %v", err)
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	log.Infof("pgsql database connection closed successfully")
	return nil
}

// Configure 更新 PgSQL 配置
// 该函数接收一个任意类型的参数，尝试将其转换为 *conf.Pgsql 类型，如果转换成功则更新配置
func (p *DBPgsqlClient) Configure(c any) error {
	// 尝试将传入的配置转换为 *conf.Pgsql 类型
	if pgsqlConf, ok := c.(*conf.Pgsql); ok {
		// 保存旧配置用于回滚
		oldConf := p.conf
		p.conf = pgsqlConf

		// 设置默认配置
		p.setDefaultConfig()

		// 验证新配置
		if err := p.validateConfig(); err != nil {
			// 配置无效，回滚到旧配置
			p.conf = oldConf
			log.Errorf("invalid new configuration, rolling back: %v", err)
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		log.Infof("pgsql configuration updated successfully")
		return nil
	}

	// 转换失败，返回配置无效错误
	return plugins.ErrInvalidConfiguration
}

// CheckHealth 对数据库连接进行全面的健康检查
// 该函数检查连接池状态和数据库连接健康性
func (p *DBPgsqlClient) CheckHealth() error {
	if p.dri == nil {
		return fmt.Errorf("database driver is not initialized")
	}

	// 更新统计信息
	p.updateStats()

	// 检查连接池状态
	if p.stats.OpenConnections >= p.stats.MaxOpenConnections {
		log.Warnf("connection pool is at maximum capacity: %d/%d",
			p.stats.OpenConnections, p.stats.MaxOpenConnections)
	}

	// 检查等待连接的情况
	if p.stats.WaitCount > 0 {
		log.Warnf("connection pool has wait count: %d, total wait duration: %v",
			p.stats.WaitCount, p.stats.WaitDuration)
	}

	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	// 执行数据库连接健康检查
	err := p.dri.DB().PingContext(ctx)

	// 记录 Prometheus 健康检查指标
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.RecordHealthCheck(err == nil, p.conf)
	}

	if err != nil {
		log.Errorf("database health check failed: %v", err)
		return fmt.Errorf("database health check failed: %w", err)
	}

	log.Debugf("database health check passed, pool stats: open=%d, in_use=%d, idle=%d",
		p.stats.OpenConnections, p.stats.InUse, p.stats.Idle)
	return nil
}

// GetStats 获取连接池统计信息
func (p *DBPgsqlClient) GetStats() *ConnectionPoolStats {
	if p.dri == nil {
		return nil
	}
	p.updateStats()
	return p.stats
}

// GetConfig 获取当前配置
func (p *DBPgsqlClient) GetConfig() *conf.Pgsql {
	return p.conf
}

// IsConnected 检查是否已连接
func (p *DBPgsqlClient) IsConnected() bool {
	return p.dri != nil && !p.closed
}

// GetDriver 获取数据库驱动
func (p *DBPgsqlClient) GetDriver() *esql.Driver {
	return p.dri
}
