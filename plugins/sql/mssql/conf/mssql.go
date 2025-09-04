package conf

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/durationpb"
)

// Mssql represents Microsoft SQL Server configuration
type Mssql struct {
	// The driver name for the Microsoft SQL Server database
	Driver string `json:"driver" yaml:"driver"`
	// The data source name (DSN) for the Microsoft SQL Server database
	Source string `json:"source" yaml:"source"`
	// The minimum number of connections to maintain in the connection pool
	MinConn int32 `json:"min_conn" yaml:"min_conn"`
	// The maximum number of connections to maintain in the connection pool
	MaxConn int32 `json:"max_conn" yaml:"max_conn"`
	// The maximum lifetime for a connection in the connection pool
	MaxLifeTime *durationpb.Duration `json:"max_life_time" yaml:"max_life_time"`
	// The maximum idle time for a connection in the connection pool
	MaxIdleTime *durationpb.Duration `json:"max_idle_time" yaml:"max_idle_time"`
	// The maximum number of idle connections
	MaxIdleConn int32 `json:"max_idle_conn" yaml:"max_idle_conn"`
	// SQL Server specific configuration
	ServerConfig *ServerConfig `json:"server_config" yaml:"server_config"`
}

// ServerConfig represents SQL Server specific configuration options
type ServerConfig struct {
	// SQL Server instance name
	InstanceName string `json:"instance_name" yaml:"instance_name"`
	// SQL Server port (default: 1433)
	Port int32 `json:"port" yaml:"port"`
	// Database name
	Database string `json:"database" yaml:"database"`
	// Username for authentication
	Username string `json:"username" yaml:"username"`
	// Password for authentication
	Password string `json:"password" yaml:"password"`
	// Enable encryption (default: true for Azure, false for on-premise)
	Encrypt bool `json:"encrypt" yaml:"encrypt"`
	// Trust server certificate (default: false)
	TrustServerCertificate bool `json:"trust_server_certificate" yaml:"trust_server_certificate"`
	// Connection timeout in seconds
	ConnectionTimeout int32 `json:"connection_timeout" yaml:"connection_timeout"`
	// Command timeout in seconds
	CommandTimeout int32 `json:"command_timeout" yaml:"command_timeout"`
	// Application name for connection identification
	ApplicationName string `json:"application_name" yaml:"application_name"`
	// Workstation ID for connection identification
	WorkstationID string `json:"workstation_id" yaml:"workstation_id"`
	// Enable connection pooling (default: true)
	ConnectionPooling bool `json:"connection_pooling" yaml:"connection_pooling"`
	// Maximum pool size
	MaxPoolSize int32 `json:"max_pool_size" yaml:"max_pool_size"`
	// Minimum pool size
	MinPoolSize int32 `json:"min_pool_size" yaml:"min_pool_size"`
	// Pool blocking timeout in seconds
	PoolBlockingTimeout int32 `json:"pool_blocking_timeout" yaml:"pool_blocking_timeout"`
	// Pool lifetime timeout in seconds
	PoolLifetimeTimeout int32 `json:"pool_lifetime_timeout" yaml:"pool_lifetime_timeout"`
}

// GetDriver returns the driver name
func (m *Mssql) GetDriver() string {
	return m.Driver
}

// GetSource returns the data source name
func (m *Mssql) GetSource() string {
	return m.Source
}

// GetMinConn returns the minimum number of connections
func (m *Mssql) GetMinConn() int32 {
	return m.MinConn
}

// GetMaxConn returns the maximum number of connections
func (m *Mssql) GetMaxConn() int32 {
	return m.MaxConn
}

// GetMaxLifeTime returns the maximum lifetime for a connection
func (m *Mssql) GetMaxLifeTime() *durationpb.Duration {
	return m.MaxLifeTime
}

// GetMaxIdleTime returns the maximum idle time for a connection
func (m *Mssql) GetMaxIdleTime() *durationpb.Duration {
	return m.MaxIdleTime
}

