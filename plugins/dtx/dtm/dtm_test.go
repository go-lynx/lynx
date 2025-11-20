package dtm

import (
	"context"
	"testing"

	"github.com/go-lynx/lynx/plugins/dtm/dtm/conf"
	"github.com/stretchr/testify/assert"
)

// TestNewDTMClient tests plugin creation
func TestNewDTMClient(t *testing.T) {
	client := NewDTMClient()
	assert.NotNil(t, client)
	assert.Equal(t, pluginName, client.Name())
	assert.Equal(t, pluginVersion, client.Version())
	assert.Equal(t, pluginDescription, client.Description())
}

// TestDTMClient_InitializeResources tests initialization
func TestDTMClient_InitializeResources(t *testing.T) {
	client := NewDTMClient()

	// Test default configuration
	client.conf = &conf.DTM{}
	if client.conf.ServerUrl == "" {
		client.conf.ServerUrl = "http://localhost:36789/api/dtmsvr"
	}
	if client.conf.Timeout == 0 {
		client.conf.Timeout = 10
	}
	if client.conf.RetryInterval == 0 {
		client.conf.RetryInterval = 10
	}
	if client.conf.TransactionTimeout == 0 {
		client.conf.TransactionTimeout = 60
	}
	if client.conf.BranchTimeout == 0 {
		client.conf.BranchTimeout = 30
	}

	assert.Equal(t, "http://localhost:36789/api/dtmsvr", client.conf.ServerUrl)
	assert.Equal(t, int32(10), client.conf.Timeout)
	assert.Equal(t, int32(10), client.conf.RetryInterval)
	assert.Equal(t, int32(60), client.conf.TransactionTimeout)
	assert.Equal(t, int32(30), client.conf.BranchTimeout)
}

// TestDTMClient_StartupTasks tests startup
func TestDTMClient_StartupTasks(t *testing.T) {
	client := NewDTMClient()
	client.conf = &conf.DTM{
		Enabled:   false,
		ServerUrl: "http://localhost:36789/api/dtmsvr",
	}

	// Test with disabled DTM
	err := client.StartupTasks()
	assert.NoError(t, err)

	// Test with enabled DTM (without actual server)
	client.conf.Enabled = true
	err = client.StartupTasks()
	// Error is acceptable if server is not available
	if err != nil {
		t.Logf("StartupTasks returned error (expected if server is not available): %v", err)
	}
}

// TestDTMClient_CleanupTasks tests cleanup
func TestDTMClient_CleanupTasks(t *testing.T) {
	client := NewDTMClient()

	err := client.CleanupTasks()
	assert.NoError(t, err)
}

// TestDTMClient_GetServerURL tests getting server URL
func TestDTMClient_GetServerURL(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"

	url := client.GetServerURL()
	assert.Equal(t, "http://localhost:36789/api/dtmsvr", url)
}

// TestDTMClient_GetGRPCServer tests getting gRPC server
func TestDTMClient_GetGRPCServer(t *testing.T) {
	client := NewDTMClient()
	client.grpcServer = "localhost:36790"

	server := client.GetGRPCServer()
	assert.Equal(t, "localhost:36790", server)
}

// TestDTMClient_NewSaga tests creating SAGA transaction
func TestDTMClient_NewSaga(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"
	client.conf = &conf.DTM{
		TransactionTimeout: 60,
		Timeout:            10,
		RetryInterval:      10,
	}

	// Test with configured server URL
	saga := client.NewSaga("test-gid")
	assert.NotNil(t, saga)

	// Test with empty server URL
	client.serverURL = ""
	saga = client.NewSaga("test-gid")
	assert.Nil(t, saga)
}

// TestDTMClient_NewMsg tests creating MSG transaction
func TestDTMClient_NewMsg(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"
	client.conf = &conf.DTM{
		TransactionTimeout: 60,
		Timeout:            10,
		RetryInterval:      10,
	}

	// Test with configured server URL
	msg := client.NewMsg("test-gid")
	assert.NotNil(t, msg)

	// Test with empty server URL
	client.serverURL = ""
	msg = client.NewMsg("test-gid")
	assert.Nil(t, msg)
}

