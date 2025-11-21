package badgerstore

import (
	"os"
	"testing"
)

// OpenInMemory creates a temporary BadgerDB instance for testing
// The database is automatically cleaned up when the test completes
func OpenInMemory(t *testing.T) *BadgerStore {
	t.Helper()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Clean up on test completion
	t.Cleanup(func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	})

	store, err := Open(&Config{Path: tempDir})
	if err != nil {
		t.Fatalf("Failed to open test BadgerDB: %v", err)
	}

	// Clean up BadgerDB on test completion
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close BadgerDB: %v", err)
		}
	})

	return store
}