// BuildConnectionString builds a SQL Server connection string from configuration
func (m *Mssql) BuildConnectionString() string {
	if m.Source != "" {
		return m.Source
	}

	var parts []string

	// Server and port
	if m.ServerConfig != nil && m.ServerConfig.InstanceName != "" {
		parts = append(parts, fmt.Sprintf("server=%s", m.ServerConfig.InstanceName))
		if m.ServerConfig.Port > 0 {
			parts = append(parts, fmt.Sprintf("port=%d", m.ServerConfig.Port))
		}
	} else {
		parts = append(parts, "server=localhost")
		if m.ServerConfig != nil && m.ServerConfig.Port > 0 {
			parts = append(parts, fmt.Sprintf("port=%d", m.ServerConfig.Port))
		} else {
			parts = append(parts, "port=1433")
		}
	}

	// Database
	if m.ServerConfig != nil && m.ServerConfig.Database != "" {
		parts = append(parts, fmt.Sprintf("database=%s", m.ServerConfig.Database))
	}

	// Authentication
	if m.ServerConfig != nil && m.ServerConfig.Username != "" {
		parts = append(parts, fmt.Sprintf("user id=%s", m.ServerConfig.Username))
		if m.ServerConfig.Password != "" {
			parts = append(parts, fmt.Sprintf("password=%s", m.ServerConfig.Password))
		}
	} else {
		parts = append(parts, "trusted_connection=true")
	}

	// Encryption
	if m.ServerConfig != nil {
		if m.ServerConfig.Encrypt {
			parts = append(parts, "encrypt=true")
			if m.ServerConfig.TrustServerCertificate {
				parts = append(parts, "trustServerCertificate=true")
			}
		} else {
			parts = append(parts, "encrypt=false")
		}

		// Timeouts
		if m.ServerConfig.ConnectionTimeout > 0 {
			parts = append(parts, fmt.Sprintf("connection timeout=%d", m.ServerConfig.ConnectionTimeout))
		}
		if m.ServerConfig.CommandTimeout > 0 {
			parts = append(parts, fmt.Sprintf("command timeout=%d", m.ServerConfig.CommandTimeout))
		}

		// Application identification
		if m.ServerConfig.ApplicationName != "" {
			parts = append(parts, fmt.Sprintf("app name=%s", m.ServerConfig.ApplicationName))
		}
		if m.ServerConfig.WorkstationID != "" {
			parts = append(parts, fmt.Sprintf("workstation id=%s", m.ServerConfig.WorkstationID))
		}

		// Connection pooling
		if m.ServerConfig.ConnectionPooling {
			parts = append(parts, "connection pooling=true")
			if m.ServerConfig.MaxPoolSize > 0 {
				parts = append(parts, fmt.Sprintf("max pool size=%d", m.ServerConfig.MaxPoolSize))
			}
			if m.ServerConfig.MinPoolSize > 0 {
				parts = append(parts, fmt.Sprintf("min pool size=%d", m.ServerConfig.MinPoolSize))
			}
			if m.ServerConfig.PoolBlockingTimeout > 0 {
				parts = append(parts, fmt.Sprintf("pool blocking timeout=%d", m.ServerConfig.PoolBlockingTimeout))
			}
			if m.ServerConfig.PoolLifetimeTimeout > 0 {
				parts = append(parts, fmt.Sprintf("pool lifetime timeout=%d", m.ServerConfig.PoolLifetimeTimeout))
			}
		}
	}

	return strings.Join(parts, ";")
}

// Validate validates the configuration
func (m *Mssql) Validate() error {
	if m.Driver == "" {
		m.Driver = "mssql"
	}

	if m.MinConn <= 0 {
		m.MinConn = 1
	}

	if m.MaxConn <= 0 {
		m.MaxConn = 10
	}

	if m.MaxConn < m.MinConn {
		m.MaxConn = m.MinConn
	}

	if m.MaxIdleConn <= 0 {
		m.MaxIdleConn = int32(m.MinConn)
	}

	if m.MaxIdleTime == nil {
		m.MaxIdleTime = &durationpb.Duration{Seconds: 300} // 5 minutes
	}

	if m.MaxLifeTime == nil {
		m.MaxLifeTime = &durationpb.Duration{Seconds: 3600} // 1 hour
	}

	// Set default values for ServerConfig if not provided
	if m.ServerConfig == nil {
		m.ServerConfig = &ServerConfig{}
	}

	if m.ServerConfig.Port <= 0 {
		m.ServerConfig.Port = 1433
	}

	if m.ServerConfig.ApplicationName == "" {
		m.ServerConfig.ApplicationName = "Lynx-MSSQL-Plugin"
	}

	if m.ServerConfig.ConnectionTimeout <= 0 {
		m.ServerConfig.ConnectionTimeout = 30
	}

	if m.ServerConfig.CommandTimeout <= 0 {
		m.ServerConfig.CommandTimeout = 30
	}

	if m.ServerConfig.MaxPoolSize <= 0 {
		m.ServerConfig.MaxPoolSize = m.MaxConn
	}

	if m.ServerConfig.MinPoolSize <= 0 {
		m.ServerConfig.MinPoolSize = m.MinConn
	}

	if m.ServerConfig.PoolBlockingTimeout <= 0 {
		m.ServerConfig.PoolBlockingTimeout = 30
	}

	if m.ServerConfig.PoolLifetimeTimeout <= 0 {
		m.ServerConfig.PoolLifetimeTimeout = 3600
	}

	return nil
}
