package mongodb

import (
	"sync"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/mongodb/conf"
	"go.mongodb.org/mongo-driver/mongo"
)

// PlugMongoDB represents a MongoDB plugin instance
type PlugMongoDB struct {
	// Inherits from base plugin
	*plugins.BasePlugin
	// MongoDB configuration
	conf *conf.MongoDB
	// MongoDB client instance
	client *mongo.Client
	// MongoDB database instance
	database *mongo.Database
	// Metrics collection
	statsQuit     chan struct{}
	statsWG       sync.WaitGroup
	statsClosed   bool
	statsMu       sync.Mutex
	metricsCancel func()
	healthCancel  func()
}