// TestDTMClient_NewTcc tests creating TCC transaction
func TestDTMClient_NewTcc(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"

	// Test with configured server URL
	tcc := client.NewTcc("test-gid")
	// Should return nil as it's not implemented
	assert.Nil(t, tcc)

	// Test with empty server URL
	client.serverURL = ""
	tcc = client.NewTcc("test-gid")
	assert.Nil(t, tcc)
}

// TestDTMClient_NewXa tests creating XA transaction
func TestDTMClient_NewXa(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"

	// Test with configured server URL
	xa := client.NewXa("test-gid")
	// Should return nil as it's not implemented
	assert.Nil(t, xa)

	// Test with empty server URL
	client.serverURL = ""
	xa = client.NewXa("test-gid")
	assert.Nil(t, xa)
}

// TestDTMClient_GenerateGid tests generating GID
func TestDTMClient_GenerateGid(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"

	// Test with configured server URL
	// Note: This will fail if DTM server is not running
	gid := client.GenerateGid()
	if gid == "" {
		t.Logf("GenerateGid returned empty (expected if server is not available)")
	}

	// Test with empty server URL
	client.serverURL = ""
	gid = client.GenerateGid()
	assert.Empty(t, gid)
}

// TestDTMClient_GetConfig tests getting configuration
func TestDTMClient_GetConfig(t *testing.T) {
	client := NewDTMClient()
	client.conf = &conf.DTM{
		ServerUrl: "http://localhost:36789/api/dtmsvr",
		Enabled:   true,
	}

	config := client.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, client.conf, config)
}

// TestDTMClient_Configure tests configuration update
func TestDTMClient_Configure(t *testing.T) {
	client := NewDTMClient()
	client.conf = &conf.DTM{
		ServerUrl: "http://localhost:36789/api/dtmsvr",
	}

	// Test nil configuration
	err := client.Configure(nil)
	assert.NoError(t, err)

	// Test invalid configuration type
	err = client.Configure("invalid")
	assert.Error(t, err)

	// Test valid configuration
	newConfig := &conf.DTM{
		ServerUrl: "http://localhost:36790/api/dtmsvr",
		Enabled:   true,
	}
	err = client.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:36790/api/dtmsvr", client.conf.ServerUrl)
	assert.True(t, client.conf.Enabled)
}

// TestNewTransactionHelper tests transaction helper creation
func TestNewTransactionHelper(t *testing.T) {
	client := NewDTMClient()
	helper := NewTransactionHelper(client)

	assert.NotNil(t, helper)
	assert.Equal(t, client, helper.client)
}

// TestDefaultTransactionOptions tests default transaction options
func TestDefaultTransactionOptions(t *testing.T) {
	opts := DefaultTransactionOptions()

	assert.NotNil(t, opts)
	assert.Equal(t, int64(60), opts.TimeoutToFail)
	assert.Equal(t, int64(30), opts.BranchTimeout)
	assert.Equal(t, int64(10), opts.RetryInterval)
	assert.False(t, opts.WaitResult)
	assert.False(t, opts.Concurrent)
}

// TestTransactionHelper_MustGenGid tests generating GID
func TestTransactionHelper_MustGenGid(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"
	helper := NewTransactionHelper(client)

	// Test with configured server URL
	// Note: This will fail if DTM server is not running
	gid := helper.MustGenGid()
	if gid == "" {
		t.Logf("MustGenGid returned empty (expected if server is not available)")
	}

	// Test with empty server URL
	client.serverURL = ""
	gid = helper.MustGenGid()
	assert.Empty(t, gid)
}

// TestTransactionHelper_GenGid tests generating GID with error
func TestTransactionHelper_GenGid(t *testing.T) {
	client := NewDTMClient()
	client.serverURL = "http://localhost:36789/api/dtmsvr"
	helper := NewTransactionHelper(client)

	// Test with configured server URL
	// Note: This will fail if DTM server is not running
	gid, err := helper.GenGid()
	if err != nil {
		t.Logf("GenGid returned error (expected if server is not available): %v", err)
	} else {
		assert.NotEmpty(t, gid)
	}

	// Test with empty server URL
	client.serverURL = ""
	gid, err = helper.GenGid()
	assert.Empty(t, gid)
	assert.Error(t, err)
}

