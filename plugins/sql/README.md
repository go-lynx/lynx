# SQL Plugin

一个简洁、灵活的 SQL 数据库插件系统，为 Lynx 框架提供数据库连接管理。

## 设计理念

- **简洁性**：只提供核心的数据库连接管理功能
- **自由度**：用户可以自由选择任何 ORM 或查询构建器
- **可扩展**：轻松支持不同的数据库类型

## 核心功能

- ✅ 数据库连接池管理
- ✅ 自动健康检查
- ✅ 优雅关闭
- ✅ 多数据库支持（MySQL、PostgreSQL、MSSQL等）

## 使用示例

### 基础使用

```go
// 获取数据库连接
db, err := sqlPlugin.GetDB()
if err != nil {
    return err
}

// 执行原生 SQL
rows, err := db.Query("SELECT * FROM users WHERE age > ?", 18)
```

### 与 GORM 集成

```go
import (
    "gorm.io/gorm"
    "gorm.io/driver/mysql"
)

func setupGORM(sqlPlugin interfaces.SQLPlugin) (*gorm.DB, error) {
    db, err := sqlPlugin.GetDB()
    if err != nil {
        return nil, err
    }
    
    return gorm.Open(mysql.New(mysql.Config{
        Conn: db,
    }), &gorm.Config{})
}
```

### 与 sqlx 集成

```go
import "github.com/jmoiron/sqlx"

func setupSQLX(sqlPlugin interfaces.SQLPlugin) (*sqlx.DB, error) {
    db, err := sqlPlugin.GetDB()
    if err != nil {
        return nil, err
    }
    
    return sqlx.NewDb(db, sqlPlugin.GetDialect()), nil
}
```

### 与 Bun 集成

```go
import (
    "github.com/uptrace/bun"
    "github.com/uptrace/bun/dialect/mysqldialect"
)

func setupBun(sqlPlugin interfaces.SQLPlugin) (*bun.DB, error) {
    db, err := sqlPlugin.GetDB()
    if err != nil {
        return nil, err
    }
    
    bunDB := bun.NewDB(db, mysqldialect.New())
    return bunDB, nil
}
```

## 配置

```yaml
mysql:
  driver: mysql
  dsn: "user:password@tcp(localhost:3306)/database?charset=utf8mb4&parseTime=True"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 3600  # seconds
  conn_max_idle_time: 300  # seconds
  health_check_interval: 30  # seconds, 0 to disable
  health_check_query: "SELECT 1"  # optional custom query
```

## 项目结构

```
plugins/sql/
├── interfaces/
│   └── sql.go          # 核心接口定义
├── base/
│   ├── base_plugin.go  # 基础插件实现
│   └── health_checker.go # 健康检查
├── mysql/
│   └── mysql_plugin.go # MySQL 实现
├── postgres/
│   └── postgres_plugin.go # PostgreSQL 实现
└── mssql/
    └── mssql_plugin.go # MSSQL 实现
```

## 为什么这样设计？

1. **不重复造轮子**：市面上已有很多优秀的 ORM 和查询构建器，我们不需要再造一个
2. **保持灵活性**：不同项目有不同需求，强制使用特定 ORM 会限制用户选择
3. **专注核心功能**：插件应该专注于连接管理，让用户自由选择上层工具
4. **易于维护**：代码简洁，功能明确，维护成本低

## 支持的数据库

- MySQL / MariaDB
- PostgreSQL
- Microsoft SQL Server
- SQLite
- ClickHouse（计划中）

## License

MIT