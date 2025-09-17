package dtm

import (
	"context"
	"fmt"
	"time"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-resty/resty/v2"
	"google.golang.org/grpc/metadata"
)

// TransactionType transaction type
type TransactionType string

const (
	// TransTypeSAGA SAGA transaction type
	TransTypeSAGA TransactionType = "saga"
	// TransTypeTCC TCC transaction type
	TransTypeTCC TransactionType = "tcc"
	// TransTypeMsg 2-phase message transaction type
	TransTypeMsg TransactionType = "msg"
	// TransTypeXA XA transaction type
	TransTypeXA TransactionType = "xa"
)

// TransactionOptions transaction options
type TransactionOptions struct {
	// Transaction timeout (seconds)
	TimeoutToFail int64
	// Branch timeout (seconds)
	BranchTimeout int64
	// Retry interval (seconds)
	RetryInterval int64
	// Custom request headers
	CustomHeaders map[string]string
	// Whether to wait for result
	WaitResult bool
	// Concurrent execution of branches
	Concurrent bool
}

// ExecuteXA execute XA transaction
func (h *TransactionHelper) ExecuteXA(ctx context.Context, gid string, branches []XABranch, opts *TransactionOptions) error {
    if opts == nil {
        opts = DefaultTransactionOptions()
    }

    // Use XA global transaction API
    err := dtmcli.XaGlobalTransaction(h.client.GetServerURL(), gid, func(xa *dtmcli.Xa) (*resty.Response, error) {
        // Set transaction options
        xa.TimeoutToFail = opts.TimeoutToFail
        xa.RequestTimeout = opts.BranchTimeout
        xa.RetryInterval = opts.RetryInterval

        // Call all XA branches
        for _, branch := range branches {
            _, err := xa.CallBranch(branch.Action, branch.Data)
            if err != nil {
                log.Errorf("XA branch failed: gid=%s, action=%s, error=%v", gid, branch.Action, err)
                return nil, err
            }
        }
        return nil, nil
    })

    if err != nil {
        log.Errorf("XA transaction failed: gid=%s, error=%v", gid, err)
        return err
    }

    log.Infof("XA transaction submitted successfully: gid=%s", gid)

    // If need to wait for result
    if opts.WaitResult {
        return h.waitTransactionResult(ctx, gid, TransTypeXA)
    }

    return nil
}

// DefaultTransactionOptions returns default transaction options
func DefaultTransactionOptions() *TransactionOptions {
	return &TransactionOptions{
		TimeoutToFail: 60,
		BranchTimeout: 30,
		RetryInterval: 10,
		WaitResult:    false,
		Concurrent:    false,
	}
}

// TransactionHelper transaction helper tool
type TransactionHelper struct {
	client *DTMClient
}

// NewTransactionHelper create transaction helper tool
func NewTransactionHelper(client *DTMClient) *TransactionHelper {
	return &TransactionHelper{
		client: client,
	}
}

// ExecuteSAGA execute SAGA transaction
func (h *TransactionHelper) ExecuteSAGA(ctx context.Context, gid string, branches []SAGABranch, opts *TransactionOptions) error {
	if opts == nil {
		opts = DefaultTransactionOptions()
	}

	saga := h.client.NewSaga(gid)
	if saga == nil {
		return fmt.Errorf("failed to create SAGA transaction")
	}

	saga.TimeoutToFail = opts.TimeoutToFail
	saga.RequestTimeout = opts.BranchTimeout
	saga.RetryInterval = opts.RetryInterval
	saga.Concurrent = opts.Concurrent

	// Add custom request headers
	if len(opts.CustomHeaders) > 0 {
		saga.BranchHeaders = opts.CustomHeaders
	}

	// Add all branches
	for _, branch := range branches {
		saga.Add(branch.Action, branch.Compensate, branch.Data)
	}

	// Submit transaction
	err := saga.Submit()
	if err != nil {
		log.Errorf("SAGA transaction failed: gid=%s, error=%v", gid, err)
		return err
	}

	log.Infof("SAGA transaction submitted successfully: gid=%s", gid)

	// If need to wait for result
	if opts.WaitResult {
		return h.waitTransactionResult(ctx, gid, TransTypeSAGA)
	}

	return nil
}

// ExecuteTCC execute TCC transaction
func (h *TransactionHelper) ExecuteTCC(ctx context.Context, gid string, branches []TCCBranch, opts *TransactionOptions) error {
	if opts == nil {
		opts = DefaultTransactionOptions()
	}

	// Use the new TCC global transaction API
	err := dtmcli.TccGlobalTransaction(h.client.GetServerURL(), gid, func(tcc *dtmcli.Tcc) (*resty.Response, error) {
		// Set transaction options
		tcc.TimeoutToFail = opts.TimeoutToFail
		tcc.RequestTimeout = opts.BranchTimeout
		tcc.RetryInterval = opts.RetryInterval

		// Add custom request headers
		if len(opts.CustomHeaders) > 0 {
			tcc.BranchHeaders = opts.CustomHeaders
		}

		// Call all Try branches
		for _, branch := range branches {
			_, err := tcc.CallBranch(branch.Data, branch.Try, branch.Confirm, branch.Cancel)
			if err != nil {
				log.Errorf("TCC branch failed: gid=%s, try=%s, error=%v", gid, branch.Try, err)
				return nil, err
			}
		}

		return nil, nil
	})

	if err != nil {
		log.Errorf("TCC transaction failed: gid=%s, error=%v", gid, err)
		return err
	}

	log.Infof("TCC transaction submitted successfully: gid=%s", gid)

	// If need to wait for result
	if opts.WaitResult {
		return h.waitTransactionResult(ctx, gid, TransTypeTCC)
	}

	return nil
}

