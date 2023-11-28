package boot

import (
	"flag"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugin"
	"google.golang.org/protobuf/encoding/protojson"
	"os"
	"time"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	app      *App
	flagConf string
)

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	flag.Parse()
}

func LynxApp() *App {
	return app
}

type App struct {
	host        string
	name        string
	version     string
	wire        wireApp
	plugManager LynxPluginManager
}

type wireApp func(confServer *Lynx, logger log.Logger) (*kratos.App, error)

// Run This function is the application startup entry point
func (a *App) Run() {
	st := time.Now()

	log.Infof("Lynx reading local bootstrap configuration file/folder:%v", flagConf)
	lynx := localBootFileLoad()

	// set lynx microservice name and version
	a.name = lynx.Application.Name
	a.version = lynx.Application.Version
	a.host, _ = os.Hostname()

	log.Infof("Lynx Log component loading")
	logger := InitLogger()

	// Load the plugin first, then execute the wireApp
	dfLog.Infof("Lynx application is starting up")
	a.plugManager.LoadPlugins()

	app, err := a.wire(lynx, logger)
	if err != nil {
		dfLog.Error(err)
		panic(err)
	}

	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	dfLog.Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)
	defer a.plugManager.UnloadPlugins()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		dfLog.Error(err)
		panic(err)
	}
}

// NewApp create a lynx microservice
func NewApp(w wireApp, p ...plugin.Plugin) *App {
	a := &App{
		wire: w,
	}

	// Manually load the plugins
	if p != nil && len(p) > 0 {
		a.plugManager.Init(p...)
	}

	// The app is in Singleton pattern
	app = a
	return a
}
