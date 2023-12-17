package boot

import (
	"flag"
	"fmt"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
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
	conf    config.Config
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.Parse()
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

type wireApp func(confServer *conf.Bootstrap, logger log.Logger) (*kratos.App, error)

// Run This function is the application startup entry point
func (b *Boot) Run() {
	defer b.handlePanic()
	st := time.Now()

	c := b.loadLocalBootFile()
	app.NewApp(b.conf, b.plugins...)
	app.Lynx().InitLogger()
	app.Lynx().Helper().Infof("Lynx application is starting up")
	app.Lynx().PlugManager().PreparePlug(b.conf)

	// Load the plugin first, then execute the wireApp
	app.Lynx().PlugManager().LoadPlugins(b.conf)
	k, err := b.wire(c, app.Lynx().Logger())
	if err != nil {
		app.Lynx().Helper().Error(err)
		panic(err)
	}

	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	app.Lynx().Helper().Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)

	// kratos start and wait for stop signal
	if err := k.Run(); err != nil {
		app.Lynx().Helper().Error(err)
		panic(err)
	}
}

func (b *Boot) handlePanic() {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		// If Lynx helper is initialized, log with it, otherwise use standard log package
		if helper := app.Lynx().Helper(); helper != nil {
			helper.Error(err)
		} else {
			log.Error(err)
		}
	}

	// Unload plugins whether there was a panic or not
	if app.Lynx() != nil && app.Lynx().PlugManager() != nil {
		app.Lynx().PlugManager().UnloadPlugins()
	}
}

// LynxApplication Create a Lynx microservice bootstrap program
func LynxApplication(wire wireApp, p ...plugin.Plugin) *Boot {
	return &Boot{
		wire:    wire,
		plugins: p,
	}
}
