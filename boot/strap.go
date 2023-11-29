package boot

import (
	"flag"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugin"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

var (
	flagConf string
)

type Boot struct {
	wire    wireApp
	plugins []plugin.Plugin
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.Parse()
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

type wireApp func(confServer *conf.Lynx, logger log.Logger) (*kratos.App, error)

// Run This function is the application startup entry point
func (b *Boot) Run() {
	st := time.Now()

	c := localBootFileLoad()
	app.NewApp(c, b.plugins...)
	app.Lynx().InitLogger()

	app.Lynx().GetHelper().Infof("Lynx application is starting up")
	// Load the plugin first, then execute the wireApp
	app.Lynx().PlugManager().LoadPlugins()
	k, err := b.wire(c, app.Lynx().GetLogger())
	if err != nil {
		app.Lynx().GetHelper().Error(err)
		panic(err)
	}
	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	app.Lynx().GetHelper().Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)
	defer app.Lynx().PlugManager().UnloadPlugins()

	// kratos start and wait for stop signal
	if err := k.Run(); err != nil {
		app.Lynx().GetHelper().Error(err)
		panic(err)
	}
}

// LynxApplication Create a Lynx microservice bootstrap program
func LynxApplication(wire wireApp, p ...plugin.Plugin) *Boot {
	return &Boot{
		wire:    wire,
		plugins: p,
	}
}
