package conf

import (
	"time"
)

// OpenIM configuration structure
type OpenIM struct {
	// Server configuration
	Server *Server `json:"server" yaml:"server"`
	// Client configuration
	Client *Client `json:"client" yaml:"client"`
	// Message configuration
	Message *Message `json:"message" yaml:"message"`
	// Security configuration
	Security *Security `json:"security" yaml:"security"`
	// Storage configuration
	Storage *Storage `json:"storage" yaml:"storage"`
}

// Server configuration
type Server struct {
	// Server address (e.g., "localhost:10002")
	Addr string `json:"addr" yaml:"addr"`
	// API version
	APIVersion string `json:"api_version" yaml:"api_version"`
	// Platform ID
	PlatformID int32 `json:"platform_id" yaml:"platform_id"`
	// Server name
	ServerName string `json:"server_name" yaml:"server_name"`
	// Log level
	LogLevel string `json:"log_level" yaml:"log_level"`
	// Log output path
	LogOutputPath string `json:"log_output_path" yaml:"log_output_path"`
	// Log rotation max size (MB)
	LogRotationMaxSize int32 `json:"log_rotation_max_size" yaml:"log_rotation_max_size"`
	// Log rotation max age (days)
	LogRotationMaxAge int32 `json:"log_rotation_max_age" yaml:"log_rotation_max_age"`
	// Log rotation max backups
	LogRotationMaxBackups int32 `json:"log_rotation_max_backups" yaml:"log_rotation_max_backups"`
	// Log is stdout
	LogIsStdout bool `json:"log_is_stdout" yaml:"log_is_stdout"`
	// Log is json
	LogIsJSON bool `json:"log_is_json" yaml:"log_is_json"`
	// Log with stack
	LogWithStack bool `json:"log_with_stack" yaml:"log_with_stack"`
}

// Client configuration
type Client struct {
	// Client user ID
	UserID string `json:"user_id" yaml:"user_id"`
	// Client token
	Token string `json:"token" yaml:"token"`
	// Client platform ID
	PlatformID int32 `json:"platform_id" yaml:"platform_id"`
	// Client server address
	ServerAddr string `json:"server_addr" yaml:"server_addr"`
	// Client API version
	APIVersion string `json:"api_version" yaml:"api_version"`
	// Client timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// Client heartbeat interval
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
}

// Message configuration
type Message struct {
	// Message type (text, image, voice, video, file, etc.)
	Type string `json:"type" yaml:"type"`
	// Message content
	Content string `json:"content" yaml:"content"`
	// Message sender
	Sender string `json:"sender" yaml:"sender"`
	// Message receiver
	Receiver string `json:"receiver" yaml:"receiver"`
	// Message group ID (for group messages)
	GroupID string `json:"group_id" yaml:"group_id"`
	// Message sequence number
	Sequence int64 `json:"sequence" yaml:"sequence"`
	// Message timestamp
	Timestamp int64 `json:"timestamp" yaml:"timestamp"`
	// Message status
	Status string `json:"status" yaml:"status"`
}

// Security configuration
type Security struct {
	// Enable TLS
	TLSEnable bool `json:"tls_enable" yaml:"tls_enable"`
	// TLS certificate file
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
	// TLS key file
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file"`
	// TLS CA file
	TLSCAFile string `json:"tls_ca_file" yaml:"tls_ca_file"`
	// Enable authentication
	AuthEnable bool `json:"auth_enable" yaml:"auth_enable"`
	// JWT secret
	JWTSecret string `json:"jwt_secret" yaml:"jwt_secret"`
	// JWT expire time
	JWTExpire time.Duration `json:"jwt_expire" yaml:"jwt_expire"`
}

// Storage configuration
type Storage struct {
	// Storage type (redis, mysql, mongodb, etc.)
	Type string `json:"type" yaml:"type"`
	// Storage address
	Addr string `json:"addr" yaml:"addr"`
	// Storage username
	Username string `json:"username" yaml:"username"`
	// Storage password
	Password string `json:"password" yaml:"password"`
	// Storage database
	Database string `json:"database" yaml:"database"`
	// Storage pool size
	PoolSize int32 `json:"pool_size" yaml:"pool_size"`
	// Storage timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}