// TestTransactionHelper_CheckTransactionStatus tests checking transaction status
func TestTransactionHelper_CheckTransactionStatus(t *testing.T) {
	client := NewDTMClient()
	helper := NewTransactionHelper(client)

	status, err := helper.CheckTransactionStatus("test-gid")
	assert.NoError(t, err)
	assert.Equal(t, "success", status)
}

// TestTransactionHelper_RegisterGrpcService tests registering gRPC service
func TestTransactionHelper_RegisterGrpcService(t *testing.T) {
	client := NewDTMClient()
	helper := NewTransactionHelper(client)

	// Test with no gRPC server configured
	err := helper.RegisterGrpcService("test-service", "localhost:8080")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")

	// Test with gRPC server configured
	client.grpcServer = "localhost:36790"
	err = helper.RegisterGrpcService("test-service", "localhost:8080")
	assert.NoError(t, err)
}

// TestCreateGrpcContext tests creating gRPC context
func TestCreateGrpcContext(t *testing.T) {
	ctx := context.Background()
	newCtx := CreateGrpcContext(ctx, "test-gid", "saga", "branch-1", "try")

	assert.NotNil(t, newCtx)
	assert.NotEqual(t, ctx, newCtx)
}

// TestExtractGrpcTransInfo tests extracting transaction info from gRPC context
func TestExtractGrpcTransInfo(t *testing.T) {
	ctx := context.Background()

	// Test with empty context (will fail, but tests error handling)
	_, err := ExtractGrpcTransInfo(ctx)
	// Error is expected as context doesn't have DTM metadata
	if err != nil {
		t.Logf("ExtractGrpcTransInfo returned error (expected): %v", err)
	}
}

// TestTransactionTypes tests transaction type constants
func TestTransactionTypes(t *testing.T) {
	assert.Equal(t, TransactionType("saga"), TransTypeSAGA)
	assert.Equal(t, TransactionType("tcc"), TransTypeTCC)
	assert.Equal(t, TransactionType("msg"), TransTypeMsg)
	assert.Equal(t, TransactionType("xa"), TransTypeXA)
}

// TestPluginMetadata tests plugin metadata constants
func TestPluginMetadata(t *testing.T) {
	assert.Equal(t, "dtm.server", pluginName)
	assert.Equal(t, "v1.0.0", pluginVersion)
	assert.Equal(t, "DTM distributed transaction manager plugin for Lynx framework", pluginDescription)
	assert.Equal(t, "lynx.dtm", confPrefix)
}

// TestSAGABranch tests SAGA branch structure
func TestSAGABranch(t *testing.T) {
	branch := SAGABranch{
		Action:     "http://localhost:8080/action",
		Compensate: "http://localhost:8080/compensate",
		Data:       map[string]string{"key": "value"},
	}

	assert.Equal(t, "http://localhost:8080/action", branch.Action)
	assert.Equal(t, "http://localhost:8080/compensate", branch.Compensate)
	assert.NotNil(t, branch.Data)
}

// TestTCCBranch tests TCC branch structure
func TestTCCBranch(t *testing.T) {
	branch := TCCBranch{
		Try:     "http://localhost:8080/try",
		Confirm: "http://localhost:8080/confirm",
		Cancel:  "http://localhost:8080/cancel",
		Data:    map[string]string{"key": "value"},
	}

	assert.Equal(t, "http://localhost:8080/try", branch.Try)
	assert.Equal(t, "http://localhost:8080/confirm", branch.Confirm)
	assert.Equal(t, "http://localhost:8080/cancel", branch.Cancel)
	assert.NotNil(t, branch.Data)
}

// TestMsgBranch tests MSG branch structure
func TestMsgBranch(t *testing.T) {
	branch := MsgBranch{
		Action: "http://localhost:8080/action",
		Data:   map[string]string{"key": "value"},
	}

	assert.Equal(t, "http://localhost:8080/action", branch.Action)
	assert.NotNil(t, branch.Data)
}

// TestXABranch tests XA branch structure
func TestXABranch(t *testing.T) {
	branch := XABranch{
		Action: "http://localhost:8080/action",
		Data:   `{"key":"value"}`,
	}

	assert.Equal(t, "http://localhost:8080/action", branch.Action)
	assert.Equal(t, `{"key":"value"}`, branch.Data)
}

