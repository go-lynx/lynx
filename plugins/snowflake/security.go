package snowflake

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	// Authentication settings
	EnableAuthentication bool     `json:"enable_authentication"`
	APIKeys              []string `json:"api_keys"`
	TokenExpiration      int64    `json:"token_expiration"` // seconds

	// Access control settings
	EnableIPWhitelist bool     `json:"enable_ip_whitelist"`
	AllowedIPs        []string `json:"allowed_ips"`
	EnableRateLimit   bool     `json:"enable_rate_limit"`
	RateLimit         int      `json:"rate_limit"` // requests per second

	// Encryption settings
	EnableEncryption bool   `json:"enable_encryption"`
	EncryptionKey    string `json:"encryption_key"`

	// Audit settings
	EnableAuditLog bool   `json:"enable_audit_log"`
	AuditLogPath   string `json:"audit_log_path"`
}

// SecurityManager manages security features for the snowflake plugin
type SecurityManager struct {
	config      *SecurityConfig
	mu          sync.RWMutex
	rateLimiter *RateLimiter
	auditLogger *AuditLogger
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *SecurityConfig) (*SecurityManager, error) {
	if config == nil {
		return nil, fmt.Errorf("security config cannot be nil")
	}

	sm := &SecurityManager{
		config: config,
	}

	// Initialize rate limiter if enabled
	if config.EnableRateLimit {
		sm.rateLimiter = NewRateLimiter(config.RateLimit)
	}

	// Initialize audit logger if enabled
	if config.EnableAuditLog {
		auditLogger, err := NewAuditLogger(config.AuditLogPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize audit logger: %w", err)
		}
		sm.auditLogger = auditLogger
	}

	return sm, nil
}

// ValidateAPIKey validates an API key
func (sm *SecurityManager) ValidateAPIKey(apiKey string) bool {
	if !sm.config.EnableAuthentication {
		return true // Authentication disabled
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, validKey := range sm.config.APIKeys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
			return true
		}
	}

	return false
}

