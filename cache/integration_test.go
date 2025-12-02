package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUserService_Integration(t *testing.T) {
	service, err := NewUserService()
	if err != nil {
		t.Fatalf("Failed to create user service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Test single user fetch with caching
	user1, err := service.GetUser(ctx, "123")
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}
	if user1.ID != "123" {
		t.Errorf("Expected user ID 123, got %s", user1.ID)
	}

	// Second fetch should hit cache
	user2, err := service.GetUser(ctx, "123")
	if err != nil {
		t.Errorf("Failed to get cached user: %v", err)
	}
	if user1.Name != user2.Name {
		t.Error("Cached user data mismatch")
	}

	// Test update with cache invalidation
	user1.Name = "Updated Name"
	err = service.UpdateUser(ctx, user1)
	if err != nil {
		t.Errorf("Failed to update user: %v", err)
	}

	// Fetch after update should get new data
	user3, err := service.GetUser(ctx, "123")
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}
	if user3.Name != "Updated Name" {
		t.Errorf("Expected updated name, got %s", user3.Name)
	}

	// Test batch operations
	userIDs := []string{"1", "2", "3", "4", "5"}
	users, err := service.GetMultipleUsers(ctx, userIDs)
	if err != nil {
		t.Errorf("Failed to get multiple users: %v", err)
	}
	if len(users) != len(userIDs) {
		t.Errorf("Expected %d users, got %d", len(userIDs), len(users))
	}

	// Test role-based query
	roleUsers, err := service.GetUsersByRole(ctx, "admin")
	if err != nil {
		t.Errorf("Failed to get users by role: %v", err)
	}
	if len(roleUsers) == 0 {
		t.Error("Expected users with admin role")
	}

	// Check cache stats
	stats := service.GetCacheStats()
	if stats == "{}" {
		t.Log("Cache metrics not available")
	} else {
		t.Logf("Cache stats: %s", stats)
	}
}

func TestAPICache_Integration(t *testing.T) {
	apiCache, err := NewAPICache()
	if err != nil {
		t.Fatalf("Failed to create API cache: %v", err)
	}
	defer apiCache.cache.Close()

	// Test caching API response
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"items": []string{"item1", "item2", "item3"},
			"total": 3,
		},
	}

	err = apiCache.CacheResponse("/api/items", response, 1*time.Minute)
	if err != nil {
		t.Errorf("Failed to cache response: %v", err)
	}

	// Retrieve cached response
	var cached map[string]interface{}
	err = apiCache.GetCachedResponse("/api/items", &cached)
	if err != nil {
		t.Errorf("Failed to get cached response: %v", err)
	}

	if cached["status"] != "success" {
		t.Error("Cached response mismatch")
	}

	// Test cache miss
	var notCached map[string]interface{}
	err = apiCache.GetCachedResponse("/api/not-cached", &notCached)
	if err != ErrCacheMiss {
		t.Error("Expected cache miss error")
	}
}

func TestSessionCache_Integration(t *testing.T) {
	sessionCache, err := NewSessionCache(30 * time.Minute)
	if err != nil {
		t.Fatalf("Failed to create session cache: %v", err)
	}
	defer sessionCache.cache.Close()

	// Create session
	session, err := sessionCache.CreateSession("user123", 1*time.Hour)
	if err != nil {
		t.Errorf("Failed to create session: %v", err)
	}

	// Add data to session
	session.Data["username"] = "testuser"
	session.Data["role"] = "admin"

	// Update session
	err = sessionCache.UpdateSession(session)
	if err != nil {
		t.Errorf("Failed to update session: %v", err)
	}

	// Retrieve session
	retrieved, err := sessionCache.GetSession(session.ID)
	if err != nil {
		t.Errorf("Failed to get session: %v", err)
	}

	if retrieved.UserID != "user123" {
		t.Errorf("Session user ID mismatch")
	}

	if retrieved.Data["username"] != "testuser" {
		t.Error("Session data mismatch")
	}

	// Delete session
	sessionCache.DeleteSession(session.ID)

	// Try to get deleted session
	_, err = sessionCache.GetSession(session.ID)
	if err != ErrCacheMiss {
		t.Error("Expected cache miss for deleted session")
	}
}

func TestConcurrentIntegration(t *testing.T) {
	service, err := NewUserService()
	if err != nil {
		t.Fatalf("Failed to create user service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			userID := fmt.Sprintf("user%d", id%5)
			_, err := service.GetUser(ctx, userID)
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
		}(i)
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			user := &User{
				ID:    fmt.Sprintf("user%d", id),
				Name:  fmt.Sprintf("User %d", id),
				Email: fmt.Sprintf("user%d@example.com", id),
			}
			err := service.UpdateUser(ctx, user)
			if err != nil {
				t.Errorf("Concurrent write failed: %v", err)
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}

func BenchmarkUserService_GetUser(b *testing.B) {
	service, err := NewUserService()
	if err != nil {
		b.Fatalf("Failed to create user service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Pre-warm cache
	service.GetUser(ctx, "bench-user")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.GetUser(ctx, "bench-user")
			if err != nil {
				b.Errorf("Failed to get user: %v", err)
			}
		}
	})
}

func BenchmarkAPICache_CacheResponse(b *testing.B) {
	apiCache, err := NewAPICache()
	if err != nil {
		b.Fatalf("Failed to create API cache: %v", err)
	}
	defer apiCache.cache.Close()

	response := map[string]interface{}{
		"status": "success",
		"data":   "test data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		endpoint := fmt.Sprintf("/api/endpoint%d", i%100)
		err := apiCache.CacheResponse(endpoint, response, 1*time.Minute)
		if err != nil {
			b.Errorf("Failed to cache response: %v", err)
		}
	}
}
