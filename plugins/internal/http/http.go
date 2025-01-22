package http

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugin-http/v2/conf"
)

// Plugin metadata
const (
	pluginName        = "http.server"
	pluginVersion     = "v2.0.0"
	pluginDescription = "HTTP server plugin for Lynx framework"
	confPrefix        = "lynx.http"
)

// ServiceHttp implements the HTTP plugin functionality
type ServiceHttp struct {
	*plugins.BasePlugin
	conf   *conf.Http
	server *http.Server
}

// NewServiceHttp creates a new HTTP plugin instance
func NewServiceHttp() *ServiceHttp {
	return &ServiceHttp{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
		),
	}
}

// InitializeResources implements custom initialization for HTTP plugin
func (h *ServiceHttp) InitializeResources(rt plugins.Runtime) error {
	// Add default configuration if not provided
	if h.conf == nil {
		h.conf = &conf.Http{
			Network: "tcp",
			Addr:    ":8080",
		}
	}
	err := rt.GetConfig().Scan(h.conf)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks implements custom startup logic for HTTP plugin
func (h *ServiceHttp) StartupTasks() error {
	app.Lynx().GetLogHelper().Infof("Starting HTTP service")

	opts := []http.ServerOption{
		http.Middleware(
			tracing.Server(tracing.WithTracerName(app.GetName())),
			logging.Server(app.Lynx().GetLogger()),
			app.Lynx().GetControlPlane().HTTPRateLimit(),
			validate.Validator(),
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					return nil
				}),
			),
			ResponsePack(),
		),
		http.ResponseEncoder(ResponseEncoder),
	}

	if h.conf.Network != "" {
		opts = append(opts, http.Network(h.conf.Network))
	}
	if h.conf.Addr != "" {
		opts = append(opts, http.Address(h.conf.Addr))
	}
	if h.conf.Timeout != nil {
		opts = append(opts, http.Timeout(h.conf.Timeout.AsDuration()))
	}
	if h.conf.GetTls() {
		opts = append(opts, h.tlsLoad())
	}

	h.server = http.NewServer(opts...)
	app.Lynx().GetLogHelper().Infof("HTTP service successfully started")
	return nil
}

// CleanupTasks implements custom cleanup logic for HTTP plugin
func (h *ServiceHttp) CleanupTasks() error {
	if h.server != nil {
		h.EmitEvent(plugins.PluginEvent{
			Type:     plugins.EventPluginStopping,
			Priority: plugins.PriorityHigh,
			Source:   "Stop",
			Category: "lifecycle",
		})

		if err := h.server.Stop(context.Background()); err != nil {
			return plugins.NewPluginError(h.ID(), "Stop", "Failed to stop HTTP server", err)
		}

		h.EmitEvent(plugins.PluginEvent{
			Type:     plugins.EventPluginStopped,
			Priority: plugins.PriorityNormal,
			Source:   "Stop",
			Category: "lifecycle",
		})
	}

	return h.BasePlugin.Stop()
}

// Configure updates the HTTP server configuration
func (h *ServiceHttp) Configure(c any) error {
	if httpConf, ok := c.(*conf.Http); ok {
		h.conf = httpConf
		return nil
	}
	return plugins.ErrInvalidConfiguration
}

// CheckHealth performs a health check of the HTTP server
func (h *ServiceHttp) CheckHealth(report *plugins.HealthReport) error {
	return nil
}