// CheckIPWhitelist checks if an IP is in the whitelist
func (sm *SecurityManager) CheckIPWhitelist(clientIP string) bool {
	if !sm.config.EnableIPWhitelist {
		return true // IP whitelist disabled
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	clientIPAddr := net.ParseIP(clientIP)
	if clientIPAddr == nil {
		return false
	}

	for _, allowedIP := range sm.config.AllowedIPs {
		// Support CIDR notation
		if strings.Contains(allowedIP, "/") {
			_, ipNet, err := net.ParseCIDR(allowedIP)
			if err != nil {
				continue
			}
			if ipNet.Contains(clientIPAddr) {
				return true
			}
		} else {
			// Direct IP comparison
			allowedIPAddr := net.ParseIP(allowedIP)
			if allowedIPAddr != nil && allowedIPAddr.Equal(clientIPAddr) {
				return true
			}
		}
	}

	return false
}

// CheckRateLimit checks if the request is within rate limits
func (sm *SecurityManager) CheckRateLimit(clientID string) bool {
	if !sm.config.EnableRateLimit || sm.rateLimiter == nil {
		return true // Rate limiting disabled
	}

	return sm.rateLimiter.Allow(clientID)
}

// EncryptData encrypts data if encryption is enabled
func (sm *SecurityManager) EncryptData(data []byte) ([]byte, error) {
	if !sm.config.EnableEncryption {
		return data, nil // Encryption disabled
	}

	// Simple XOR encryption for demonstration
	// In production, use proper encryption like AES
	key := []byte(sm.config.EncryptionKey)
	if len(key) == 0 {
		return nil, fmt.Errorf("encryption key is empty")
	}

	encrypted := make([]byte, len(data))
	for i, b := range data {
		encrypted[i] = b ^ key[i%len(key)]
	}

	return encrypted, nil
}

// DecryptData decrypts data if encryption is enabled
func (sm *SecurityManager) DecryptData(encryptedData []byte) ([]byte, error) {
	if !sm.config.EnableEncryption {
		return encryptedData, nil // Encryption disabled
	}

	// Simple XOR decryption (same as encryption for XOR)
	return sm.EncryptData(encryptedData)
}

// LogAuditEvent logs an audit event
func (sm *SecurityManager) LogAuditEvent(event *AuditEvent) {
	if !sm.config.EnableAuditLog || sm.auditLogger == nil {
		return
	}

	sm.auditLogger.Log(event)
}

// GenerateAPIKey generates a new API key
func (sm *SecurityManager) GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashAPIKey creates a hash of an API key for secure storage
func (sm *SecurityManager) HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// ValidateSecurityConfig validates the security configuration
func ValidateSecurityConfig(config *SecurityConfig) error {
	if config == nil {
		return fmt.Errorf("security config cannot be nil")
	}

	// Validate authentication settings
	if config.EnableAuthentication {
		if len(config.APIKeys) == 0 {
			return fmt.Errorf("authentication enabled but no API keys provided")
		}

		for i, key := range config.APIKeys {
			if len(key) < 16 {
				return fmt.Errorf("API key %d is too short (minimum 16 characters)", i)
			}
		}

		if config.TokenExpiration <= 0 {
			return fmt.Errorf("token expiration must be positive")
		}
	}

	// Validate IP whitelist settings
	if config.EnableIPWhitelist {
		if len(config.AllowedIPs) == 0 {
			return fmt.Errorf("IP whitelist enabled but no allowed IPs provided")
		}

		for i, ip := range config.AllowedIPs {
			if strings.Contains(ip, "/") {
				// Validate CIDR notation
				_, _, err := net.ParseCIDR(ip)
				if err != nil {
					return fmt.Errorf("invalid CIDR notation in allowed IP %d: %s", i, ip)
				}
			} else {
				// Validate IP address
				if net.ParseIP(ip) == nil {
					return fmt.Errorf("invalid IP address in allowed IP %d: %s", i, ip)
				}
			}
		}
	}

	// Validate rate limit settings
	if config.EnableRateLimit {
		if config.RateLimit <= 0 {
			return fmt.Errorf("rate limit must be positive")
		}
	}

	// Validate encryption settings
	if config.EnableEncryption {
		if len(config.EncryptionKey) == 0 {
			return fmt.Errorf("encryption enabled but no encryption key provided")
		}
		if len(config.EncryptionKey) < 16 {
			return fmt.Errorf("encryption key is too short (minimum 16 characters)")
		}
	}

	// Validate audit log settings
	if config.EnableAuditLog {
		if len(config.AuditLogPath) == 0 {
			return fmt.Errorf("audit log enabled but no log path provided")
		}
	}

	return nil
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	rate    int
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens     int
	capacity   int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int) *RateLimiter {
	return &RateLimiter{
		rate:    rate,
		buckets: make(map[string]*TokenBucket),
	}
}

// Allow checks if a request is allowed for the given client ID
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	bucket, exists := rl.buckets[clientID]
	if !exists {
		bucket = &TokenBucket{
			tokens:     rl.rate,
			capacity:   rl.rate,
			lastRefill: time.Now(),
		}
		rl.buckets[clientID] = bucket
	}
	rl.mu.Unlock()

	return bucket.consume()
}

// consume consumes a token from the bucket
func (tb *TokenBucket) consume() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds())
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// AuditEvent represents an audit log event
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	ClientIP  string    `json:"client_ip"`
	UserAgent string    `json:"user_agent"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	Details   string    `json:"details"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	logPath string
	mu      sync.Mutex
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logPath string) (*AuditLogger, error) {
	return &AuditLogger{
		logPath: logPath,
	}, nil
}

// Log logs an audit event
func (al *AuditLogger) Log(event *AuditEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// For now, just log to the application logger
	// In production, this should write to a dedicated audit log file
	log.Infof("AUDIT: %s - %s from %s - %s on %s - %s (%s)",
		event.Timestamp.Format(time.RFC3339),
		event.Action,
		event.ClientIP,
		event.Result,
		event.Resource,
		event.Details,
		event.UserAgent,
	)
}
