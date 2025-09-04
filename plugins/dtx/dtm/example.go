package dtm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-resty/resty/v2"
)

// ExampleService example service, demonstrating how to use the DTM plugin
type ExampleService struct {
	dtmClient *DTMClient
	db        *sql.DB
}

// NewExampleService create example service
func NewExampleService(dtmClient *DTMClient, db *sql.DB) *ExampleService {
	return &ExampleService{
		dtmClient: dtmClient,
		db:        db,
	}
}

// TransferExample transfer example - using SAGA pattern
func (s *ExampleService) TransferExample(ctx context.Context, fromAccount, toAccount string, amount float64) error {
	// Generate global transaction ID
	gid := s.dtmClient.GenerateGid()

	// Create SAGA transaction
	saga := s.dtmClient.NewSaga(gid)

	// Define transfer data
	transferOut := map[string]interface{}{
		"account": fromAccount,
		"amount":  amount,
	}

	transferIn := map[string]interface{}{
		"account": toAccount,
		"amount":  amount,
	}

	// Add transfer out branch (forward operation and compensation operation)
	saga.Add(
		"http://localhost:8080/api/transfer/out",
		"http://localhost:8080/api/transfer/out/compensate",
		transferOut,
	)

	// Add transfer in branch
	saga.Add(
		"http://localhost:8080/api/transfer/in",
		"http://localhost:8080/api/transfer/in/compensate",
		transferIn,
	)

	// Submit SAGA transaction
	err := saga.Submit()
	if err != nil {
		log.Errorf("Transfer failed: gid=%s, error=%v", gid, err)
		return fmt.Errorf("transfer failed: %w", err)
	}

	log.Infof("Transfer initiated successfully: gid=%s, from=%s, to=%s, amount=%.2f",
		gid, fromAccount, toAccount, amount)
	return nil
}

