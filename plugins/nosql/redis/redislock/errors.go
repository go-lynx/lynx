package redislock

import (
	"errors"
	"fmt"
)

// 错误码定义
const (
	ErrCodeLockNotHeld           = "LOCK_NOT_HELD"
	ErrCodeLockAcquireFailed     = "LOCK_ACQUIRE_FAILED"
	ErrCodeLockAcquireTimeout    = "LOCK_ACQUIRE_TIMEOUT"
	ErrCodeLockAcquireConflict   = "LOCK_ACQUIRE_CONFLICT"
	ErrCodeRedisClientNotFound   = "REDIS_CLIENT_NOT_FOUND"
	ErrCodeMaxRetriesExceeded    = "MAX_RETRIES_EXCEEDED"
	ErrCodeLockFnRequired        = "LOCK_FN_REQUIRED"
	ErrCodeLockRenewalFailed     = "LOCK_RENEWAL_FAILED"
	ErrCodeRenewalServiceStopped = "RENEWAL_SERVICE_STOPPED"
	ErrCodeInvalidOptions        = "INVALID_OPTIONS"
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
	// ErrLockAcquireTimeout 表示获取锁超时
	ErrLockAcquireTimeout = newLockError(ErrCodeLockAcquireTimeout, "lock acquire timeout", nil)
	// ErrLockAcquireConflict 表示获取锁冲突
	ErrLockAcquireConflict = newLockError(ErrCodeLockAcquireConflict, "lock acquire conflict", nil)
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
	// ErrInvalidOptions 表示配置选项无效
	ErrInvalidOptions = newLockError(ErrCodeInvalidOptions, "invalid options", nil)
)

// 错误信息国际化映射（可以根据需要扩展）
var errorMessages = map[string]map[string]string{
	"en": {
		ErrCodeLockNotHeld:           "Lock is not held by current instance",
		ErrCodeLockAcquireFailed:     "Failed to acquire lock",
		ErrCodeLockAcquireTimeout:    "Lock acquisition timeout",
		ErrCodeLockAcquireConflict:   "Lock acquisition conflict",
		ErrCodeRedisClientNotFound:   "Redis client not found",
		ErrCodeMaxRetriesExceeded:    "Maximum retries exceeded",
		ErrCodeLockFnRequired:        "Lock function is required",
		ErrCodeLockRenewalFailed:     "Lock renewal failed",
		ErrCodeRenewalServiceStopped: "Renewal service stopped",
		ErrCodeInvalidOptions:        "Invalid options provided",
	},
	"zh": {
		ErrCodeLockNotHeld:           "锁未被当前实例持有",
		ErrCodeLockAcquireFailed:     "获取锁失败",
		ErrCodeLockAcquireTimeout:    "获取锁超时",
		ErrCodeLockAcquireConflict:   "获取锁冲突",
		ErrCodeRedisClientNotFound:   "未找到Redis客户端",
		ErrCodeMaxRetriesExceeded:    "超过最大重试次数",
		ErrCodeLockFnRequired:        "锁保护函数不能为空",
		ErrCodeLockRenewalFailed:     "锁续期失败",
		ErrCodeRenewalServiceStopped: "续期服务已停止",
		ErrCodeInvalidOptions:        "配置选项无效",
	},
}

// GetErrorMessage 获取国际化错误信息
func GetErrorMessage(code, lang string) string {
	if messages, ok := errorMessages[lang]; ok {
		if msg, ok := messages[code]; ok {
			return msg
		}
	}
	// 默认返回英文
	if messages, ok := errorMessages["en"]; ok {
		if msg, ok := messages[code]; ok {
			return msg
		}
	}
	return code
}

// 错误检查辅助函数
func IsLockError(err error, code string) bool {
	var lockErr *LockError
	if errors.As(err, &lockErr) {
		return lockErr.Code == code
	}
	return false
}
