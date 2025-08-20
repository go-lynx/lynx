package boot

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	lynxapp "github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// flagConf holds the configuration file path from command line arguments
var (
	flagConf string
)

// Application represents the main bootstrap structure for Lynx applications, responsible for managing application initialization, configuration loading, and lifecycle
type Application struct {
	wire    wireApp          // Function used to initialize Kratos application
	plugins []plugins.Plugin // List of plugins to initialize
	conf    config.Config    // Application configuration instance
	cleanup func()           // Cleanup function for resource cleanup when application shuts down
	lynxApp *lynxapp.LynxApp // Lynx application instance
}

// init package initialization function for parsing command line arguments and configuring JSON serialization options
func init() {
	// Only parse command line arguments in non-test environments
	if !isTestEnvironment() {
		// Use configuration manager to get default configuration path
		configMgr := GetConfigManager()
		defaultConfPath := configMgr.GetDefaultConfigPath()
		flag.StringVar(&flagConf, "conf", defaultConfPath, "config path, eg: -conf config.yaml")
		flag.Parse()

		// Set parsed configuration path to configuration manager
		configMgr.SetConfigPath(flagConf)
	}
}

// isTestEnvironment checks if running in test environment
func isTestEnvironment() bool {
	return flag.Lookup("test.v") != nil || flag.Lookup("test.run") != nil
}

// wireApp is a function type used to initialize and return a Kratos application instance
type wireApp func(logger kratoslog.Logger) (*kratos.App, error)

// Run starts the Lynx application and manages its lifecycle
func (app *Application) Run() error {
	// Check if Application instance is nil
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot start Lynx application")
	}

	// Improved resource cleanup order: handle panic first, then execute cleanup
	defer func() {
		if r := recover(); r != nil {
			app.handlePanic(r)
		}
		if app.cleanup != nil {
			app.cleanup()
		}
	}()

	// Record application startup time for calculating startup duration
	startTime := time.Now()

	// Load bootstrap configuration
	if err := app.LoadBootstrapConfig(); err != nil {
		return fmt.Errorf("failed to load bootstrap configuration: %w", err)
	}

	// Initialize Lynx application
	lynxApp, err := lynxapp.NewApp(app.conf, app.plugins...)
	if err != nil {
		return fmt.Errorf("failed to create Lynx application: %w", err)
	}
	app.lynxApp = lynxApp

	// Initialize logger
	if err := log.InitLogger(app.GetName(), app.GetHost(), app.GetVersion(), app.conf); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Log application startup information
	log.Info("lynx application is starting up")

	// Get plugin manager
	pluginManager := lynxApp.GetPluginManager()
	if pluginManager == nil {
		return fmt.Errorf("plugin manager is nil: cannot manage plugins")
	}

	// Load plugins
	err = pluginManager.LoadPlugins(app.conf)
	if err != nil {
		return err
	}

	// Initialize Kratos application
	kratosApp, err := app.wire(log.Logger)
	if err != nil {
		log.Error(err)
		return fmt.Errorf("failed to initialize Kratos application: %w", err)
	}

	// Configure protocol buffers JSON serialization options
	jsonEmit, jsonConfErr := lynxApp.GetGlobalConfig().Value("lynx.http.response.json.emitUnpopulated").Bool()
	if jsonConfErr != nil && errors.Is(jsonConfErr, config.ErrNotFound) {
		jsonEmit = false
	}
	// EmitUnpopulated: include unset fields during serialization
	// UseProtoNames: use field names defined in proto files
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: jsonEmit,
		UseProtoNames:   true,
	}

	// Calculate application startup duration
	elapsedMs := time.Since(startTime).Milliseconds()
	var elapsedDisplay string
	switch {
	case elapsedMs < 1000:
		// Less than 1 second, display milliseconds
		elapsedDisplay = fmt.Sprintf("%d ms", elapsedMs)
	case elapsedMs < 60_000:
		// Less than 1 minute, display seconds (keep two decimal places)
		elapsedDisplay = fmt.Sprintf("%.2f s", float64(elapsedMs)/1000)
	default:
		// More than 1 minute, display minutes (keep two decimal places)
		elapsedDisplay = fmt.Sprintf("%.2f m", float64(elapsedMs)/1000/60)
	}
	log.Infof("lynx application started successfully, elapsed time: %s, port listening initiated", elapsedDisplay)

	// Run Kratos application
	if err := kratosApp.Run(); err != nil {
		log.Error(err)
		return fmt.Errorf("failed to run Kratos application: %w", err)
	}

	return nil
}

// handlePanic recovers from panic and ensures proper resource cleanup
func (app *Application) handlePanic(r interface{}) {
	var err error
	// Convert panic to error based on its type
	switch v := r.(type) {
	case error:
		err = v
	case string:
		err = fmt.Errorf("panic: %s", v)
	default:
		err = fmt.Errorf("panic: %v", r)
	}
	log.Error(err)

	// Ensure plugins are unloaded
	if app.lynxApp != nil && app.lynxApp.GetPluginManager() != nil {
		app.lynxApp.GetPluginManager().UnloadPlugins()
	}
}

// NewApplication creates a new Lynx microservice bootstrap instance
// Parameters:
//   - wire: Function used to initialize Kratos application
//   - plugins: Optional plugin list to initialize with the application
//
// Returns:
//   - *Application: Initialized Application instance
func NewApplication(wire wireApp, plugins ...plugins.Plugin) *Application {
	// Check if wire function is nil
	if wire == nil {
		log.Error("wire function cannot be nil: required for Kratos application initialization")
		return nil
	}

	// Return initialized Application instance
	return &Application{
		wire:    wire,
		plugins: plugins,
	}
}
