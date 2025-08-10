package redislock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// 进程级别的锁标识前缀，在进程启动时生成
var lockValuePrefix string

// 原子计数器，用于生成唯一序列号
var sequenceNum uint64

// init 初始化锁标识前缀
func init() {
	lockValuePrefix = generateLockValuePrefix()
}

// generateLockValuePrefix 生成锁值前缀，带重试机制
func generateLockValuePrefix() string {
	// 获取主机名
	hostname := getHostnameWithRetry()

	// 获取本机 IP
	ip := getLocalIPWithRetry()

	// 生成进程级别的唯一标识前缀
	return fmt.Sprintf("%s-%s-%d-", hostname, ip, os.Getpid())
}

// getHostnameWithRetry 获取主机名，带重试机制
func getHostnameWithRetry() string {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		hostname, err := os.Hostname()
		if err == nil {
			return hostname
		}
		log.ErrorCtx(context.Background(), "failed to get hostname", "attempt", i+1, "error", err)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
		}
	}
	return "unknown-host"
}

// getLocalIPWithRetry 获取本机IP，带重试机制
func getLocalIPWithRetry() string {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipv4 := ipnet.IP.To4(); ipv4 != nil {
						return ipv4.String()
					}
				}
			}
			break
		}
		log.ErrorCtx(context.Background(), "failed to get interface addresses", "attempt", i+1, "error", err)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
		}
	}
	return "unknown-ip"
}

// generateLockValue 生成锁的唯一标识值
func generateLockValue() string {
	// 使用原子操作获取递增的序列号
	seq := atomic.AddUint64(&sequenceNum, 1)
	// 使用 crypto/rand 生成 16 字节高熵随机数，并 hex 编码
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 极端情况下读取熵失败，回退到时间戳，仍保留前缀与序列号，避免阻塞
		return fmt.Sprintf("%s%d-%d", lockValuePrefix, seq, time.Now().UnixNano())
	}
	token := hex.EncodeToString(b[:])
	// 生成唯一标识：进程前缀 + 序列号 + 随机 token
	return fmt.Sprintf("%s%d-%s", lockValuePrefix, seq, token)
}
