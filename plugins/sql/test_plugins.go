//go:build ignore
// +build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	"github.com/go-lynx/lynx/plugins/sql/mysql"
	"github.com/go-lynx/lynx/plugins/sql/pgsql"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	fmt.Println("=== 验证数据库插件运行状态 ===\n")

	// 测试MySQL插件
	fmt.Println("1. 测试MySQL插件...")
	if err := testMySQLPlugin(); err != nil {
		fmt.Printf("❌ MySQL插件测试失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ MySQL插件测试通过\n")

	// 测试PostgreSQL插件
	fmt.Println("2. 测试PostgreSQL插件...")
	if err := testPostgreSQLPlugin(); err != nil {
		fmt.Printf("❌ PostgreSQL插件测试失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ PostgreSQL插件测试通过\n")

	fmt.Println("=== 所有测试通过 ===")
}

func testMySQLPlugin() error {
	// 检查MySQL是否可用
	if !isMySQLAvailable() {
		return fmt.Errorf("MySQL不可用")
	}

	// 创建MySQL客户端
	client := mysql.NewMysqlClient()

	// 创建runtime
	rt := createMySQLRuntime()

	// 初始化资源
	if err := client.InitializeResources(rt); err != nil {
		return fmt.Errorf("InitializeResources失败: %w", err)
	}

	// 启动插件
	if err := client.StartupTasks(); err != nil {
		return fmt.Errorf("StartupTasks失败: %w", err)
	}
	defer client.CleanupTasks()

	// 验证连接
	if !client.IsConnected() {
		return fmt.Errorf("插件未连接")
	}

	// 测试获取数据库连接
	db, err := client.GetDB()
	if err != nil {
		return fmt.Errorf("GetDB失败: %w", err)
	}

	// 测试查询
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("查询结果不符合预期: 期望1，实际%d", result)
	}

	// 测试健康检查
	if err := client.CheckHealth(); err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}

	// 测试连接池统计
	stats := client.GetStats()
	fmt.Printf("  连接池统计: MaxOpen=%d, Open=%d, InUse=%d, Idle=%d\n",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse, stats.Idle)

	// 验证连接池配置
	if stats.MaxOpenConnections != 10 {
		return fmt.Errorf("MaxOpenConnections不符合预期: 期望10，实际%d", stats.MaxOpenConnections)
	}

	return nil
}

func testPostgreSQLPlugin() error {
	// 检查PostgreSQL是否可用
	if !isPostgreSQLAvailable() {
		return fmt.Errorf("PostgreSQL不可用")
	}

	// 创建PostgreSQL客户端
	client := pgsql.NewPgsqlClient()

	// 创建runtime
	rt := createPostgreSQLRuntime()

	// 初始化资源
	if err := client.InitializeResources(rt); err != nil {
		return fmt.Errorf("InitializeResources失败: %w", err)
	}

	// 启动插件
	if err := client.StartupTasks(); err != nil {
		return fmt.Errorf("StartupTasks失败: %w", err)
	}
	defer client.CleanupTasks()

	// 验证连接
	if !client.IsConnected() {
		return fmt.Errorf("插件未连接")
	}

	// 测试获取数据库连接
	db, err := client.GetDB()
	if err != nil {
		return fmt.Errorf("GetDB失败: %w", err)
	}

	// 测试查询
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("查询失败: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("查询结果不符合预期: 期望1，实际%d", result)
	}

	// 测试健康检查
	if err := client.CheckHealth(); err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}

	// 测试连接池统计
	stats := client.GetStats()
	fmt.Printf("  连接池统计: MaxOpen=%d, Open=%d, InUse=%d, Idle=%d\n",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse, stats.Idle)

	return nil
}

func isMySQLAvailable() bool {
	db, err := sql.Open("mysql", "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True")
	if err != nil {
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return false
	}
	return true
}

func isPostgreSQLAvailable() bool {
	db, err := sql.Open("pgx", "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable")
	if err != nil {
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return false
	}
	return true
}

func createMySQLRuntime() plugins.Runtime {
	mockConfig := &mockConfig{
		values: map[string]interface{}{
			"lynx.mysql": &interfaces.Config{
				Driver:              "mysql",
				DSN:                 "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True",
				MaxOpenConns:        10,
				MaxIdleConns:        5,
				ConnMaxLifetime:     3600,
				ConnMaxIdleTime:     300,
				HealthCheckInterval: 0,
				AutoReconnectInterval: 0,
			},
		},
	}

	rt := app.NewTypedRuntimePlugin()
	rt.SetConfig(mockConfig)
	return rt
}

func createPostgreSQLRuntime() plugins.Runtime {
	mockConfig := &mockConfig{
		values: map[string]interface{}{
			"lynx.pgsql": &conf.Pgsql{
				Driver:  "pgx",
				Source:  "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable",
				MinConn: 5,
				MaxConn: 20,
			},
		},
	}

	rt := app.NewTypedRuntimePlugin()
	rt.SetConfig(mockConfig)
	return rt
}

// mockConfig实现
type mockConfig struct {
	values map[string]interface{}
}

func (m *mockConfig) Value(key string) config.Value {
	return &mockValue{key: key, values: m.values}
}

type mockValue struct {
	key    string
	values map[string]interface{}
}

func (m *mockValue) Scan(dest interface{}) error {
	if val, ok := m.values[m.key]; ok {
		if cfg, ok := dest.(*interfaces.Config); ok {
			if configVal, ok := val.(*interfaces.Config); ok {
				*cfg = *configVal
				return nil
			}
		}
		if pbConfig, ok := dest.(*conf.Pgsql); ok {
			if cfg, ok := val.(*conf.Pgsql); ok {
				*pbConfig = *cfg
				return nil
			}
		}
	}
	return nil
}

func (m *mockValue) String() (string, error) { return "", nil }
func (m *mockValue) Bool() (bool, error) { return false, nil }
func (m *mockValue) Int() (int64, error) { return 0, nil }
func (m *mockValue) Float() (float64, error) { return 0, nil }
func (m *mockValue) Duration() (time.Duration, error) { return 0, nil }

func (m *mockConfig) Load() error { return nil }
func (m *mockConfig) Watch(key string, o config.Observer) error { return nil }
func (m *mockConfig) Close() error { return nil }

