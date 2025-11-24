package main

import (
	"fmt"
	"os"

	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	"github.com/go-lynx/lynx/plugins/sql/mysql"
	"github.com/go-lynx/lynx/plugins/sql/pgsql"
)

func main() {
	fmt.Println("=== 验证数据库插件连接池配置 ===\n")

	// 测试MySQL默认配置
	fmt.Println("1. 测试MySQL默认连接池配置:")
	mysqlClient := mysql.NewMysqlClient()
	checkConnectionPoolConfig("MySQL", mysqlClient.GetConfig())

	// 测试PostgreSQL默认配置
	fmt.Println("\n2. 测试PostgreSQL默认连接池配置:")
	pgsqlClient := pgsql.NewPgsqlClient()
	checkConnectionPoolConfig("PostgreSQL", pgsqlClient.GetConfig())

	// 测试自定义配置
	fmt.Println("\n3. 测试自定义连接池配置:")
	customConfig := &interfaces.Config{
		Driver:       "mysql",
		DSN:          "test://dsn",
		MaxOpenConns: 50,
		MaxIdleConns: 10,
	}
	checkConnectionPoolConfig("Custom", customConfig)

	// 验证配置合理性
	fmt.Println("\n4. 验证配置合理性:")
	validateConfig(customConfig)

	fmt.Println("\n=== 验证完成 ===")
}

func checkConnectionPoolConfig(name string, config *interfaces.Config) {
	if config == nil {
		fmt.Printf("  ❌ %s: 配置为nil\n", name)
		return
	}

	fmt.Printf("  ✅ %s 连接池配置:\n", name)
	fmt.Printf("    - MaxOpenConns: %d\n", config.MaxOpenConns)
	fmt.Printf("    - MaxIdleConns: %d\n", config.MaxIdleConns)
	fmt.Printf("    - ConnMaxLifetime: %d秒\n", config.ConnMaxLifetime)
	fmt.Printf("    - ConnMaxIdleTime: %d秒\n", config.ConnMaxIdleTime)

	// 检查配置合理性
	if config.MaxIdleConns > config.MaxOpenConns {
		fmt.Printf("    ⚠️  警告: MaxIdleConns (%d) > MaxOpenConns (%d)\n", 
			config.MaxIdleConns, config.MaxOpenConns)
		os.Exit(1)
	}

	if config.MaxOpenConns <= 0 {
		fmt.Printf("    ⚠️  警告: MaxOpenConns 必须大于0\n")
		os.Exit(1)
	}

	// 检查默认值是否合理
	if name == "MySQL" {
		if config.MaxOpenConns != 25 {
			fmt.Printf("    ⚠️  警告: MySQL默认MaxOpenConns应该是25，实际是%d\n", config.MaxOpenConns)
		}
		if config.MaxIdleConns != 5 {
			fmt.Printf("    ⚠️  警告: MySQL默认MaxIdleConns应该是5，实际是%d\n", config.MaxIdleConns)
		}
	}
}

func validateConfig(config *interfaces.Config) {
	issues := []string{}

	if config.MaxOpenConns <= 0 {
		issues = append(issues, "MaxOpenConns必须大于0")
	}

	if config.MaxIdleConns < 0 {
		issues = append(issues, "MaxIdleConns不能为负数")
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		issues = append(issues, fmt.Sprintf("MaxIdleConns (%d) 不能大于 MaxOpenConns (%d)", 
			config.MaxIdleConns, config.MaxOpenConns))
	}

	if config.ConnMaxLifetime < 0 {
		issues = append(issues, "ConnMaxLifetime不能为负数")
	}

	if config.ConnMaxIdleTime < 0 {
		issues = append(issues, "ConnMaxIdleTime不能为负数")
	}

	if len(issues) > 0 {
		fmt.Println("  ❌ 发现配置问题:")
		for _, issue := range issues {
			fmt.Printf("    - %s\n", issue)
		}
		os.Exit(1)
	} else {
		fmt.Println("  ✅ 所有配置验证通过")
	}
}

