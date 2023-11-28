package boot

import (
	"flag"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/conf"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

var (
	flagConf string
)

type Boot struct {
	wire wireApp
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
	flag.Parse()
}

type wireApp func(confServer *conf.Lynx, logger log.Logger) (*kratos.App, error)

// Run This function is the application startup entry point
func (b *Boot) Run() {
	st := time.Now()

	c := localBootFileLoad()
	a := app.NewApp(c)
	logger := app.InitLogger()
	app.GetHelper().Infof("Lynx application is starting up")

	// Load the plugin first, then execute the wireApp
	a.PlugManager().LoadPlugins()
	k, err := b.wire(c, logger)
	if err != nil {
		app.GetHelper().Error(err)
		panic(err)
	}

	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	app.GetHelper().Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)
	defer a.PlugManager().UnloadPlugins()

	// start and wait for stop signal
	if err := k.Run(); err != nil {
		app.GetHelper().Error(err)
		panic(err)
	}
}

// LynxApplication Create a Lynx microservice bootstrap program
func LynxApplication(wire wireApp) *Boot {
	return &Boot{wire: wire}
}
