package plugin

import (
	"fmt"
	"strings"
)

// PluginType represents the type of plugin
type PluginType string

const (
	TypeService PluginType = "service"
	TypeMQ      PluginType = "mq"
	TypeSQL     PluginType = "sql"
	TypeNoSQL   PluginType = "nosql"
	TypeTracer  PluginType = "tracer"
	TypeDTX     PluginType = "dtx"
	TypeConfig  PluginType = "config"
	TypeOther   PluginType = "other"
)

// PluginStatus represents the installation status
type PluginStatus string

const (
	StatusInstalled    PluginStatus = "installed"
	StatusNotInstalled PluginStatus = "not_installed"
	StatusUpdatable    PluginStatus = "updatable"
	StatusUnknown      PluginStatus = "unknown"
)

// Dependency represents a plugin dependency
type Dependency struct {
	Name     string `json:"name" yaml:"name"`
	Version  string `json:"version" yaml:"version"`
	Required bool   `json:"required" yaml:"required"`
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
	Name         string            `json:"name" yaml:"name"`
	Type         PluginType        `json:"type" yaml:"type"`
	Version      string            `json:"version" yaml:"version"`
	Description  string            `json:"description" yaml:"description"`
	Repository   string            `json:"repository" yaml:"repository"`
	ImportPath   string            `json:"import_path" yaml:"import_path"`
	Author       string            `json:"author" yaml:"author"`
	License      string            `json:"license" yaml:"license"`
	Dependencies []Dependency      `json:"dependencies" yaml:"dependencies"`
	Tags         []string          `json:"tags" yaml:"tags"`
	Compatible   string            `json:"compatible" yaml:"compatible"` // Compatible Lynx version
	Status       PluginStatus      `json:"status" yaml:"status"`
	InstalledVer string            `json:"installed_version,omitempty" yaml:"installed_version,omitempty"`
	ConfigFile   string            `json:"config_file,omitempty" yaml:"config_file,omitempty"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Official     bool              `json:"official" yaml:"official"`
	ExtraInfo    map[string]string `json:"extra_info,omitempty" yaml:"extra_info,omitempty"`
}

// PluginRegistry manages available plugins
type PluginRegistry struct {
	plugins map[string]*PluginMetadata
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	registry := &PluginRegistry{
		plugins: make(map[string]*PluginMetadata),
	}
	registry.loadOfficialPlugins()
	return registry
}

// loadOfficialPlugins loads the official plugin list
func (r *PluginRegistry) loadOfficialPlugins() {
	officialPlugins := []*PluginMetadata{
		// Service plugins
		{
			Name:        "http",
			Type:        TypeService,
			Version:     "v2.0.0",
			Description: "HTTP service plugin with middleware support",
			Repository:  "github.com/go-lynx/lynx/plugins/service/http",
			ImportPath:  "github.com/go-lynx/lynx/plugins/service/http",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"http", "rest", "api"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "grpc",
			Type:        TypeService,
			Version:     "v2.0.0",
			Description: "gRPC service plugin with interceptor support",
			Repository:  "github.com/go-lynx/lynx/plugins/service/grpc",
			ImportPath:  "github.com/go-lynx/lynx/plugins/service/grpc",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"grpc", "rpc", "protobuf"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "openim",
			Type:        TypeService,
			Version:     "v2.0.0",
			Description: "OpenIM instant messaging service plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/service/openim",
			ImportPath:  "github.com/go-lynx/lynx/plugins/service/openim",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"im", "chat", "messaging"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},

		// Message Queue plugins
		{
			Name:        "kafka",
			Type:        TypeMQ,
			Version:     "v2.0.0",
			Description: "Apache Kafka message queue plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/mq/kafka",
			ImportPath:  "github.com/go-lynx/lynx/plugins/mq/kafka",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"kafka", "streaming", "mq"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "rabbitmq",
			Type:        TypeMQ,
			Version:     "v2.0.0",
			Description: "RabbitMQ message queue plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/mq/rabbitmq",
			ImportPath:  "github.com/go-lynx/lynx/plugins/mq/rabbitmq",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"rabbitmq", "amqp", "mq"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "rocketmq",
			Type:        TypeMQ,
			Version:     "v2.0.0",
			Description: "Apache RocketMQ message queue plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/mq/rocketmq",
			ImportPath:  "github.com/go-lynx/lynx/plugins/mq/rocketmq",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"rocketmq", "mq"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "pulsar",
			Type:        TypeMQ,
			Version:     "v2.0.0",
			Description: "Apache Pulsar message queue plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/mq/pulsar",
			ImportPath:  "github.com/go-lynx/lynx/plugins/mq/pulsar",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"pulsar", "streaming", "mq"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},

		// SQL Database plugins
		{
			Name:        "mysql",
			Type:        TypeSQL,
			Version:     "v2.0.0",
			Description: "MySQL database plugin with connection pooling",
			Repository:  "github.com/go-lynx/lynx/plugins/sql/mysql",
			ImportPath:  "github.com/go-lynx/lynx/plugins/sql/mysql",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"mysql", "sql", "database"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "postgresql",
			Type:        TypeSQL,
			Version:     "v2.0.0",
			Description: "PostgreSQL database plugin with advanced features",
			Repository:  "github.com/go-lynx/lynx/plugins/sql/pgsql",
			ImportPath:  "github.com/go-lynx/lynx/plugins/sql/pgsql",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"postgresql", "pgsql", "sql", "database"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "mssql",
			Type:        TypeSQL,
			Version:     "v2.0.0",
			Description: "Microsoft SQL Server plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/sql/mssql",
			ImportPath:  "github.com/go-lynx/lynx/plugins/sql/mssql",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"mssql", "sqlserver", "sql", "database"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},

		// NoSQL Database plugins
		{
			Name:        "redis",
			Type:        TypeNoSQL,
			Version:     "v2.0.0",
			Description: "Redis cache and NoSQL database plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/nosql/redis",
			ImportPath:  "github.com/go-lynx/lynx/plugins/nosql/redis",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"redis", "cache", "nosql"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "mongodb",
			Type:        TypeNoSQL,
			Version:     "v2.0.0",
			Description: "MongoDB NoSQL database plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/nosql/mongodb",
			ImportPath:  "github.com/go-lynx/lynx/plugins/nosql/mongodb",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"mongodb", "nosql", "document"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "elasticsearch",
			Type:        TypeNoSQL,
			Version:     "v2.0.0",
			Description: "Elasticsearch search engine plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/nosql/elasticsearch",
			ImportPath:  "github.com/go-lynx/lynx/plugins/nosql/elasticsearch",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"elasticsearch", "search", "nosql"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},

		// Distributed Transaction plugins
		{
			Name:        "seata",
			Type:        TypeDTX,
			Version:     "v2.0.0",
			Description: "Seata distributed transaction plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/dtx/seata",
			ImportPath:  "github.com/go-lynx/lynx/plugins/dtx/seata",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"seata", "dtx", "transaction"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "dtm",
			Type:        TypeDTX,
			Version:     "v2.0.0",
			Description: "DTM distributed transaction plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/dtx/dtm",
			ImportPath:  "github.com/go-lynx/lynx/plugins/dtx/dtm",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"dtm", "dtx", "transaction"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},

		// Other plugins
		{
			Name:        "polaris",
			Type:        TypeOther,
			Version:     "v2.0.0",
			Description: "Polaris service discovery and governance plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/polaris",
			ImportPath:  "github.com/go-lynx/lynx/plugins/polaris",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"polaris", "discovery", "governance"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "tracer",
			Type:        TypeTracer,
			Version:     "v2.0.0",
			Description: "Distributed tracing plugin with OpenTelemetry",
			Repository:  "github.com/go-lynx/lynx/plugins/tracer",
			ImportPath:  "github.com/go-lynx/lynx/plugins/tracer",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"tracing", "opentelemetry", "observability"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
		{
			Name:        "swagger",
			Type:        TypeOther,
			Version:     "v2.0.0",
			Description: "Swagger API documentation generator plugin",
			Repository:  "github.com/go-lynx/lynx/plugins/swagger",
			ImportPath:  "github.com/go-lynx/lynx/plugins/swagger",
			Author:      "go-lynx",
			License:     "Apache-2.0",
			Tags:        []string{"swagger", "openapi", "documentation"},
			Compatible:  ">=v2.0.0",
			Official:    true,
		},
	}

	for _, plugin := range officialPlugins {
		r.plugins[plugin.Name] = plugin
	}
}

// GetPlugin returns a plugin by name
func (r *PluginRegistry) GetPlugin(name string) (*PluginMetadata, error) {
	plugin, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return plugin, nil
}

// GetAllPlugins returns all plugins
func (r *PluginRegistry) GetAllPlugins() []*PluginMetadata {
	plugins := make([]*PluginMetadata, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetPluginsByType returns plugins filtered by type
func (r *PluginRegistry) GetPluginsByType(pluginType PluginType) []*PluginMetadata {
	var plugins []*PluginMetadata
	for _, plugin := range r.plugins {
		if plugin.Type == pluginType {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// SearchPlugins searches plugins by keyword
func (r *PluginRegistry) SearchPlugins(keyword string) []*PluginMetadata {
	keyword = strings.ToLower(keyword)
	var plugins []*PluginMetadata
	
	for _, plugin := range r.plugins {
		// Search in name, description, and tags
		if strings.Contains(strings.ToLower(plugin.Name), keyword) ||
			strings.Contains(strings.ToLower(plugin.Description), keyword) {
			plugins = append(plugins, plugin)
			continue
		}
		
		// Search in tags
		for _, tag := range plugin.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				plugins = append(plugins, plugin)
				break
			}
		}
	}
	
	return plugins
}

// GetOfficialPlugins returns only official plugins
func (r *PluginRegistry) GetOfficialPlugins() []*PluginMetadata {
	var plugins []*PluginMetadata
	for _, plugin := range r.plugins {
		if plugin.Official {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// AddCustomPlugin adds a custom plugin to the registry
func (r *PluginRegistry) AddCustomPlugin(plugin *PluginMetadata) {
	plugin.Official = false
	r.plugins[plugin.Name] = plugin
}

// UpdatePluginStatus updates the status of a plugin
func (r *PluginRegistry) UpdatePluginStatus(name string, status PluginStatus, installedVersion string) error {
	plugin, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	plugin.Status = status
	plugin.InstalledVer = installedVersion
	return nil
}