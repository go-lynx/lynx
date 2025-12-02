package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
)

// Example integration with Lynx framework

// UserService demonstrates cache usage in a service layer
type UserService struct {
	cache *Cache
	// In real scenario, this would be a database connection
}

// User represents a user entity
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserService creates a new user service with cache
func NewUserService() (*UserService, error) {
	// Create a cache optimized for user data
	cache, err := NewBuilder("user-service").
		WithMaxItems(10000).    // Support 10k users
		WithMaxMemory(1 << 28). // 256MB memory
		WithMetrics(true).      // Enable metrics for monitoring
		WithEvictionCallback(func(item *ristretto.Item) {
			// Log eviction for debugging
			fmt.Printf("User cache evicted: %v\n", item.Key)
		}).
		Build()

	if err != nil {
		return nil, err
	}

	return &UserService{cache: cache}, nil
}

// GetUser retrieves a user with caching
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	cacheKey := fmt.Sprintf("user:%s", userID)

	// Try to get from cache first
	value, err := s.cache.GetOrSetContext(ctx, cacheKey,
		func(ctx context.Context) (interface{}, error) {
			// Simulate database fetch
			return s.fetchUserFromDB(ctx, userID)
		}, 5*time.Minute) // Cache for 5 minutes

	if err != nil {
		return nil, err
	}

	user, ok := value.(*User)
	if !ok {
		return nil, fmt.Errorf("invalid cache data type")
	}

	return user, nil
}

// UpdateUser updates a user and invalidates cache
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	// Update in database (simulated)
	if err := s.updateUserInDB(ctx, user); err != nil {
		return err
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("user:%s", user.ID)
	s.cache.Delete(cacheKey)

	// Optionally, pre-warm the cache with updated data
	s.cache.Set(cacheKey, user, 5*time.Minute)

	return nil
}

// GetUsersByRole demonstrates batch caching
func (s *UserService) GetUsersByRole(ctx context.Context, role string) ([]*User, error) {
	cacheKey := fmt.Sprintf("users:role:%s", role)

	value, err := s.cache.GetOrSetContext(ctx, cacheKey,
		func(ctx context.Context) (interface{}, error) {
			// Simulate fetching multiple users from DB
			return s.fetchUsersByRoleFromDB(ctx, role)
		}, 3*time.Minute) // Shorter TTL for list queries

	if err != nil {
		return nil, err
	}

	users, ok := value.([]*User)
	if !ok {
		return nil, fmt.Errorf("invalid cache data type")
	}

	return users, nil
}

// GetMultipleUsers demonstrates batch get
func (s *UserService) GetMultipleUsers(ctx context.Context, userIDs []string) (map[string]*User, error) {
	// Prepare cache keys
	keys := make([]interface{}, len(userIDs))
	keyMap := make(map[interface{}]string)
	for i, id := range userIDs {
		key := fmt.Sprintf("user:%s", id)
		keys[i] = key
		keyMap[key] = id
	}

	// Batch get from cache
	cached := s.cache.GetMulti(keys)
	result := make(map[string]*User)
	missingIDs := []string{}

	// Process cached results
	for key, userID := range keyMap {
		if value, ok := cached[key]; ok {
			if user, ok := value.(*User); ok {
				result[userID] = user
			}
		} else {
			missingIDs = append(missingIDs, userID)
		}
	}

	// Fetch missing users from DB
	if len(missingIDs) > 0 {
		missing, err := s.fetchMultipleUsersFromDB(ctx, missingIDs)
		if err != nil {
			return nil, err
		}

		// Cache the fetched users
		toCache := make(map[interface{}]interface{})
		for id, user := range missing {
			key := fmt.Sprintf("user:%s", id)
			toCache[key] = user
			result[id] = user
		}

		if len(toCache) > 0 {
			s.cache.SetMulti(toCache, 5*time.Minute)
		}
	}

	return result, nil
}

// InvalidateUserCache clears all user-related caches
func (s *UserService) InvalidateUserCache() {
	s.cache.Clear()
}

// GetCacheStats returns cache statistics
func (s *UserService) GetCacheStats() string {
	if metrics := s.cache.Metrics(); metrics != nil {
		stats := map[string]interface{}{
			"hits":         metrics.Hits(),
			"misses":       metrics.Misses(),
			"keys_added":   metrics.KeysAdded(),
			"keys_evicted": metrics.KeysEvicted(),
			"cost_added":   metrics.CostAdded(),
			"cost_evicted": metrics.CostEvicted(),
		}

		if metrics.Hits() > 0 || metrics.Misses() > 0 {
			hitRatio := float64(metrics.Hits()) / float64(metrics.Hits()+metrics.Misses()) * 100
			stats["hit_ratio"] = fmt.Sprintf("%.2f%%", hitRatio)
		}

		data, _ := json.MarshalIndent(stats, "", "  ")
		return string(data)
	}
	return "{}"
}

