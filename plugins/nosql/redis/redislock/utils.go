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

// Process-level lock identifier prefix, generated at process startup
var lockValuePrefix string

// Atomic counter, used to generate unique sequence numbers
var sequenceNum uint64

// init initializes the lock identifier prefix
func init() {
	lockValuePrefix = generateLockValuePrefix()
}

// generateLockValuePrefix generates lock value prefix with retry mechanism
func generateLockValuePrefix() string {
	// Get hostname
	hostname := getHostnameWithRetry()

	// Get local IP
	ip := getLocalIPWithRetry()

	// Generate process-level unique identifier prefix
	return fmt.Sprintf("%s-%s-%d-", hostname, ip, os.Getpid())
}

// getHostnameWithRetry gets hostname with retry mechanism
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

// getLocalIPWithRetry gets local IP with retry mechanism
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

// generateLockValue generates unique identifier value for the lock
func generateLockValue() string {
	// Use atomic operation to get incrementing sequence number
	seq := atomic.AddUint64(&sequenceNum, 1)
	// Use crypto/rand to generate 16-byte high-entropy random number and hex encode it
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// In extreme cases where entropy reading fails, fall back to timestamp, still retaining prefix and sequence number to avoid blocking
		return fmt.Sprintf("%s%d-%d", lockValuePrefix, seq, time.Now().UnixNano())
	}
	token := hex.EncodeToString(b[:])
	// Generate unique identifier: process prefix + sequence number + random token
	return fmt.Sprintf("%s%d-%s", lockValuePrefix, seq, token)
}
