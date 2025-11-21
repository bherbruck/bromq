package storage

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	config := DefaultSQLiteConfig(":memory:")
	// Use isolated Prometheus registry to prevent duplicate registration in tests
	cache := NewCacheWithRegistry(prometheus.NewRegistry())
	db, err := OpenWithCache(config, cache)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create default admin user for tests
	if err := db.CreateDefaultAdmin("admin", "admin"); err != nil {
		t.Fatalf("failed to create default admin: %v", err)
	}

	return db
}

// createTestDashboardUser is a helper to create a test admin user
func createTestDashboardUser(t *testing.T, db *DB, username, password, role string) *DashboardUser {
	t.Helper()

	user, err := db.CreateDashboardUser(username, password, role)
	if err != nil {
		t.Fatalf("failed to create test admin user: %v", err)
	}

	return user
}

// createTestMQTTUser is a helper to create a test MQTT user
func createTestMQTTUser(t *testing.T, db *DB, username, password, description string) *MQTTUser {
	t.Helper()

	user, err := db.CreateMQTTUser(username, password, description, nil)
	if err != nil {
		t.Fatalf("failed to create test MQTT user: %v", err)
	}

	return user
}

// createTestACLRule is a helper to create a test ACL rule
func createTestACLRule(t *testing.T, db *DB, mqttUserID uint, topicPattern, permission string) *ACLRule {
	t.Helper()

	rule, err := db.CreateACLRule(mqttUserID, topicPattern, permission)
	if err != nil {
		t.Fatalf("failed to create test ACL rule: %v", err)
	}

	return rule
}
