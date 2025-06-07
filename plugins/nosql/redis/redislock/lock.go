package redislock

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	lynx "github.com/go-lynx/lynx/plugins/nosql/redis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

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

	// 全局锁管理器
	globalLockManager = &lockManager{
		locks: make(map[string]*RedisLock),
	}

	// 进程级别的锁标识前缀，在进程启动时生成
	lockValuePrefix string
)

// startRenewalService 启动续期服务
func (lm *lockManager) startRenewalService() {
	lm.mutex.Lock()
	if lm.running {
		lm.mutex.Unlock()
		return
	}
	lm.renewCtx, lm.renewCancel = context.WithCancel(context.Background())
	lm.running = true
	lm.mutex.Unlock()

	go func() {
		ticker := time.NewTicker(time.Second) // 每秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.mutex.RLock()
				for _, lock := range lm.locks {
					go lm.renewLock(lock)
				}
				lm.mutex.RUnlock()
			case <-lm.renewCtx.Done():
				return
			}
		}
	}()
}

// renewLock 续期单个锁
func (lm *lockManager) renewLock(lock *RedisLock) {
	// 检查是否需要续期
	if time.Until(lock.expiresAt) > lock.expiration/3 {
		return
	}

	// 执行续期脚本
	result, err := lock.client.Eval(context.Background(), renewScript, []string{lock.key},
		lock.value, lock.expiration.Milliseconds()).Result()
	if err != nil {
		log.Error(context.Background(), "failed to renew lock", "error", err)
		return
	}

	switch result.(int64) {
	case 1: // 续期成功
		lock.mutex.Lock()
		lock.expiresAt = time.Now().Add(lock.expiration)
		lock.mutex.Unlock()
	case 0, -1, -2: // 锁不存在或不是当前持有者
		lm.mutex.Lock()
		delete(lm.locks, lock.key)
		lm.mutex.Unlock()
	default:
		log.Error(context.Background(), "unknown renewal result", "result", result)
	}
}

// init 初始化锁标识前缀
func init() {
	// 获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
		log.Error(context.Background(), "failed to get hostname", "error", err)
	}

	// 获取本机 IP
	ip := "unknown-ip"
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Error(context.Background(), "failed to get interface addresses", "error", err)
	} else {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipv4 := ipnet.IP.To4(); ipv4 != nil {
					ip = ipv4.String()
					break
				}
			}
		}
	}

	// 生成进程级别的唯一标识前缀
	lockValuePrefix = fmt.Sprintf("%s-%s-%d-", hostname, ip, os.Getpid())
}

// lockManager 管理所有的分布式锁实例
type lockManager struct {
	mutex sync.RWMutex
	locks map[string]*RedisLock
	// 续期服务
	renewCtx    context.Context
	renewCancel context.CancelFunc
	running     bool
}

// RedisLock 实现了基于 Redis 的分布式锁
type RedisLock struct {
	client     *redis.Client // Redis 客户端
	key        string        // 锁的键名
	value      string        // 锁的值（用于识别持有者）
	expiration time.Duration // 锁的过期时间
	expiresAt  time.Time     // 锁的过期时间点
	mutex      sync.Mutex    // 保护内部状态
}

// 获取锁的 Lua 脚本
const lockScript = `
if redis.call("EXISTS", KEYS[1]) == 0 then
    -- 设置锁的值和过期时间
    redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX")
    return "OK"
end

-- 如果锁已存在，检查是否是当前持有者
-- 这里可以扩展实现可重入锁
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("PEXPIRE", KEYS[1], ARGV[2])
    return "OK"
end

return "LOCKED"`

// 释放锁的 Lua 脚本
const unlockScript = `
-- 检查锁是否存在且是否为当前持有者
if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("DEL", KEYS[1])
    return 1
end

-- 检查锁是否存在
if redis.call("EXISTS", KEYS[1]) == 0 then
    return 0
end

-- 锁存在但不是当前持有者
return -1`

// 续期锁的 Lua 脚本
const renewScript = `
-- 检查锁是否存在且是否为当前持有者
local value = redis.call("GET", KEYS[1])
if value == ARGV[1] then
    -- 如果是当前持有者，则续期
    if redis.call("PEXPIRE", KEYS[1], ARGV[2]) == 1 then
        return 1
    end
    -- 如果 PEXPIRE 失败，说明锁已经不存在
    return -2
end

-- 锁不存在
if value == false then
    return -1
end

-- 锁存在但不是当前持有者
return 0`

