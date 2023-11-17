package boot

import (
	"flag"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugin"
	"google.golang.org/protobuf/encoding/protojson"
	"os"
	"time"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// xxName is the name of the compiled software.
	name string
	// Version is the version of the compiled software.
	version string
	// flagConf is the config flag.
	flagConf string
	// id The IP address of the current application
	id, _ = os.Hostname()
)

func GetName() string {
	return name
}

func GetVersion() string {
	return version
}

func GetHostname() string {
	return id
}

type wireApp func(confServer *conf.Bootstrap, logger log.Logger) (*kratos.App, error)

type App struct {
	p []plugin.Plugin
	wireApp
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

func NewApp(w wireApp, p ...plugin.Plugin) *App {
	return &App{
		p:       p,
		wireApp: w,
	}
}

// Run This function is the application startup entry point
func (a *App) Run() {
	flag.Parse()
	st := time.Now()

	log.Infof("Lynx reading local bootstrap configuration file/folder:%v", flagConf)
	var bc conf.Bootstrap
	configLoad(&bc)

	logger := InitLogger()
	dfLog.Infof("Lynx application is starting up")

	a.initPolaris(&bc)
	a.loadingPlugin(&bc)

	app, err := a.wireApp(&bc, logger)
	if err != nil {
		dfLog.Error(err)
		panic(err)
	}

	defer a.cleanPlugin()
	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	dfLog.Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		dfLog.Error(err)
		panic(err)
	}
}
