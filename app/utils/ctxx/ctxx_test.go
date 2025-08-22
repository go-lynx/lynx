package ctxx

import (
	"context"
	"testing"
	"time"
)

func TestCtxX(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if ctx.Err() == nil {
		t.Fatalf("WithTimeout should cancel")
	}

	// Value generic
	key := struct{}{}
	ctx2 := context.WithValue(context.Background(), key, 123)
	if v, ok := Value[int](ctx2, key); !ok || v != 123 {
		t.Fatalf("Value typed get failed")
	}

	// Detach should not be canceled when parent is canceled
	p, cancelP := context.WithCancel(context.Background())
	d := Detach(p)
	cancelP()
	select {
	case <-d.Done():
		t.Fatalf("Detach should not cancel")
	default:
	}
}
