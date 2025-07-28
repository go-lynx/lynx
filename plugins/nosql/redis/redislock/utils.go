package redislock

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/go-lynx/lynx/app/log"
	"github.com/google/uuid"
)

// 进程级别的锁标识前缀，在进程启动时生成
var lockValuePrefix string

// 原子计数器，用于生成唯一序列号
var sequenceNum uint64

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

// generateLockValue 生成锁的唯一标识值
func generateLockValue() string {
	// 使用原子操作获取递增的序列号
	seq := atomic.AddUint64(&sequenceNum, 1)
	// 生成 UUID v4
	uid := uuid.New()
	// 生成唯一标识：进程前缀 + 序列号 + UUID
	return fmt.Sprintf("%s%d-%s", lockValuePrefix, seq, uid.String())
}