// OrderExample order example - using TCC pattern
func (s *ExampleService) OrderExample(ctx context.Context, orderID string, userID string, productID string, quantity int) error {
	// Generate global transaction ID
	gid := s.dtmClient.GenerateGid()

	// Inventory deduction request
	inventoryReq := map[string]interface{}{
		"product_id": productID,
		"quantity":   quantity,
	}

	// Create order request
	orderReq := map[string]interface{}{
		"order_id":   orderID,
		"user_id":    userID,
		"product_id": productID,
		"quantity":   quantity,
	}

	// Use the new TCC global transaction API
	err := dtmcli.TccGlobalTransaction(s.dtmClient.GetServerURL(), gid, func(tcc *dtmcli.Tcc) (*resty.Response, error) {
		// Call inventory service TCC branch
		_, err := tcc.CallBranch(
			inventoryReq,
			"http://localhost:8081/api/inventory/try",
			"http://localhost:8081/api/inventory/confirm",
			"http://localhost:8081/api/inventory/cancel",
		)
		if err != nil {
			log.Errorf("Inventory TCC branch failed: %v", err)
			return nil, err
		}

		// Call order service TCC branch
		_, err = tcc.CallBranch(
			orderReq,
			"http://localhost:8082/api/order/try",
			"http://localhost:8082/api/order/confirm",
			"http://localhost:8082/api/order/cancel",
		)
		if err != nil {
			log.Errorf("Order TCC branch failed: %v", err)
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		log.Errorf("Order transaction failed: gid=%s, error=%v", gid, err)
		return fmt.Errorf("order transaction failed: %w", err)
	}

	log.Infof("Order created successfully: gid=%s, orderID=%s", gid, orderID)
	return nil
}

// MessageExample message example - using 2-phase message pattern
func (s *ExampleService) MessageExample(ctx context.Context, messageID string, content string) error {
	// Generate global transaction ID
	gid := s.dtmClient.GenerateGid()

	// Create message transaction
	msg := s.dtmClient.NewMsg(gid)

	// Message data
	messageData := map[string]interface{}{
		"message_id": messageID,
		"content":    content,
	}

	// Add message processing steps
	msg.Add("http://localhost:8083/api/message/process", messageData)
	msg.Add("http://localhost:8083/api/message/notify", messageData)

	// Prepare message (query preparation status)
	err := msg.Prepare("http://localhost:8083/api/message/query")
	if err != nil {
		log.Errorf("Message prepare failed: %v", err)
		return err
	}

	// Submit message transaction
	err = msg.Submit()
	if err != nil {
		log.Errorf("Message transaction failed: gid=%s, error=%v", gid, err)
		return fmt.Errorf("message transaction failed: %w", err)
	}

	log.Infof("Message sent successfully: gid=%s, messageID=%s", gid, messageID)
	return nil
}

// BarrierExample branch barrier example - handling idempotency, suspension, and empty compensation issues
func (s *ExampleService) BarrierExample(ctx context.Context, bb *dtmcli.BranchBarrier) error {
	// Use branch barrier to execute business logic
	return bb.CallWithDB(s.db, func(tx *sql.Tx) error {
		// Execute business logic within transaction
		// Branch barrier automatically handles idempotency, suspension, and empty compensation issues

		// Example: Update account balance
		_, err := tx.Exec(
			"UPDATE accounts SET balance = balance - ? WHERE account_id = ?",
			100.0, "account123",
		)
		if err != nil {
			return err
		}

		log.Infof("Business logic executed within barrier: gid=%s, branchID=%s",
			bb.Gid, bb.BranchID)
		return nil
	})
}

// HandleTCCTryExample TCC Try phase handling example
func (s *ExampleService) HandleTCCTryExample(ctx context.Context, req map[string]interface{}) error {
	// Create branch barrier from request
	bb, err := dtmcli.BarrierFromQuery(nil)
	if err != nil {
		return err
	}

	// Use branch barrier to handle Try phase
	return bb.CallWithDB(s.db, func(tx *sql.Tx) error {
		// Try phase: Reserve resources
		productID := req["product_id"].(string)
		quantity := req["quantity"].(int)

		// Check inventory
		var available int
		err := tx.QueryRow(
			"SELECT available FROM inventory WHERE product_id = ?",
			productID,
		).Scan(&available)
		if err != nil {
			return err
		}

		if available < quantity {
			return fmt.Errorf("insufficient inventory")
		}

		// Reserve inventory
		_, err = tx.Exec(
			"UPDATE inventory SET available = available - ?, reserved = reserved + ? WHERE product_id = ?",
			quantity, quantity, productID,
		)
		if err != nil {
			return err
		}

		log.Infof("TCC Try succeeded: productID=%s, quantity=%d", productID, quantity)
		return nil
	})
}

// HandleTCCConfirmExample TCC Confirm phase handling example
func (s *ExampleService) HandleTCCConfirmExample(ctx context.Context, req map[string]interface{}) error {
	bb, err := dtmcli.BarrierFromQuery(nil)
	if err != nil {
		return err
	}

	return bb.CallWithDB(s.db, func(tx *sql.Tx) error {
		// Confirm phase: Confirm reserved resources
		productID := req["product_id"].(string)
		quantity := req["quantity"].(int)

		// Convert reserved inventory to sold
		_, err := tx.Exec(
			"UPDATE inventory SET reserved = reserved - ?, sold = sold + ? WHERE product_id = ?",
			quantity, quantity, productID,
		)
		if err != nil {
			return err
		}

		log.Infof("TCC Confirm succeeded: productID=%s, quantity=%d", productID, quantity)
		return nil
	})
}

// HandleTCCCancelExample TCC Cancel phase handling example
func (s *ExampleService) HandleTCCCancelExample(ctx context.Context, req map[string]interface{}) error {
	bb, err := dtmcli.BarrierFromQuery(nil)
	if err != nil {
		return err
	}

	return bb.CallWithDB(s.db, func(tx *sql.Tx) error {
		// Cancel phase: Release reserved resources
		productID := req["product_id"].(string)
		quantity := req["quantity"].(int)

		// Release reserved inventory back to available inventory
		_, err := tx.Exec(
			"UPDATE inventory SET available = available + ?, reserved = reserved - ? WHERE product_id = ?",
			quantity, quantity, productID,
		)
		if err != nil {
			return err
		}

		log.Infof("TCC Cancel succeeded: productID=%s, quantity=%d", productID, quantity)
		return nil
	})
}

// WorkflowExample workflow example - using helper tools
func (s *ExampleService) WorkflowExample(ctx context.Context) error {
	helper := NewTransactionHelper(s.dtmClient)

	// Use helper tools to execute SAGA transaction
	gid := helper.MustGenGid()

	branches := []SAGABranch{
		{
			Action:     "http://localhost:8080/api/step1",
			Compensate: "http://localhost:8080/api/step1/compensate",
			Data:       map[string]interface{}{"step": 1},
		},
		{
			Action:     "http://localhost:8080/api/step2",
			Compensate: "http://localhost:8080/api/step2/compensate",
			Data:       map[string]interface{}{"step": 2},
		},
		{
			Action:     "http://localhost:8080/api/step3",
			Compensate: "http://localhost:8080/api/step3/compensate",
			Data:       map[string]interface{}{"step": 3},
		},
	}

	opts := &TransactionOptions{
		TimeoutToFail: 120,
		BranchTimeout: 30,
		WaitResult:    true,
		Concurrent:    true, // Execute branches concurrently
	}

	err := helper.ExecuteSAGA(ctx, gid, branches, opts)
	if err != nil {
		log.Errorf("Workflow failed: %v", err)
		return err
	}

	log.Infof("Workflow completed successfully: gid=%s", gid)
	return nil
}
