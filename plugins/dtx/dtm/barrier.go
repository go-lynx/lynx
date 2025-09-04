package dtm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmcli/dtmimp"
	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/go-lynx/lynx/app/log"
)

// BarrierHandler transaction barrier handler
type BarrierHandler struct {
	client *DTMClient
}

// NewBarrierHandler create transaction barrier handler
func NewBarrierHandler(client *DTMClient) *BarrierHandler {
	return &BarrierHandler{
		client: client,
	}
}

// CallWithDB execute branch barrier within database transaction
func (b *BarrierHandler) CallWithDB(ctx context.Context, db *sql.DB, req *dtmcli.BranchBarrier, fn dtmcli.BarrierBusiFunc) error {
	return req.CallWithDB(db, fn)
}

// CallWithTx execute branch barrier within existing transaction
func (b *BarrierHandler) CallWithTx(ctx context.Context, tx *sql.Tx, req *dtmcli.BranchBarrier, fn dtmcli.BarrierBusiFunc) error {
	return req.Call(tx, fn)
}

// CreateBarrierFromGin create branch barrier from Gin request
func (b *BarrierHandler) CreateBarrierFromGin(c interface{}) (*dtmcli.BranchBarrier, error) {
	// Adaptation is needed based on the actual web framework
	// Example code assumes using Gin framework
	return dtmcli.BarrierFromQuery(nil)
}

// CreateBarrierFromGrpc create branch barrier from gRPC request
func (b *BarrierHandler) CreateBarrierFromGrpc(ctx context.Context) (*dtmcli.BranchBarrier, error) {
	// Use the built-in function from dtmgrpc package
	return dtmgrpc.BarrierFromGrpc(ctx)
}

// HandleTCCTry handle TCC Try phase
func (b *BarrierHandler) HandleTCCTry(ctx context.Context, bb *dtmcli.BranchBarrier, db *sql.DB, busiCall dtmcli.BarrierBusiFunc) error {
	if bb.Op != dtmimp.OpTry {
		return fmt.Errorf("invalid operation for TCC Try: %s", bb.Op)
	}

	return bb.CallWithDB(db, func(tx *sql.Tx) error {
		log.Infof("Executing TCC Try for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		return busiCall(tx)
	})
}

// HandleTCCConfirm handle TCC Confirm phase
func (b *BarrierHandler) HandleTCCConfirm(ctx context.Context, bb *dtmcli.BranchBarrier, db *sql.DB, busiCall dtmcli.BarrierBusiFunc) error {
	if bb.Op != dtmimp.OpConfirm {
		return fmt.Errorf("invalid operation for TCC Confirm: %s", bb.Op)
	}

	return bb.CallWithDB(db, func(tx *sql.Tx) error {
		log.Infof("Executing TCC Confirm for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		return busiCall(tx)
	})
}

// HandleTCCCancel handle TCC Cancel phase
func (b *BarrierHandler) HandleTCCCancel(ctx context.Context, bb *dtmcli.BranchBarrier, db *sql.DB, busiCall dtmcli.BarrierBusiFunc) error {
	if bb.Op != dtmimp.OpCancel {
		return fmt.Errorf("invalid operation for TCC Cancel: %s", bb.Op)
	}

	return bb.CallWithDB(db, func(tx *sql.Tx) error {
		log.Infof("Executing TCC Cancel for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		return busiCall(tx)
	})
}

// HandleSAGA handle SAGA transaction
func (b *BarrierHandler) HandleSAGA(ctx context.Context, bb *dtmcli.BranchBarrier, db *sql.DB, busiCall dtmcli.BarrierBusiFunc) error {
	if bb.Op != dtmimp.OpAction && bb.Op != dtmimp.OpCompensate {
		return fmt.Errorf("invalid operation for SAGA: %s", bb.Op)
	}

	return bb.CallWithDB(db, func(tx *sql.Tx) error {
		if bb.Op == dtmimp.OpAction {
			log.Infof("Executing SAGA Action for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		} else {
			log.Infof("Executing SAGA Compensate for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		}
		return busiCall(tx)
	})
}

// HandleMsg handle 2-phase message
func (b *BarrierHandler) HandleMsg(ctx context.Context, bb *dtmcli.BranchBarrier, db *sql.DB, busiCall dtmcli.BarrierBusiFunc) error {
	if bb.Op != dtmimp.OpAction {
		return fmt.Errorf("invalid operation for MSG: %s", bb.Op)
	}

	return bb.CallWithDB(db, func(tx *sql.Tx) error {
		log.Infof("Executing MSG Action for gid: %s, branch: %s", bb.Gid, bb.BranchID)
		return busiCall(tx)
	})
}
