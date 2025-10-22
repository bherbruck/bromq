package storage

import (
	"testing"
)

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	return db
}

// createTestUser is a helper to create a test user
func createTestUser(t *testing.T, db *DB, username, password, role string) *User {
	t.Helper()

	user, err := db.CreateUser(username, password, role)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

// createTestACLRule is a helper to create a test ACL rule
func createTestACLRule(t *testing.T, db *DB, userID int, topicPattern, permission string) *ACLRule {
	t.Helper()

	rule, err := db.CreateACLRule(userID, topicPattern, permission)
	if err != nil {
		t.Fatalf("failed to create test ACL rule: %v", err)
	}

	return rule
}
