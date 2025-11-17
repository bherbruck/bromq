package storage

import (
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCacheMQTTUser(t *testing.T) {
	cache := NewCacheWithRegistry(prometheus.NewRegistry())
	defer cache.Stop()

	// Test cache miss
	user, found := cache.GetMQTTUser("testuser")
	if found {
		t.Error("Expected cache miss for non-existent user")
	}
	if user != nil {
		t.Error("Expected nil user on cache miss")
	}

	// Test cache set and hit
	testUser := &MQTTUser{
		ID:       1,
		Username: "testuser",
	}
	cache.SetMQTTUser("testuser", testUser)

	user, found = cache.GetMQTTUser("testuser")
	if !found {
		t.Error("Expected cache hit after setting user")
	}
	if user == nil || user.Username != "testuser" {
		t.Errorf("Expected user with username 'testuser', got %v", user)
	}

	// Test cache eviction
	cache.DeleteMQTTUser("testuser")
	user, found = cache.GetMQTTUser("testuser")
	if found {
		t.Error("Expected cache miss after eviction")
	}
}

func TestCacheACLRules(t *testing.T) {
	cache := NewCacheWithRegistry(prometheus.NewRegistry())
	defer cache.Stop()

	// Test cache miss
	rules, found := cache.GetACLRules(1)
	if found {
		t.Error("Expected cache miss for non-existent ACL rules")
	}
	if rules != nil {
		t.Error("Expected nil rules on cache miss")
	}

	// Test cache set and hit
	testRules := []ACLRule{
		{ID: 1, MQTTUserID: 1, Topic: "test/#", Permission: "pubsub"},
		{ID: 2, MQTTUserID: 1, Topic: "data/+", Permission: "pub"},
	}
	cache.SetACLRules(1, testRules)

	rules, found = cache.GetACLRules(1)
	if !found {
		t.Error("Expected cache hit after setting ACL rules")
	}
	if len(rules) != 2 {
		t.Errorf("Expected 2 ACL rules, got %d", len(rules))
	}

	// Test cache eviction
	cache.DeleteACLRules(1)
	rules, found = cache.GetACLRules(1)
	if found {
		t.Error("Expected cache miss after eviction")
	}
}

func TestCacheTTLExpiration(t *testing.T) {
	// Use very short TTL for testing (100ms)
	cache := NewCacheWithTTL(100*time.Millisecond, prometheus.NewRegistry())
	defer cache.Stop()

	// Add user to cache
	testUser := &MQTTUser{
		ID:       1,
		Username: "testuser",
	}
	cache.SetMQTTUser("testuser", testUser)

	// Should be in cache immediately
	user, found := cache.GetMQTTUser("testuser")
	if !found {
		t.Error("Expected cache hit immediately after setting")
	}
	if user == nil {
		t.Error("Expected non-nil user")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should now be expired
	user, found = cache.GetMQTTUser("testuser")
	if found {
		t.Error("Expected cache miss after TTL expiration")
	}
	if user != nil {
		t.Error("Expected nil user after expiration")
	}
}

func TestCacheTTLACLRules(t *testing.T) {
	// Use very short TTL for testing (100ms)
	cache := NewCacheWithTTL(100*time.Millisecond, prometheus.NewRegistry())
	defer cache.Stop()

	// Add ACL rules to cache
	testRules := []ACLRule{
		{ID: 1, MQTTUserID: 1, Topic: "test/#", Permission: "pubsub"},
	}
	cache.SetACLRules(1, testRules)

	// Should be in cache immediately
	rules, found := cache.GetACLRules(1)
	if !found {
		t.Error("Expected cache hit immediately after setting")
	}
	if len(rules) != 1 {
		t.Errorf("Expected 1 ACL rule, got %d", len(rules))
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should now be expired
	rules, found = cache.GetACLRules(1)
	if found {
		t.Error("Expected cache miss after TTL expiration")
	}
	if len(rules) != 0 {
		t.Error("Expected empty rules after expiration")
	}
}

func TestCacheBackgroundCleanup(t *testing.T) {
	// Use very short TTL for testing (100ms)
	cache := NewCacheWithTTL(100*time.Millisecond, prometheus.NewRegistry())
	defer cache.Stop()

	// Add multiple users and ACL rules
	for i := 1; i <= 5; i++ {
		user := &MQTTUser{
			ID:       uint(i),
			Username: "user" + string(rune('0'+i)),
		}
		cache.SetMQTTUser(user.Username, user)

		rules := []ACLRule{
			{ID: uint(i), MQTTUserID: uint(i), Topic: "test/#", Permission: "pubsub"},
		}
		cache.SetACLRules(uint(i), rules)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Manually trigger cleanup (normally runs every 1 minute)
	cache.cleanupExpired()

	// All entries should be cleaned up
	for i := 1; i <= 5; i++ {
		user, found := cache.GetMQTTUser("user" + string(rune('0'+i)))
		if found || user != nil {
			t.Errorf("Expected user %d to be cleaned up", i)
		}

		rules, found := cache.GetACLRules(uint(i))
		if found || len(rules) != 0 {
			t.Errorf("Expected ACL rules for user %d to be cleaned up", i)
		}
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	cache := NewCacheWithRegistry(prometheus.NewRegistry())
	defer cache.Stop()

	// Simulate concurrent reads and writes
	done := make(chan bool, 10)

	// Writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			user := &MQTTUser{
				ID:       uint(id),
				Username: "user" + string(rune('0'+id)),
			}
			cache.SetMQTTUser(user.Username, user)

			rules := []ACLRule{
				{ID: uint(id), MQTTUserID: uint(id), Topic: "test/#", Permission: "pubsub"},
			}
			cache.SetACLRules(uint(id), rules)
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		go func(id int) {
			// Try to read (may or may not exist yet)
			cache.GetMQTTUser("user" + string(rune('0'+id)))
			cache.GetACLRules(uint(id))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without data races, test passed
}

func TestCacheTTLConfigLimits(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{
			name:     "default when not set",
			envValue: "",
			want:     5 * time.Minute,
		},
		{
			name:     "valid value",
			envValue: "10m",
			want:     10 * time.Minute,
		},
		{
			name:     "too low - enforced minimum",
			envValue: "10s",
			want:     1 * time.Minute,
		},
		{
			name:     "too high - enforced maximum",
			envValue: "2h",
			want:     1 * time.Hour,
		},
		{
			name:     "invalid format - use default",
			envValue: "invalid",
			want:     5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CACHE_TTL", tt.envValue)
			} else {
				os.Unsetenv("CACHE_TTL")
			}
			defer os.Unsetenv("CACHE_TTL")

			got := loadCacheTTL()
			if got != tt.want {
				t.Errorf("loadCacheTTL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheMetrics(t *testing.T) {
	cache := NewCacheWithRegistry(prometheus.NewRegistry())
	defer cache.Stop()

	// Initial state - should have misses
	user, found := cache.GetMQTTUser("testuser")
	if found || user != nil {
		t.Error("Expected cache miss")
	}

	// Add user - should have hit on second read
	testUser := &MQTTUser{
		ID:       1,
		Username: "testuser",
	}
	cache.SetMQTTUser("testuser", testUser)

	user, found = cache.GetMQTTUser("testuser")
	if !found || user == nil {
		t.Error("Expected cache hit")
	}

	// Metrics should be updated (hits and misses counters)
	// We can't easily test the exact values without exposing the metrics,
	// but we can verify the cache behaves correctly which exercises the metrics
}