// 原子计数器，用于生成唯一序列号
var sequenceNum uint64

// generateLockValue 生成锁的唯一标识值
func generateLockValue() string {
	// 使用原子操作获取递增的序列号
	seq := atomic.AddUint64(&sequenceNum, 1)
	// 生成 UUID v4
	uuid := uuid.New()
	// 生成唯一标识：进程前缀 + 序列号 + UUID
	return fmt.Sprintf("%s%d-%s", lockValuePrefix, seq, uuid.String())
}

// RetryStrategy 定义锁重试策略
type RetryStrategy struct {
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试间隔
}

// DefaultRetryStrategy 默认的重试策略
var DefaultRetryStrategy = RetryStrategy{
	MaxRetries: 3,
	RetryDelay: 100 * time.Millisecond,
}

// Lock 获取锁并执行函数
func Lock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	return LockWithRetry(ctx, key, expiration, fn, RetryStrategy{MaxRetries: 0})
}

// LockWithRetry 获取锁并执行函数，支持重试
func LockWithRetry(ctx context.Context, key string, expiration time.Duration, fn func() error, strategy RetryStrategy) error {
	if fn == nil {
		return ErrLockFnRequired
	}

	// 从应用程序获取 Redis 客户端
	client := lynx.GetRedis()
	if client == nil {
		return ErrRedisClientNotFound
	}

	// 生成锁值
	value := generateLockValue()

	// 创建新的锁实例
	lock := &RedisLock{
		client:     client,
		key:        key,
		value:      value,
		expiration: expiration,
	}

	// 尝试获取锁
	for retries := 0; ; retries++ {
		// 检查是否超过最大重试次数
		if strategy.MaxRetries > 0 && retries >= strategy.MaxRetries {
			return ErrMaxRetriesExceeded
		}

		// 如果不是第一次尝试，等待重试间隔
		if retries > 0 {
			select {
			case <-time.After(strategy.RetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// 执行加锁脚本
		result, err := lock.client.Eval(ctx, lockScript, []string{lock.key},
			lock.value, lock.expiration.Milliseconds()).Result()
		if err != nil {
			return err
		}

		// 检查是否成功获取锁
		switch result {
		case "OK":
			// 设置锁的过期时间点
			lock.expiresAt = time.Now().Add(lock.expiration)
			// 添加到全局锁管理器
			globalLockManager.mutex.Lock()
			globalLockManager.locks[key] = lock
			globalLockManager.mutex.Unlock()
			// 启动续期服务
			globalLockManager.startRenewalService()

			// 使用 defer 确保锁会被释放
			var err error
			defer func() {
				if unlockErr := Unlock(ctx, key); unlockErr != nil {
					log.Error(ctx, "failed to unlock", "error", unlockErr)
					// 如果原始错误为空，则返回解锁错误
					if err == nil {
						err = unlockErr
					}
				}
			}()

			// 执行用户函数
			err = fn()
			return err
		case "LOCKED":
			// 如果不需要重试，直接返回错误
			if strategy.MaxRetries == 0 {
				return ErrLockAcquireFailed
			}
			// 否则继续重试
			continue
		default:
			return fmt.Errorf("unknown lock result: %v", result)
		}
	}
}

// Unlock 释放指定键的分布式锁
func Unlock(ctx context.Context, key string) error {
	// 获取锁实例
	globalLockManager.mutex.Lock()
	lock, exists := globalLockManager.locks[key]
	if !exists {
		globalLockManager.mutex.Unlock()
		return ErrLockNotHeld
	}
	delete(globalLockManager.locks, key)
	globalLockManager.mutex.Unlock()

	// 执行解锁脚本
	result, err := lock.client.Eval(ctx, unlockScript, []string{lock.key}, lock.value).Result()
	if err != nil {
		return err
	}

	// 检查是否成功释放锁
	switch result.(int64) {
	case 1: // 成功释放锁
		return nil
	case 0: // 锁不存在
		return ErrLockNotHeld
	case -1: // 锁存在但不是当前持有者
		return ErrLockNotHeld
	default:
		return fmt.Errorf("unknown unlock result: %v", result)
	}

}

// IsLocked 检查锁是否被当前实例持有
func (rl *RedisLock) IsLocked(ctx context.Context) (bool, error) {
	value, err := rl.client.Get(ctx, rl.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == rl.value, nil
}
