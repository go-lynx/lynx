//go:build ignore
// +build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	fmt.Println("=== 验证数据库插件连接 ===\n")

	// 测试MySQL
	fmt.Println("测试MySQL连接...")
	mysqlDSN := "lynx:lynx123456@tcp(localhost:3306)/lynx_test?charset=utf8mb4&parseTime=True"
	mysqlDB, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		fmt.Printf("❌ MySQL连接失败: %v\n", err)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mysqlDB.PingContext(ctx); err != nil {
			fmt.Printf("❌ MySQL Ping失败: %v\n", err)
		} else {
			fmt.Println("✅ MySQL连接成功")
			var result int
			if err := mysqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
				fmt.Printf("❌ MySQL查询失败: %v\n", err)
			} else {
				fmt.Printf("✅ MySQL查询成功，结果: %d\n", result)
			}
		}
		mysqlDB.Close()
	}

	fmt.Println()

	// 测试PostgreSQL
	fmt.Println("测试PostgreSQL连接...")
	pgDSN := "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable"
	pgDB, err := sql.Open("pgx", pgDSN)
	if err != nil {
		fmt.Printf("❌ PostgreSQL连接失败: %v\n", err)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pgDB.PingContext(ctx); err != nil {
			fmt.Printf("❌ PostgreSQL Ping失败: %v\n", err)
		} else {
			fmt.Println("✅ PostgreSQL连接成功")
			var result int
			if err := pgDB.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
				fmt.Printf("❌ PostgreSQL查询失败: %v\n", err)
			} else {
				fmt.Printf("✅ PostgreSQL查询成功，结果: %d\n", result)
			}
		}
		pgDB.Close()
	}

	fmt.Println("\n=== 验证完成 ===")
	
	// 如果都成功，退出码为0，否则为1
	if mysqlDB != nil && pgDB != nil {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}