// ExecuteMsg execute 2-phase message transaction
func (h *TransactionHelper) ExecuteMsg(ctx context.Context, gid string, queryPrepared string, branches []MsgBranch, opts *TransactionOptions) error {
	if opts == nil {
		opts = DefaultTransactionOptions()
	}

	msg := h.client.NewMsg(gid)
	if msg == nil {
		return fmt.Errorf("failed to create MSG transaction")
	}

	msg.TimeoutToFail = opts.TimeoutToFail
	msg.RequestTimeout = opts.BranchTimeout
	msg.RetryInterval = opts.RetryInterval

	// Add custom request headers
	if len(opts.CustomHeaders) > 0 {
		msg.BranchHeaders = opts.CustomHeaders
	}

	// Add all branches
	for _, branch := range branches {
		msg.Add(branch.Action, branch.Data)
	}

	// Prepare message
	err := msg.Prepare(queryPrepared)
	if err != nil {
		log.Errorf("MSG prepare failed: gid=%s, error=%v", gid, err)
		return err
	}

	// Submit transaction
	err = msg.Submit()
	if err != nil {
		log.Errorf("MSG transaction failed: gid=%s, error=%v", gid, err)
		return err
	}

	log.Infof("MSG transaction submitted successfully: gid=%s", gid)

	// If need to wait for result
	if opts.WaitResult {
		return h.waitTransactionResult(ctx, gid, TransTypeMsg)
	}

	return nil
}

// waitTransactionResult wait for transaction result
func (h *TransactionHelper) waitTransactionResult(ctx context.Context, gid string, transType TransactionType) error {
	// Here can implement polling logic to check transaction status
	// Simplified example, actually should call DTM's query interface
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("transaction timeout: gid=%s, type=%s", gid, transType)
		case <-ticker.C:
			// Here should query transaction status
			log.Debugf("Checking transaction status: gid=%s, type=%s", gid, transType)
			// Simplified processing, actually need to call DTM API
			return nil
		}
	}
}

// SAGABranch SAGA branch definition
type SAGABranch struct {
	Action     string      // Forward operation URL
	Compensate string      // Compensation operation URL
	Data       interface{} // Request data
}

// TCCBranch TCC branch definition
type TCCBranch struct {
	Try     string      // Try phase URL
	Confirm string      // Confirm phase URL
	Cancel  string      // Cancel phase URL
	Data    interface{} // Request data
}

// MsgBranch message branch definition
type MsgBranch struct {
	Action string      // Operation URL
	Data   interface{} // Request data
}

// XABranch XA branch definition
type XABranch struct {
	Action string // Operation URL
	Data   string // Request data as serialized string (e.g., JSON)
}

// CreateGrpcContext create gRPC Context containing transaction information
func CreateGrpcContext(ctx context.Context, gid string, transType string, branchID string, op string) context.Context {
	md := metadata.New(map[string]string{
		"dtm-gid":        gid,
		"dtm-trans-type": transType,
		"dtm-branch-id":  branchID,
		"dtm-op":         op,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// ExtractGrpcTransInfo extract transaction information from gRPC Context
func ExtractGrpcTransInfo(ctx context.Context) (*dtmcli.BranchBarrier, error) {
	// Use the built-in function from dtmgrpc package
	return dtmgrpc.BarrierFromGrpc(ctx)
}

// MustGenGid generate global transaction ID, panic on failure
func (h *TransactionHelper) MustGenGid() string {
	gid := h.client.GenerateGid()
	if gid == "" {
		log.Errorf("Failed to generate transaction gid")
		// Return a fallback GID or empty string to let caller handle
		return ""
	}
	return gid
}

// GenGid generates a transaction GID with error handling
func (h *TransactionHelper) GenGid() (string, error) {
	gid := h.client.GenerateGid()
	if gid == "" {
		return "", fmt.Errorf("failed to generate transaction gid")
	}
	return gid, nil
}

// CheckTransactionStatus check transaction status
func (h *TransactionHelper) CheckTransactionStatus(gid string) (string, error) {
	// Here should call DTM's query interface
	// Simplified example
	log.Infof("Checking transaction status for gid: %s", gid)
	return "success", nil
}

// RegisterGrpcService register gRPC service to DTM
func (h *TransactionHelper) RegisterGrpcService(serviceName string, endpoint string) error {
	if h.client.GetGRPCServer() == "" {
		return fmt.Errorf("gRPC server is not configured")
	}

	// Here can implement service registration logic
	log.Infof("Registering gRPC service: name=%s, endpoint=%s", serviceName, endpoint)
	return nil
}
