package redislock

import (
	"context"
	"testing"
	"time"

	redismock "github.com/elliotchance/redismock/v9"
)

// helper to create a basic RedisLock for tests with mocked client and default expiration
func newTestLock(client redismock.ClientMock, key string, value string, exp time.Duration) *RedisLock {
	ownerKey, countKey := buildLockKeys(key)
	tokenKey := buildTokenKey(key)
	return &RedisLock{
		client:           client,
		key:              key,
		value:            value,
		expiration:       exp,
		renewalThreshold: DefaultLockOptions.RenewalThreshold,
		ownerKey:         ownerKey,
		countKey:         countKey,
		tokenKey:         tokenKey,
	}
}

func TestFencingToken_OnFirstAcquireAndReentry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mock := redismock.NewClientMock()

	key := "order:123"
	exp := 2 * time.Second
	lock := newTestLock(client, key, "val-A", exp)

	// First acquire returns 1 (first ownership) -> should INCR tokenKey
	mock.ExpectEvalSha(lockScript.Hash(), []string{lock.ownerKey, lock.countKey}, lock.value, exp.Milliseconds()).SetVal(int64(1))
	mock.ExpectIncr(lock.tokenKey).SetVal(100)

	if err := lock.Acquire(ctx); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if got := lock.GetToken(); got != 100 {
		t.Fatalf("expected token 100 on first acquire, got %d", got)
	}

	// Re-entrant acquire returns >1 (e.g., 2). No INCR should happen.
	mock.ExpectEvalSha(lockScript.Hash(), []string{lock.ownerKey, lock.countKey}, lock.value, exp.Milliseconds()).SetVal(int64(2))

	if err := lock.Acquire(ctx); err != nil {
		t.Fatalf("reentrant acquire failed: %v", err)
	}
	if got := lock.GetToken(); got != 100 {
		t.Fatalf("expected token to remain 100 after reentry, got %d", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet redis expectations: %v", err)
	}
}

func TestFencingToken_RenewDoesNotChangeToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mock := redismock.NewClientMock()

	key := "task:777"
	exp := 1500 * time.Millisecond
	lock := newTestLock(client, key, "val-B", exp)

	// First acquire -> INCR token
	mock.ExpectEvalSha(lockScript.Hash(), []string{lock.ownerKey, lock.countKey}, lock.value, exp.Milliseconds()).SetVal(int64(1))
	mock.ExpectIncr(lock.tokenKey).SetVal(200)

	if err := lock.Acquire(ctx); err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if got := lock.GetToken(); got != 200 {
		t.Fatalf("expected token 200, got %d", got)
	}

	// Renew should not change token; return 1 for successful renewal
	mock.ExpectEvalSha(renewScript.Hash(), []string{lock.ownerKey, lock.countKey}, lock.value, exp.Milliseconds()).SetVal(int64(1))

	if err := lock.Renew(ctx, exp); err != nil {
		t.Fatalf("renew failed: %v", err)
	}
	if got := lock.GetToken(); got != 200 {
		t.Fatalf("expected token to remain 200 after renew, got %d", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet redis expectations: %v", err)
	}
}

func TestFencingToken_ReacquireAfterReleaseIncrements(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mock := redismock.NewClientMock()

	key := "job:9"
	exp := 1200 * time.Millisecond

	// First lock
	lock1 := newTestLock(client, key, "val-C", exp)
	mock.ExpectEvalSha(lockScript.Hash(), []string{lock1.ownerKey, lock1.countKey}, lock1.value, exp.Milliseconds()).SetVal(int64(1))
	mock.ExpectIncr(lock1.tokenKey).SetVal(300)
	if err := lock1.Acquire(ctx); err != nil {
		t.Fatalf("acquire #1 failed: %v", err)
	}
	if lock1.GetToken() != 300 {
		t.Fatalf("expected token 300, got %d", lock1.GetToken())
	}

	// Release fully (script returns 1). Our Release passes TTL=0.
	mock.ExpectEvalSha(unlockScript.Hash(), []string{lock1.ownerKey, lock1.countKey}, lock1.value, int64(0)).SetVal(int64(1))
	if err := lock1.Release(ctx); err != nil {
		t.Fatalf("release #1 failed: %v", err)
	}

	// New lock instance (different owner/value) acquires again -> INCR to 301
	lock2 := newTestLock(client, key, "val-D", exp)
	mock.ExpectEvalSha(lockScript.Hash(), []string{lock2.ownerKey, lock2.countKey}, lock2.value, exp.Milliseconds()).SetVal(int64(1))
	mock.ExpectIncr(lock2.tokenKey).SetVal(301)
	if err := lock2.Acquire(ctx); err != nil {
		t.Fatalf("acquire #2 failed: %v", err)
	}
	if lock2.GetToken() != 301 {
		t.Fatalf("expected token 301, got %d", lock2.GetToken())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet redis expectations: %v", err)
	}
}
