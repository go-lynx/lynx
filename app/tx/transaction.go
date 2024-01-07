package tx

import (
	"context"
)

type Transaction interface {
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TransactionManager interface {
	WithTx(ctx context.Context, fn func() error) error
	Transaction
}

func NewLynxTransactionManager() TransactionManager {
	return nil
}

// LynxTransactionManager is the local implementation of the TransactionManager
type LynxTransactionManager struct {
	tx TransactionManager
}

func (l *LynxTransactionManager) WithTx(ctx context.Context, fn func() error) error {
	err := l.tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = l.tx.Rollback(ctx)
		}
	}()

	err = fn()
	if err != nil {
		_ = l.tx.Rollback(ctx)
		return err
	}
	return l.tx.Commit(ctx)
}

func WithTx(ctx context.Context, fn func() error) error {
	l := &LynxTransactionManager{}
	err := l.WithTx(ctx, fn)
	if err != nil {
		return err
	}
	return err
}
