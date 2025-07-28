package redislock

import "errors"

var (
	// ErrLockNotHeld 表示尝试释放未持有的锁
	ErrLockNotHeld = errors.New("lock not held")
	// ErrLockAcquireFailed 表示获取锁失败
	ErrLockAcquireFailed = errors.New("failed to acquire lock")
	// ErrRedisClientNotFound 表示未找到 Redis 客户端
	ErrRedisClientNotFound = errors.New("redis client not found")
	// ErrMaxRetriesExceeded 表示超过最大重试次数
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	// ErrLockFnRequired 表示锁保护的函数不能为空
	ErrLockFnRequired = errors.New("lock function is required")
	// ErrLockRenewalFailed 表示锁续期失败
	ErrLockRenewalFailed = errors.New("lock renewal failed")
	// ErrRenewalServiceStopped 表示续期服务已停止
	ErrRenewalServiceStopped = errors.New("renewal service stopped")
)