// Close closes the cache
func (s *UserService) Close() {
	s.cache.Close()
}

// Simulated database operations
func (s *UserService) fetchUserFromDB(ctx context.Context, userID string) (*User, error) {
	// Simulate database fetch
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond): // Simulate DB latency
		return &User{
			ID:        userID,
			Name:      fmt.Sprintf("User %s", userID),
			Email:     fmt.Sprintf("user%s@example.com", userID),
			Role:      "user",
			CreatedAt: time.Now(),
		}, nil
	}
}

func (s *UserService) updateUserInDB(ctx context.Context, user *User) error {
	// reference parameter to avoid unused warning in this simulated example
	_ = user
	// Simulate database update
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func (s *UserService) fetchUsersByRoleFromDB(ctx context.Context, role string) ([]*User, error) {
	// Simulate fetching multiple users
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(20 * time.Millisecond):
		users := []*User{
			{ID: "1", Name: "Alice", Email: "alice@example.com", Role: role, CreatedAt: time.Now()},
			{ID: "2", Name: "Bob", Email: "bob@example.com", Role: role, CreatedAt: time.Now()},
			{ID: "3", Name: "Charlie", Email: "charlie@example.com", Role: role, CreatedAt: time.Now()},
		}
		return users, nil
	}
}

func (s *UserService) fetchMultipleUsersFromDB(ctx context.Context, userIDs []string) (map[string]*User, error) {
	// Simulate batch fetch
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(15 * time.Millisecond):
		result := make(map[string]*User)
		for _, id := range userIDs {
			result[id] = &User{
				ID:        id,
				Name:      fmt.Sprintf("User %s", id),
				Email:     fmt.Sprintf("user%s@example.com", id),
				Role:      "user",
				CreatedAt: time.Now(),
			}
		}
		return result, nil
	}
}

// APICache demonstrates caching for API responses
type APICache struct {
	cache *Cache
}

// NewAPICache creates a new API cache
func NewAPICache() (*APICache, error) {
	cache, err := APICacheBuilder("api-responses").BuildAndRegister()
	if err != nil {
		return nil, err
	}
	return &APICache{cache: cache}, nil
}

// CacheResponse caches an API response
func (a *APICache) CacheResponse(endpoint string, response interface{}, ttl time.Duration) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return a.cache.Set(endpoint, data, ttl)
}

// GetCachedResponse retrieves a cached API response
func (a *APICache) GetCachedResponse(endpoint string, result interface{}) error {
	data, err := a.cache.Get(endpoint)
	if err != nil {
		return err
	}

	bytes, ok := data.([]byte)
	if !ok {
		return fmt.Errorf("invalid cache data type")
	}

	return json.Unmarshal(bytes, result)
}

// SessionCache demonstrates session management with cache
type SessionCache struct {
	cache *Cache
}

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// NewSessionCache creates a new session cache
func NewSessionCache(sessionTTL time.Duration) (*SessionCache, error) {
	cache, err := SessionCacheBuilder("sessions", sessionTTL).BuildAndRegister()
	if err != nil {
		return nil, err
	}
	return &SessionCache{cache: cache}, nil
}

// CreateSession creates a new session
func (s *SessionCache) CreateSession(userID string, ttl time.Duration) (*Session, error) {
	session := &Session{
		ID:        fmt.Sprintf("sess_%d", time.Now().UnixNano()),
		UserID:    userID,
		Data:      make(map[string]interface{}),
		ExpiresAt: time.Now().Add(ttl),
	}

	err := s.cache.Set(session.ID, session, ttl)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession retrieves a session
func (s *SessionCache) GetSession(sessionID string) (*Session, error) {
	value, err := s.cache.Get(sessionID)
	if err != nil {
		return nil, err
	}

	session, ok := value.(*Session)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		s.cache.Delete(sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// UpdateSession updates session data
func (s *SessionCache) UpdateSession(session *Session) error {
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session expired")
	}
	return s.cache.Set(session.ID, session, ttl)
}

// DeleteSession removes a session
func (s *SessionCache) DeleteSession(sessionID string) {
	s.cache.Delete(sessionID)
}
