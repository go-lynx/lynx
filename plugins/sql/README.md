# SQL Plugin

A simple and flexible SQL database plugin system that provides database connection management for the Lynx framework.

## Design Philosophy

- **Simplicity**: Only provides core database connection management functionality
- **Freedom**: Users can freely choose any ORM or query builder
- **Extensibility**: Easy to support different database types

## Core Features

- ✅ Database connection pool management
- ✅ Automatic health checks
- ✅ Graceful shutdown
- ✅ Multi-database support (MySQL, PostgreSQL, MSSQL, etc.)

## Usage Examples

### Basic Usage

```go
// Get database connection
db, err := sqlPlugin.GetDB()
if err != nil {
    return err
}

// Execute raw SQL
rows, err := db.Query("SELECT * FROM users WHERE age > ?", 18)
```

### Integration with GORM

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

### Integration with sqlx

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

### Integration with Bun

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

## Configuration

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

## Project Structure

```
plugins/sql/
├── interfaces/
│   └── sql.go          # Core interface definitions
├── base/
│   ├── base_plugin.go  # Base plugin implementation
│   └── health_checker.go # Health checker
├── mysql/
│   └── mysql_plugin.go # MySQL implementation
├── postgres/
│   └── postgres_plugin.go # PostgreSQL implementation
└── mssql/
    └── mssql_plugin.go # MSSQL implementation
```

## Why This Design?

1. **Don't Reinvent the Wheel**: There are already many excellent ORMs and query builders in the market, we don't need to create another one
2. **Maintain Flexibility**: Different projects have different requirements, forcing the use of a specific ORM would limit user choices
3. **Focus on Core Functionality**: Plugins should focus on connection management, allowing users to freely choose upper-layer tools
4. **Easy Maintenance**: Simple code, clear functionality, low maintenance cost

## Supported Databases

- MySQL / MariaDB
- PostgreSQL
- Microsoft SQL Server
- SQLite
- ClickHouse (planned)

## License

MIT