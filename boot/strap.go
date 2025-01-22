package boot

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	// flagConf holds the configuration file path from command line arguments
	flagConf string
)

// Boot represents the main bootstrap structure for the Lynx application
type Boot struct {
	wire    wireApp
	plugins []plugins.Plugin
	conf    config.Config
	cleanup func()
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.Parse()

	// Configure JSON marshaling options for protocol buffers
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

// wireApp is a function type that initializes and returns a Kratos application
type wireApp func(logger log.Logger) (*kratos.App, error)

// Run starts the Lynx application and manages its lifecycle
func (b *Boot) Run() error {
	if b == nil {
		return fmt.Errorf("boot instance is nil")
	}

	// Defer panic handler and cleanup
	defer b.handlePanic()
	if b.cleanup != nil {
		defer b.cleanup()
	}

	// Record start time for startup duration calculation
	startTime := time.Now()

	// Load bootstrap configuration
	if err := b.LoadLocalBootstrapConfig(); err != nil {
		return fmt.Errorf("failed to load bootstrap configuration: %w", err)
	}

	// Initialize Lynx application
	lynxApp, err := app.NewApp(b.conf, b.plugins...)
	if err != nil {
		return fmt.Errorf("failed to create Lynx application: %w", err)
	}

	// Initialize logger
	if err := lynxApp.InitLogger(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	helper := lynxApp.GetLogHelper()
	if helper == nil {
		return fmt.Errorf("log helper is nil")
	}

	helper.Info("Lynx application is starting up")

	// Prepare and load plugins
	pluginManager := lynxApp.GetPluginManager()
	if pluginManager == nil {
		return fmt.Errorf("plugin manager is nil")
	}

	// load plugins
	pluginManager.LoadPlugins(b.conf)

	// Initialize Kratos application
	kratosApp, err := b.wire(lynxApp.GetLogger())
	if err != nil {
		helper.Error(err)
		return fmt.Errorf("failed to initialize Kratos application: %w", err)
	}

	// Calculate startup duration
	elapsedMs := time.Since(startTime).Milliseconds()
	helper.Infof("Lynx application started successfully, elapsed time: %d ms, port listening initiated", elapsedMs)

	// Run Kratos application
	if err := kratosApp.Run(); err != nil {
		helper.Error(err)
		return fmt.Errorf("failed to run Kratos application: %w", err)
	}

	return nil
}

// handlePanic recovers from panics and ensures proper cleanup
func (b *Boot) handlePanic() {
	if r := recover(); r != nil {
		var err error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = fmt.Errorf(v)
		default:
			err = fmt.Errorf("%v", r)
		}

		// Log the error using the appropriate logger
		lynxApp := app.Lynx()
		if lynxApp != nil {
			if helper := lynxApp.GetLogHelper(); helper != nil {
				helper.Error(err)
			} else {
				log.Error(err)
			}
		} else {
			log.Error(err)
		}

		// Ensure plugins are unloaded
		if lynxApp != nil && lynxApp.GetPluginManager() != nil {
			lynxApp.GetPluginManager().UnloadPlugins()
		}
	}
}

// NewLynxApplication creates a new Lynx microservice bootstrap program
// Parameters:
//   - wire: Function to initialize Kratos application
//   - plugins: Optional list of plugins to initialize with
//
// Returns:
//   - *Boot: Initialized Boot instance
func NewLynxApplication(wire wireApp, plugins ...plugins.Plugin) *Boot {
	if wire == nil {
		log.Error("wire function cannot be nil")
		return nil
	}

	return &Boot{
		wire:    wire,
		plugins: plugins,
	}
}
