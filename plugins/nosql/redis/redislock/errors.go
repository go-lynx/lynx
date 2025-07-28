package redislock

import (
	"errors"
	"fmt"
)

// 错误码定义
const (
	ErrCodeLockNotHeld           = "LOCK_NOT_HELD"
	ErrCodeLockAcquireFailed     = "LOCK_ACQUIRE_FAILED"
	ErrCodeRedisClientNotFound   = "REDIS_CLIENT_NOT_FOUND"
	ErrCodeMaxRetriesExceeded    = "MAX_RETRIES_EXCEEDED"
	ErrCodeLockFnRequired        = "LOCK_FN_REQUIRED"
	ErrCodeLockRenewalFailed     = "LOCK_RENEWAL_FAILED"
	ErrCodeRenewalServiceStopped = "RENEWAL_SERVICE_STOPPED"
)

// LockError 自定义锁错误类型
type LockError struct {
	Code    string
	Message string
	Err     error
}

func (e *LockError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

// 创建锁错误的辅助函数
func newLockError(code, message string, err error) *LockError {
	return &LockError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

var (
	// ErrLockNotHeld 表示尝试释放未持有的锁
	ErrLockNotHeld = newLockError(ErrCodeLockNotHeld, "lock not held", nil)
	// ErrLockAcquireFailed 表示获取锁失败
	ErrLockAcquireFailed = newLockError(ErrCodeLockAcquireFailed, "failed to acquire lock", nil)
	// ErrRedisClientNotFound 表示未找到 Redis 客户端
	ErrRedisClientNotFound = newLockError(ErrCodeRedisClientNotFound, "redis client not found", nil)
	// ErrMaxRetriesExceeded 表示超过最大重试次数
	ErrMaxRetriesExceeded = newLockError(ErrCodeMaxRetriesExceeded, "max retries exceeded", nil)
	// ErrLockFnRequired 表示锁保护的函数不能为空
	ErrLockFnRequired = newLockError(ErrCodeLockFnRequired, "lock function is required", nil)
	// ErrLockRenewalFailed 表示锁续期失败
	ErrLockRenewalFailed = newLockError(ErrCodeLockRenewalFailed, "lock renewal failed", nil)
	// ErrRenewalServiceStopped 表示续期服务已停止
	ErrRenewalServiceStopped = newLockError(ErrCodeRenewalServiceStopped, "renewal service stopped", nil)
)

// 错误检查辅助函数
func IsLockError(err error, code string) bool {
	var lockErr *LockError
	if errors.As(err, &lockErr) {
		return lockErr.Code == code
	}
	return false
}
