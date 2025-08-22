package dtm

import (
	"context"
	"fmt"
	"time"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/dtm/dtm/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Plugin metadata
const (
	// pluginName is the unique identifier for the DTM plugin
	pluginName = "dtm.server"

	// pluginVersion indicates the current version of the DTM plugin
	pluginVersion = "v1.0.0"

	// pluginDescription briefly describes the functionality of the DTM plugin
	pluginDescription = "DTM distributed transaction manager plugin for Lynx framework"

	// confPrefix is the configuration prefix used when loading DTM configuration
	confPrefix = "lynx.dtm"
)

// DTMClient represents the DTM client plugin
type DTMClient struct {
	// Embed base plugin, inherit common properties and methods of the plugin
	*plugins.BasePlugin
	// DTM configuration information
	conf *conf.DTM
	// HTTP client for DTM
	httpClient *dtmcli.DTM
	// gRPC connection for DTM
	grpcConn *grpc.ClientConn
	// gRPC client for DTM
	grpcClient dtmgrpc.DtmClient
}

// NewDTMClient creates a new DTM plugin instance
func NewDTMClient() *DTMClient {
	return &DTMClient{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			90,
		),
	}
}

// InitializeResources method is used to load and initialize the DTM plugin
func (d *DTMClient) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	d.conf = &conf.DTM{}

	// Scan and load DTM configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(d.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	if d.conf.ServerUrl == "" {
		d.conf.ServerUrl = "http://localhost:36789/api/dtmsvr"
	}
	if d.conf.Timeout == 0 {
		d.conf.Timeout = 10
	}
	if d.conf.RetryInterval == 0 {
		d.conf.RetryInterval = 10
	}
	if d.conf.TransactionTimeout == 0 {
		d.conf.TransactionTimeout = 60
	}
	if d.conf.BranchTimeout == 0 {
		d.conf.BranchTimeout = 30
	}

	return nil
}

// StartupTasks starts the DTM client
func (d *DTMClient) StartupTasks() error {
	log.Infof("Initializing DTM client")

	if !d.conf.GetEnabled() {
		log.Infof("DTM client is disabled")
		return nil
	}

	// Initialize HTTP client
	if d.conf.ServerUrl != "" {
		d.httpClient = dtmcli.New(d.conf.ServerUrl, "")
		// Set timeout
		d.httpClient.Timeout = time.Duration(d.conf.Timeout) * time.Second
		// Set retry interval
		d.httpClient.RetryInterval = time.Duration(d.conf.RetryInterval) * time.Second
		// Set pass through headers
		if len(d.conf.PassThroughHeaders) > 0 {
			d.httpClient.PassthroughHeaders = d.conf.PassThroughHeaders
		}
		log.Infof("DTM HTTP client initialized with server: %s", d.conf.ServerUrl)
	}

	// Initialize gRPC client if configured
	if d.conf.GrpcServer != "" {
		var err error
		d.grpcConn, err = grpc.Dial(d.conf.GrpcServer,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024)),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to DTM gRPC server: %w", err)
		}
		d.grpcClient = dtmgrpc.NewDtmClient(d.grpcConn)
		log.Infof("DTM gRPC client initialized with server: %s", d.conf.GrpcServer)
	}

	log.Infof("DTM client successfully initialized")
	return nil
}

// CleanupTasks cleans up DTM client resources
func (d *DTMClient) CleanupTasks() error {
	if d.grpcConn != nil {
		if err := d.grpcConn.Close(); err != nil {
			log.Errorf("Failed to close gRPC connection: %v", err)
			return err
		}
	}
	return nil
}

// GetHTTPClient returns the HTTP client for DTM
func (d *DTMClient) GetHTTPClient() *dtmcli.DTM {
	return d.httpClient
}

// GetGRPCClient returns the gRPC client for DTM
func (d *DTMClient) GetGRPCClient() dtmgrpc.DtmClient {
	return d.grpcClient
}

// NewSaga creates a new SAGA transaction
func (d *DTMClient) NewSaga(gid string) *dtmcli.Saga {
	if d.httpClient == nil {
		log.Errorf("DTM HTTP client is not initialized")
		return nil
	}
	saga := d.httpClient.NewSaga(gid)
	saga.TimeoutToFail = int64(d.conf.TransactionTimeout)
	saga.BranchTimeout = int64(d.conf.BranchTimeout)
	return saga
}

// NewMsg creates a new 2-phase message transaction
func (d *DTMClient) NewMsg(gid string) *dtmcli.Msg {
	if d.httpClient == nil {
		log.Errorf("DTM HTTP client is not initialized")
		return nil
	}
	msg := d.httpClient.NewMsg(gid)
	msg.TimeoutToFail = int64(d.conf.TransactionTimeout)
	msg.BranchTimeout = int64(d.conf.BranchTimeout)
	return msg
}

// NewTcc creates a new TCC transaction
func (d *DTMClient) NewTcc(gid string) *dtmcli.Tcc {
	if d.httpClient == nil {
		log.Errorf("DTM HTTP client is not initialized")
		return nil
	}
	tcc := d.httpClient.NewTcc(gid)
	tcc.TimeoutToFail = int64(d.conf.TransactionTimeout)
	tcc.BranchTimeout = int64(d.conf.BranchTimeout)
	return tcc
}

// NewXa creates a new XA transaction
func (d *DTMClient) NewXa(gid string) *dtmcli.Xa {
	if d.httpClient == nil {
		log.Errorf("DTM HTTP client is not initialized")
		return nil
	}
	xa := d.httpClient.NewXa(gid)
	xa.TimeoutToFail = int64(d.conf.TransactionTimeout)
	xa.BranchTimeout = int64(d.conf.BranchTimeout)
	return xa
}

// GenerateGid generates a new global transaction ID
func (d *DTMClient) GenerateGid() string {
	if d.httpClient == nil {
		log.Errorf("DTM HTTP client is not initialized")
		return ""
	}
	return d.httpClient.MustGenGid()
}

// CallBranch calls a branch transaction
func (d *DTMClient) CallBranch(ctx context.Context, body interface{}, tryURL string, confirmURL string, cancelURL string) (*dtmcli.BranchBarrier, error) {
	if d.httpClient == nil {
		return nil, fmt.Errorf("DTM HTTP client is not initialized")
	}
	
	// Create a branch barrier for handling idempotency
	bb, err := dtmcli.BarrierFromQuery(nil)
	if err != nil {
		return nil, err
	}
	
	return bb, nil
}

// GetConfig returns the DTM configuration
func (d *DTMClient) GetConfig() *conf.DTM {
	return d.conf
}
